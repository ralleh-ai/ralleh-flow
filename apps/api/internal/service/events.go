package service

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type EventBus interface {
	PublishRunEvent(ctx context.Context, runID string, event TimelineEvent) error
	Ready(ctx context.Context) error
	Mode() string
}

type memoryEventBus struct{}

func NewInMemoryEventBus() EventBus {
	return &memoryEventBus{}
}

func (b *memoryEventBus) PublishRunEvent(_ context.Context, _ string, _ TimelineEvent) error { return nil }
func (b *memoryEventBus) Ready(_ context.Context) error                                       { return nil }
func (b *memoryEventBus) Mode() string                                                        { return "memory" }

type redisEventBus struct {
	client        *redis.Client
	stream        string
	consumerGroup string
}

func NewRedisEventBus(addr, stream, consumerGroup string) EventBus {
	return &redisEventBus{
		client: redis.NewClient(&redis.Options{
			Addr:         strings.TrimSpace(addr),
			DialTimeout:  500 * time.Millisecond,
			ReadTimeout:  500 * time.Millisecond,
			WriteTimeout: 500 * time.Millisecond,
		}),
		stream:        defaultString(strings.TrimSpace(stream), "ralleh-flow:runs"),
		consumerGroup: defaultString(strings.TrimSpace(consumerGroup), "ralleh-flow-orchestrators"),
	}
}

func (b *redisEventBus) PublishRunEvent(ctx context.Context, runID string, event TimelineEvent) error {
	if err := b.Ready(ctx); err != nil {
		return err
	}

	return b.client.XAdd(ctx, &redis.XAddArgs{
		Stream: b.stream,
		Values: map[string]any{
			"runId":  runID,
			"at":     event.At,
			"type":   event.Type,
			"detail": event.Detail,
		},
	}).Err()
}

func (b *redisEventBus) Ready(ctx context.Context) error {
	if err := b.client.Ping(ctx).Err(); err != nil {
		return err
	}

	err := b.client.XGroupCreateMkStream(ctx, b.stream, b.consumerGroup, "$").Err()
	if err == nil || strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}
	return err
}

func (b *redisEventBus) Mode() string { return "redis-streams" }

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
