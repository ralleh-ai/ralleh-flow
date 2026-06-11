package service

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrRepositoryLocked = errors.New("execution repository is busy")
	ErrRunLeaseHeld     = errors.New("run lease is already held")
)

type Lease interface {
	Release(ctx context.Context) error
	Key() string
	Owner() string
}

type Coordinator interface {
	AcquireRepoLock(ctx context.Context, repoRoot, owner string, ttl time.Duration) (Lease, error)
	AcquireRunLease(ctx context.Context, runID, owner string, ttl time.Duration) (Lease, error)
	Ready(ctx context.Context) error
	Mode() string
}

type memoryCoordinator struct {
	mu    sync.Mutex
	locks map[string]memoryLockState
	now   func() time.Time
}

type memoryLockState struct {
	owner     string
	expiresAt time.Time
}

type memoryLease struct {
	coordinator *memoryCoordinator
	key         string
	owner       string
}

func NewInMemoryCoordinator() Coordinator {
	return &memoryCoordinator{
		locks: map[string]memoryLockState{},
		now:   time.Now,
	}
}

func (c *memoryCoordinator) AcquireRepoLock(_ context.Context, repoRoot, owner string, ttl time.Duration) (Lease, error) {
	return c.acquire(repoLockKey(repoRoot), owner, ttl, ErrRepositoryLocked)
}

func (c *memoryCoordinator) AcquireRunLease(_ context.Context, runID, owner string, ttl time.Duration) (Lease, error) {
	return c.acquire(runLeaseKey(runID), owner, ttl, ErrRunLeaseHeld)
}

func (c *memoryCoordinator) Ready(_ context.Context) error { return nil }
func (c *memoryCoordinator) Mode() string                 { return "memory" }

func (c *memoryCoordinator) acquire(key, owner string, ttl time.Duration, conflict error) (Lease, error) {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}

	owner = strings.TrimSpace(owner)
	if owner == "" {
		owner = fmt.Sprintf("owner-%d", c.now().UnixNano())
	}

	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()

	if current, ok := c.locks[key]; ok {
		if current.expiresAt.After(now) {
			return nil, conflict
		}
		delete(c.locks, key)
	}

	c.locks[key] = memoryLockState{owner: owner, expiresAt: now.Add(ttl)}
	return &memoryLease{coordinator: c, key: key, owner: owner}, nil
}

func (l *memoryLease) Release(_ context.Context) error {
	l.coordinator.mu.Lock()
	defer l.coordinator.mu.Unlock()

	current, ok := l.coordinator.locks[l.key]
	if !ok {
		return nil
	}
	if current.owner != l.owner {
		return nil
	}
	delete(l.coordinator.locks, l.key)
	return nil
}

func (l *memoryLease) Key() string   { return l.key }
func (l *memoryLease) Owner() string { return l.owner }

type redisCoordinator struct {
	client *redis.Client
	prefix string
}

type redisLease struct {
	client *redis.Client
	key    string
	owner  string
}

func NewRedisCoordinator(addr string) Coordinator {
	return &redisCoordinator{
		client: redis.NewClient(&redis.Options{
			Addr:         strings.TrimSpace(addr),
			DialTimeout:  500 * time.Millisecond,
			ReadTimeout:  500 * time.Millisecond,
			WriteTimeout: 500 * time.Millisecond,
		}),
		prefix: "ralleh-flow",
	}
}

func (c *redisCoordinator) AcquireRepoLock(ctx context.Context, repoRoot, owner string, ttl time.Duration) (Lease, error) {
	return c.acquire(ctx, repoLockKey(repoRoot), owner, ttl, ErrRepositoryLocked)
}

func (c *redisCoordinator) AcquireRunLease(ctx context.Context, runID, owner string, ttl time.Duration) (Lease, error) {
	return c.acquire(ctx, runLeaseKey(runID), owner, ttl, ErrRunLeaseHeld)
}

func (c *redisCoordinator) acquire(ctx context.Context, key, owner string, ttl time.Duration, conflict error) (Lease, error) {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	owner = strings.TrimSpace(owner)
	if owner == "" {
		owner = fmt.Sprintf("owner-%d", time.Now().UnixNano())
	}

	redisKey := c.redisKey(key)
	ok, err := c.client.SetNX(ctx, redisKey, owner, ttl).Result()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, conflict
	}

	return &redisLease{client: c.client, key: redisKey, owner: owner}, nil
}

func (c *redisCoordinator) Ready(ctx context.Context) error {
		return c.client.Ping(ctx).Err()
}

func (c *redisCoordinator) Mode() string {
	return "redis"
}

func (c *redisCoordinator) redisKey(key string) string {
	return c.prefix + ":coord:" + key
}

func (l *redisLease) Release(ctx context.Context) error {
	const releaseScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`
	return l.client.Eval(ctx, releaseScript, []string{l.key}, l.owner).Err()
}

func (l *redisLease) Key() string   { return l.key }
func (l *redisLease) Owner() string { return l.owner }

func repoLockKey(repoRoot string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(repoRoot)))
	return "repo:" + hex.EncodeToString(sum[:])
}

func runLeaseKey(runID string) string {
	return "run:" + strings.TrimSpace(runID)
}
