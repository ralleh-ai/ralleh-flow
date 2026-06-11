package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Orchestrator interface {
	Start(ctx context.Context)
	Mode() string
}

type noopOrchestrator struct{}

func NewNoopOrchestrator() Orchestrator {
	return &noopOrchestrator{}
}

func (o *noopOrchestrator) Start(ctx context.Context) { <-ctx.Done() }
func (o *noopOrchestrator) Mode() string              { return "disabled" }

type runAdvancer interface {
	AdvancePendingRun(ctx context.Context, runID string) (RunRecord, error)
}

type redisStreamClient interface {
	Ping(ctx context.Context) *redis.StatusCmd
	XGroupCreateMkStream(ctx context.Context, stream, group, start string) *redis.StatusCmd
	XReadGroup(ctx context.Context, args *redis.XReadGroupArgs) *redis.XStreamSliceCmd
	XAck(ctx context.Context, stream, group string, ids ...string) *redis.IntCmd
}

type redisStreamOrchestrator struct {
	client        redisStreamClient
	stream        string
	consumerGroup string
	consumerName  string
	runAdvancer   runAdvancer
	blockTime     time.Duration
	claimLimit    int64
}

func NewRedisStreamOrchestrator(addr, stream, consumerGroup, consumerName string, runService *RunService) Orchestrator {
	return &redisStreamOrchestrator{
		client: redis.NewClient(&redis.Options{
			Addr:         strings.TrimSpace(addr),
			DialTimeout:  500 * time.Millisecond,
			ReadTimeout:  2 * time.Second,
			WriteTimeout: 2 * time.Second,
		}),
		stream:        defaultString(strings.TrimSpace(stream), "ralleh-flow:runs"),
		consumerGroup: defaultString(strings.TrimSpace(consumerGroup), "ralleh-flow-orchestrators"),
		consumerName:  defaultString(strings.TrimSpace(consumerName), fmt.Sprintf("orchestrator-%d", time.Now().UnixNano())),
		runAdvancer:   runService,
		blockTime:     2 * time.Second,
		claimLimit:    10,
	}
}

func (o *redisStreamOrchestrator) Start(ctx context.Context) {
	if o == nil || o.runAdvancer == nil {
		<-ctx.Done()
		return
	}

	for {
		if ctx.Err() != nil {
			return
		}

		if err := o.ensureGroup(ctx); err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
			}
			continue
		}

		if err := o.consumeOnce(ctx); err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
			}
		}
	}
}

func (o *redisStreamOrchestrator) Mode() string { return "redis-streams" }

func (o *redisStreamOrchestrator) ensureGroup(ctx context.Context) error {
	if err := o.client.Ping(ctx).Err(); err != nil {
		return err
	}
	err := o.client.XGroupCreateMkStream(ctx, o.stream, o.consumerGroup, "$").Err()
	if err == nil || strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}
	return err
}

func (o *redisStreamOrchestrator) consumeOnce(ctx context.Context) error {
	streams, err := o.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    o.consumerGroup,
		Consumer: o.consumerName,
		Streams:  []string{o.stream, ">"},
		Count:    o.claimLimit,
		Block:    o.blockTime,
		NoAck:    false,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) || strings.Contains(err.Error(), "context deadline exceeded") {
			return nil
		}
		return err
	}

	for _, stream := range streams {
		for _, message := range stream.Messages {
			o.handleMessage(ctx, message)
		}
	}
	return nil
}

func (o *redisStreamOrchestrator) handleMessage(ctx context.Context, message redis.XMessage) {
	runID, _ := message.Values["runId"].(string)
	eventType, _ := message.Values["type"].(string)

	ack := true
	if strings.TrimSpace(runID) != "" && eventType == "run.pending" {
		if _, err := o.runAdvancer.AdvancePendingRun(ctx, runID); err != nil && !errors.Is(err, ErrRunStateConflict) && !errors.Is(err, ErrRunLeaseHeld) && !errors.Is(err, ErrRunNotFound) {
			ack = false
		}
	}

	if ack {
		_, _ = o.client.XAck(ctx, o.stream, o.consumerGroup, message.ID).Result()
	}
}
