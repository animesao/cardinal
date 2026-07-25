package orchestrator

import (
	"context"
	"testing"
)

func TestCreateService(t *testing.T) {
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

	svc, err := CreateService("test-svc", "alpine:latest", 3, ServiceOpts{
		Ports: []ServicePort{{Port: 80, TargetPort: 80, Protocol: "tcp"}},
		Env:   map[string]string{"KEY": "value"},
	})
	if err != nil {
		t.Fatalf("CreateService failed: %v", err)
	}
	if svc.Name != "test-svc" {
		t.Fatalf("expected name 'test-svc', got %q", svc.Name)
	}
	if svc.Replicas != 3 {
		t.Fatalf("expected 3 replicas, got %d", svc.Replicas)
	}
	if svc.Image != "alpine:latest" {
		t.Fatalf("expected image 'alpine:latest', got %q", svc.Image)
	}
}

func TestCreateService_Duplicate(t *testing.T) {
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

	_, err := CreateService("dup-svc", "alpine:latest", 1, ServiceOpts{})
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	_, err = CreateService("dup-svc", "alpine:latest", 1, ServiceOpts{})
	if err == nil {
		t.Fatal("expected error for duplicate service name")
	}
}

func TestGetService_NotFound(t *testing.T) {
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

	_, err := GetService("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent service")
	}
}

func TestRemoveService_NotFound(t *testing.T) {
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

	err := RemoveService("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent service")
	}
}

func TestScaleService(t *testing.T) {
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

	_, err := CreateService("scale-svc", "alpine:latest", 1, ServiceOpts{})
	if err != nil {
		t.Fatalf("CreateService failed: %v", err)
	}

	svc, err := ScaleService("scale-svc", 5)
	if err != nil {
		t.Fatalf("ScaleService failed: %v", err)
	}
	if svc.Replicas != 5 {
		t.Fatalf("expected 5 replicas, got %d", svc.Replicas)
	}
}

func TestScaleService_NegativeReplicas(t *testing.T) {
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

	_, err := CreateService("neg-svc", "alpine:latest", 1, ServiceOpts{})
	if err != nil {
		t.Fatalf("CreateService failed: %v", err)
	}

	_, err = ScaleService("neg-svc", -1)
	if err == nil {
		t.Fatal("expected error for negative replicas")
	}
}

func TestUpdateService_ContextCancelled(t *testing.T) {
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

	_, err := UpdateService(ctx, "nonexistent", "alpine:latest", ServiceOpts{})
	if err == nil {
		t.Fatal("expected error for non-existent service")
	}
}

func TestListServices_Empty(t *testing.T) {
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

	svcs, err := ListServices()
	if err != nil {
		t.Fatalf("ListServices failed: %v", err)
	}
	if len(svcs) != 0 {
		t.Fatalf("expected 0 services, got %d", len(svcs))
	}
}

func TestReconcileServices_ContextCancellation(t *testing.T) {
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

	ReconcileServices(ctx)
}
