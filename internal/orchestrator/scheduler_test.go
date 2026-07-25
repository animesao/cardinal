package orchestrator

import (
	"context"
	"testing"
	"time"
)

func TestScheduleReplica_NoNodes(t *testing.T) {
	clusterLock.Lock()
	clusterConf = &ClusterConfig{
		Nodes:    make(map[string]*Node),
		Services: make(map[string]*Service),
	}
	clusterLock.Unlock()
	t.Cleanup(func() {
		clusterLock.Lock()
		clusterConf = &ClusterConfig{Nodes: make(map[string]*Node), Services: make(map[string]*Service)}
		clusterLock.Unlock()
	})

	ctx := context.Background()
	svc := &Service{Name: "test", Image: "alpine:latest", Replicas: 1}
	err := ScheduleReplica(ctx, "test", svc)
	if err == nil {
		t.Fatal("expected error when no nodes available")
	}
}

func TestScheduleReplica_ContextCancelled(t *testing.T) {
	clusterLock.Lock()
	clusterConf = &ClusterConfig{
		ClusterID:   "test-cluster",
		ClusterName: "test",
		NodeID:      "local-node",
		Nodes: map[string]*Node{
			"local-node": {
				ID:       "local-node",
				Name:     "local",
				Address:  "127.0.0.1",
				APIPort:  2375,
				State:    NodeStateActive,
				MemAvail: 1024 * 1024 * 1024,
			},
		},
		Services: make(map[string]*Service),
	}
	clusterLock.Unlock()
	t.Cleanup(func() {
		clusterLock.Lock()
		clusterConf = &ClusterConfig{Nodes: make(map[string]*Node), Services: make(map[string]*Service)}
		clusterLock.Unlock()
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	svc := &Service{Name: "test", Image: "alpine:latest", Replicas: 1}
	err := ScheduleReplica(ctx, "test", svc)
	if err == nil {
		t.Fatal("expected error when context is cancelled")
	}
}

func TestAutoHealServices_ContextCancelled(t *testing.T) {
	clusterLock.Lock()
	clusterConf = &ClusterConfig{
		Nodes:    make(map[string]*Node),
		Services: make(map[string]*Service),
	}
	clusterLock.Unlock()
	t.Cleanup(func() {
		clusterLock.Lock()
		clusterConf = &ClusterConfig{Nodes: make(map[string]*Node), Services: make(map[string]*Service)}
		clusterLock.Unlock()
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	AutoHealServices(ctx)
}

func TestRollingUpdateService_ContextCancelled(t *testing.T) {
	clusterLock.Lock()
	clusterConf = &ClusterConfig{
		Nodes:    make(map[string]*Node),
		Services: make(map[string]*Service),
	}
	clusterLock.Unlock()
	t.Cleanup(func() {
		clusterLock.Lock()
		clusterConf = &ClusterConfig{Nodes: make(map[string]*Node), Services: make(map[string]*Service)}
		clusterLock.Unlock()
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := RollingUpdateService(ctx, "nonexistent", "alpine:latest", ServiceOpts{})
	if err == nil {
		t.Fatal("expected error for non-existent service")
	}
}

func TestStartAutoHealer_ContextCancellation(t *testing.T) {
	clusterLock.Lock()
	clusterConf = &ClusterConfig{
		Nodes:    make(map[string]*Node),
		Services: make(map[string]*Service),
	}
	clusterLock.Unlock()
	t.Cleanup(func() {
		clusterLock.Lock()
		clusterConf = &ClusterConfig{Nodes: make(map[string]*Node), Services: make(map[string]*Service)}
		clusterLock.Unlock()
	})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		StartAutoHealer(ctx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("StartAutoHealer did not exit after context cancellation")
	}
}

func TestRemoveRemoteReplica_ContextCancelled(t *testing.T) {
	clusterLock.Lock()
	clusterConf = &ClusterConfig{
		Nodes:    make(map[string]*Node),
		Services: make(map[string]*Service),
	}
	clusterLock.Unlock()
	t.Cleanup(func() {
		clusterLock.Lock()
		clusterConf = &ClusterConfig{Nodes: make(map[string]*Node), Services: make(map[string]*Service)}
		clusterLock.Unlock()
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := RemoveRemoteReplica(ctx, "nonexistent", "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent node")
	}
}


