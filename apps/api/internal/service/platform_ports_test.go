package service

import (
	"context"
	"testing"
)

func TestNoopTaskEventPublisher(t *testing.T) {
	publisher := NewNoopTaskEventPublisher()
	if publisher.Mode() != "noop" {
		t.Fatalf("expected noop mode, got %q", publisher.Mode())
	}
	if err := publisher.PublishTaskEvent(context.Background(), TaskLifecycleEvent{RunID: "run_123", Type: "run.created"}); err != nil {
		t.Fatalf("publish noop task event: %v", err)
	}
}

func TestNoopContextProviderReturnsEmptyBundle(t *testing.T) {
	provider := NewNoopContextProvider()
	bundle, err := provider.RetrieveContext(context.Background(), "  run context query  ", RunRecord{}, StepRecord{}, 256)
	if err != nil {
		t.Fatalf("retrieve noop context: %v", err)
	}
	if bundle.Query != "run context query" {
		t.Fatalf("expected trimmed query, got %q", bundle.Query)
	}
	if len(bundle.Entries) != 0 {
		t.Fatalf("expected empty entries, got %#v", bundle.Entries)
	}
	if bundle.Source != "none" {
		t.Fatalf("expected source=none, got %q", bundle.Source)
	}
}
