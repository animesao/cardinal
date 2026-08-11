//go:build linux

package container

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"dck/internal/state"
)

// Event types
const (
	EventCreate  = "create"
	EventStart   = "start"
	EventStop    = "stop"
	EventDestroy = "destroy"
	EventRestart = "restart"
	EventKill    = "kill"
	EventDie     = "die"
	EventHealth  = "health_status"
)

// Event represents a container event
type Event struct {
	Type      string    `json:"type"`
	ActorID   string    `json:"actor_id"`
	ActorName string    `json:"actor_name"`
	ImageName string    `json:"image_name"`
	ImageTag  string    `json:"image_tag"`
	Status    string    `json:"status"`
	Time      time.Time `json:"time"`
}

var (
	eventListeners []chan Event
	eventMu        sync.RWMutex
	eventHistory   []Event
	eventHistMu    sync.RWMutex
	eventFile      *os.File
	eventFileMu    sync.Mutex
)

// EmitEvent sends an event to all listeners and persists it
func EmitEvent(etype string, c *Container) {
	evt := Event{
		Type:      etype,
		ActorID:   c.ID,
		ActorName: c.Name,
		ImageName: c.ImageName,
		ImageTag:  c.ImageTag,
		Status:    string(c.Status),
		Time:      time.Now(),
	}

	eventHistMu.Lock()
	eventHistory = append(eventHistory, evt)
	if len(eventHistory) > 1000 {
		eventHistory = eventHistory[len(eventHistory)-1000:]
	}
	eventHistMu.Unlock()

	persistEvent(evt)

	eventMu.RLock()
	for _, ch := range eventListeners {
		select {
		case ch <- evt:
		default:
			// drop if listener is slow
		}
	}
	eventMu.RUnlock()
}

// SubscribeEvents returns a channel that receives events
func SubscribeEvents(buffer int) chan Event {
	ch := make(chan Event, buffer)
	eventMu.Lock()
	eventListeners = append(eventListeners, ch)
	eventMu.Unlock()
	return ch
}

// UnsubscribeEvents removes a listener
func UnsubscribeEvents(ch chan Event) {
	eventMu.Lock()
	for i, listener := range eventListeners {
		if listener == ch {
			eventListeners = append(eventListeners[:i], eventListeners[i+1:]...)
			break
		}
	}
	eventMu.Unlock()
	close(ch)
}

// StreamEvents writes events to stdout as JSON until stop is closed
func StreamEvents(since time.Time, stop chan struct{}) {
	ch := SubscribeEvents(100)
	defer UnsubscribeEvents(ch)

	enc := json.NewEncoder(os.Stdout)

	for {
		select {
		case evt := <-ch:
			if evt.Time.Before(since) {
				continue
			}
			_ = enc.Encode(evt)
		case <-stop:
			return
		}
	}
}

func persistEvent(evt Event) {
	eventFileMu.Lock()
	defer eventFileMu.Unlock()

	if eventFile == nil {
		p := state.DataDir() + "/events.jsonl"
		f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		eventFile = f
	}

	data, _ := json.Marshal(evt)
	_, _ = eventFile.Write(append(data, '\n'))
}

// InitEvents opens event log
func InitEvents() {
	eventFileMu.Lock()
	defer eventFileMu.Unlock()

	p := state.DataDir() + "/events.jsonl"
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		eventFile = f
	}
}

// CloseEvents flushes and closes event log
func CloseEvents() {
	eventFileMu.Lock()
	defer eventFileMu.Unlock()
	if eventFile != nil {
		_ = eventFile.Close()
		eventFile = nil
	}
}

// getEventsSince returns events after a given time
func getEventsSince(since time.Time) []Event {
	eventHistMu.RLock()
	defer eventHistMu.RUnlock()

	var result []Event
	for _, e := range eventHistory {
		if e.Time.After(since) {
			result = append(result, e)
		}
	}
	return result
}
