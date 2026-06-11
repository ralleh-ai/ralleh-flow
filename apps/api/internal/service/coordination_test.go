package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestInMemoryCoordinatorRepoLockExclusive(t *testing.T) {
	coord := NewInMemoryCoordinator()
	ctx := context.Background()

	lease, err := coord.AcquireRepoLock(ctx, "/tmp/repo", "worker-a", time.Minute)
	if err != nil {
		t.Fatalf("acquire first repo lock: %v", err)
	}
	defer lease.Release(ctx)

	_, err = coord.AcquireRepoLock(ctx, "/tmp/repo", "worker-b", time.Minute)
	if !errors.Is(err, ErrRepositoryLocked) {
		t.Fatalf("expected ErrRepositoryLocked, got %v", err)
	}
}

func TestInMemoryCoordinatorRunLeaseExclusive(t *testing.T) {
	coord := NewInMemoryCoordinator()
	ctx := context.Background()

	lease, err := coord.AcquireRunLease(ctx, "run_123", "worker-a", time.Minute)
	if err != nil {
		t.Fatalf("acquire first run lease: %v", err)
	}
	defer lease.Release(ctx)

	_, err = coord.AcquireRunLease(ctx, "run_123", "worker-b", time.Minute)
	if !errors.Is(err, ErrRunLeaseHeld) {
		t.Fatalf("expected ErrRunLeaseHeld, got %v", err)
	}
}

func TestInMemoryCoordinatorLeaseReleaseAllowsReacquire(t *testing.T) {
	coord := NewInMemoryCoordinator()
	ctx := context.Background()

	lease, err := coord.AcquireRepoLock(ctx, "/tmp/repo", "worker-a", time.Minute)
	if err != nil {
		t.Fatalf("acquire repo lock: %v", err)
	}
	if err := lease.Release(ctx); err != nil {
		t.Fatalf("release repo lock: %v", err)
	}

	_, err = coord.AcquireRepoLock(ctx, "/tmp/repo", "worker-b", time.Minute)
	if err != nil {
		t.Fatalf("reacquire repo lock: %v", err)
	}
}
