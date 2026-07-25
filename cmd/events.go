//go:build linux

package cmd

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dck/internal/container"
)

func Events(args []string) {
	fs := flag.NewFlagSet("events", flag.ExitOnError)
	sinceStr := fs.String("since", "", "Show events created since timestamp")
	fs.Parse(args)

	var since time.Time
	if *sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, *sinceStr); err == nil {
			since = t
		} else if ts, err := time.Parse("2006-01-02 15:04:05", *sinceStr); err == nil {
			since = ts
		}
	}

	fmt.Fprintf(os.Stderr, "Listening for events... (since %s)\n", since.Format(time.RFC3339))
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	ch := container.SubscribeEvents(100)
	defer container.UnsubscribeEvents(ch)

	enc := json.NewEncoder(os.Stdout)

	for {
		select {
		case evt := <-ch:
			if !since.IsZero() && evt.Time.Before(since) {
				continue
			}
			enc.Encode(evt)
		case <-sig:
			fmt.Fprintf(os.Stderr, "\n")
			return
		}
	}
}
