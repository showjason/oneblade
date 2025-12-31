package opensearch

import (
	"context"
	"testing"
)

func TestService_Name(t *testing.T) {
	opts := &Options{
		Addresses: []string{"http://localhost:9200"},
		Index:     "test-index",
	}
	svc, err := NewService(opts)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	if svc.Name() != "opensearch" {
		t.Errorf("expected opensearch, got %s", svc.Name())
	}
}

func TestService_AsTool(t *testing.T) {
	opts := &Options{
		Addresses: []string{"http://localhost:9200"},
		Index:     "test-index",
	}
	svc, err := NewService(opts)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	_, err = svc.AsTool()
	if err != nil {
		t.Fatalf("AsTool failed: %v", err)
	}
	// if tool.Manifest().Name != "opensearch_service" {
	// 	t.Errorf("expected tool name opensearch_service, got %s", tool.Manifest().Name)
	// }
}

func TestService_Handle_InvalidOp(t *testing.T) {
	opts := &Options{
		Addresses: []string{"http://localhost:9200"},
		Index:     "test-index",
	}
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
