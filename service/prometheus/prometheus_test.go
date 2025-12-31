package prometheus

import (
	"context"
	"testing"
	"time"
)

func TestService_Name(t *testing.T) {
	opts := &Options{
		Address: "http://localhost:9090",
		Timeout: 10 * time.Second,
	}
	svc, err := NewService(opts)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	if svc.Name() != "prometheus" {
		t.Errorf("expected prometheus, got %s", svc.Name())
	}
}

func TestService_AsTool(t *testing.T) {
	opts := &Options{
		Address: "http://localhost:9090",
	}
	svc, err := NewService(opts)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	_, err = svc.AsTool()
	if err != nil {
		t.Fatalf("AsTool failed: %v", err)
	}
	// if tool.Manifest().Name != "prometheus_service" {
	// 	t.Errorf("expected tool name prometheus_service, got %s", tool.Manifest().Name)
	// }
}

func TestService_Handle_InvalidOp(t *testing.T) {
	opts := &Options{Address: "http://localhost:9090"}
	svc, err := NewService(opts)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := svc.Handle(context.Background(), Request{
		Operation: "invalid",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Success {
		t.Error("expected failure for invalid operation")
	}
}
