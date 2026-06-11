package service

import (
	"context"
	"strings"
	"testing"
)

func TestMemoryEventBusReady(t *testing.T) {
	bus := NewInMemoryEventBus()
	if err := bus.Ready(context.Background()); err != nil {
		t.Fatalf("expected memory event bus to be ready, got %v", err)
	}
	if bus.Mode() != "memory" {
		t.Fatalf("expected memory mode, got %q", bus.Mode())
	}
}

func TestRedisEventBusReadyFailsWhenUnavailable(t *testing.T) {
	bus := NewRedisEventBus("127.0.0.1:1", "ralleh-flow:runs", "ralleh-flow-orchestrators")
	err := bus.Ready(context.Background())
	if err == nil {
		t.Fatal("expected redis event bus readiness error")
	}
	if bus.Mode() != "redis-streams" {
		t.Fatalf("expected redis-streams mode, got %q", bus.Mode())
	}
}

func TestDefaultString(t *testing.T) {
	if got := defaultString("", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}
	if got := defaultString("value", "fallback"); got != "value" {
		t.Fatalf("expected value, got %q", got)
	}
}

func TestRepoLockKeyStable(t *testing.T) {
	first := repoLockKey("/tmp/repo")
	second := repoLockKey("/tmp/repo")
	if first != second {
		t.Fatalf("expected stable repo lock key, got %q and %q", first, second)
	}
	if !strings.HasPrefix(first, "repo:") {
		t.Fatalf("expected repo prefix, got %q", first)
	}
}
