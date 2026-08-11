//go:build linux

package orchestrator

import (
	"context"
	"testing"
	"time"
)

func TestDeployFunction_ContextCancelled(t *testing.T) {
	allFunctions = make(map[string]*Function)
	functionContainers = make(map[string][]string)
	t.Cleanup(func() {
		allFunctions = make(map[string]*Function)
		functionContainers = make(map[string][]string)
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	opts := FnOpts{Handler: "/handler", Timeout: 30, IdleTimeout: 300, Replicas: 1}
	_, err := DeployFunction(ctx, "test-fn", "alpine:latest", 8080, opts)
	if err != nil {
		t.Logf("DeployFunction returned: %v", err)
	}
}

func TestDeployFunction_Duplicate(t *testing.T) {
	allFunctions = make(map[string]*Function)
	functionContainers = make(map[string][]string)
	t.Cleanup(func() {
		allFunctions = make(map[string]*Function)
		functionContainers = make(map[string][]string)
	})

	ctx := context.Background()
	opts := FnOpts{Handler: "/handler", Timeout: 30, IdleTimeout: 300, Replicas: 1}

	_, err := DeployFunction(ctx, "test-fn", "alpine:latest", 8080, opts)
	if err != nil {
		t.Fatalf("first deploy failed: %v", err)
	}

	_, err = DeployFunction(ctx, "test-fn", "alpine:latest", 8080, opts)
	if err == nil {
		t.Fatal("expected error for duplicate function name")
	}
}

func TestGetFunction_NotFound(t *testing.T) {
	allFunctions = make(map[string]*Function)
	functionContainers = make(map[string][]string)
	t.Cleanup(func() {
		allFunctions = make(map[string]*Function)
		functionContainers = make(map[string][]string)
	})

	_, err := GetFunction("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent function")
	}
}

func TestRemoveFunction_NotFound(t *testing.T) {
	allFunctions = make(map[string]*Function)
	functionContainers = make(map[string][]string)
	t.Cleanup(func() {
		allFunctions = make(map[string]*Function)
		functionContainers = make(map[string][]string)
	})

	err := RemoveFunction(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent function")
	}
}

func TestInvokeFunction_ContextCancelled(t *testing.T) {
	allFunctions = make(map[string]*Function)
	functionContainers = make(map[string][]string)
	t.Cleanup(func() {
		allFunctions = make(map[string]*Function)
		functionContainers = make(map[string][]string)
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := InvokeFunction(ctx, "nonexistent", []byte("{}"))
	if err == nil {
		t.Fatal("expected error for non-existent function")
	}
}

func TestStartFunctionGC_ContextCancellation(t *testing.T) {
	clusterLock.Lock()
	clusterConf = &ClusterConfig{
		Nodes:    make(map[string]*Node),
		Services: make(map[string]*Service),
	}
	clusterLock.Unlock()
	allFunctions = make(map[string]*Function)
	functionContainers = make(map[string][]string)
	t.Cleanup(func() {
		clusterLock.Lock()
		clusterConf = &ClusterConfig{Nodes: make(map[string]*Node), Services: make(map[string]*Service)}
		clusterLock.Unlock()
		allFunctions = make(map[string]*Function)
		functionContainers = make(map[string][]string)
	})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		StartFunctionGC(ctx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("StartFunctionGC did not exit after context cancellation")
	}
}

func TestCleanIdleFunctions_ContextCancellation(t *testing.T) {
	allFunctions = make(map[string]*Function)
	functionContainers = make(map[string][]string)
	t.Cleanup(func() {
		allFunctions = make(map[string]*Function)
		functionContainers = make(map[string][]string)
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	CleanIdleFunctions(ctx)
}

func TestListFunctions_Empty(t *testing.T) {
	allFunctions = make(map[string]*Function)
	functionContainers = make(map[string][]string)
	t.Cleanup(func() {
		allFunctions = make(map[string]*Function)
		functionContainers = make(map[string][]string)
	})

	fns, err := ListFunctions()
	if err != nil {
		t.Fatalf("ListFunctions failed: %v", err)
	}
	if len(fns) != 0 {
		t.Fatalf("expected 0 functions, got %d", len(fns))
	}
}
