package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type stubRunAdvancer struct {
	calls []string
	err   error
}

func (s *stubRunAdvancer) AdvancePendingRun(_ context.Context, runID string) (RunRecord, error) {
	s.calls = append(s.calls, runID)
	return RunRecord{ID: runID}, s.err
}

type stubRedisStreamClient struct {
	acks []string
}

func (c *stubRedisStreamClient) Ping(ctx context.Context) *redis.StatusCmd {
	return redis.NewStatusResult("PONG", nil)
}

func (c *stubRedisStreamClient) XGroupCreateMkStream(ctx context.Context, stream, group, start string) *redis.StatusCmd {
	return redis.NewStatusResult("OK", nil)
}

func (c *stubRedisStreamClient) XReadGroup(ctx context.Context, args *redis.XReadGroupArgs) *redis.XStreamSliceCmd {
	cmd := redis.NewXStreamSliceCmd(ctx)
	cmd.SetErr(redis.Nil)
	return cmd
}

func (c *stubRedisStreamClient) XAck(ctx context.Context, stream, group string, ids ...string) *redis.IntCmd {
	c.acks = append(c.acks, ids...)
	return redis.NewIntResult(int64(len(ids)), nil)
}

func TestRedisStreamOrchestratorAcknowledgesPendingRunAfterAdvance(t *testing.T) {
	client := &stubRedisStreamClient{}
	advancer := &stubRunAdvancer{}
	orchestrator := &redisStreamOrchestrator{
		client:        client,
		stream:        "ralleh-flow:runs",
		consumerGroup: "ralleh-flow-orchestrators",
		consumerName:  "worker-1",
		runAdvancer:   advancer,
		blockTime:     time.Millisecond,
		claimLimit:    1,
	}

	orchestrator.handleMessage(context.Background(), redis.XMessage{
		ID: "1-0",
		Values: map[string]any{
			"runId": "run_123",
			"type":  "run.pending",
		},
	})

	if len(advancer.calls) != 1 || advancer.calls[0] != "run_123" {
		t.Fatalf("expected advance call for run_123, got %#v", advancer.calls)
	}
	if len(client.acks) != 1 || client.acks[0] != "1-0" {
		t.Fatalf("expected ack for message 1-0, got %#v", client.acks)
	}
}

func TestRedisStreamOrchestratorLeavesMessagePendingOnUnexpectedAdvanceError(t *testing.T) {
	client := &stubRedisStreamClient{}
	advancer := &stubRunAdvancer{err: errors.New("boom")}
	orchestrator := &redisStreamOrchestrator{
		client:        client,
		stream:        "ralleh-flow:runs",
		consumerGroup: "ralleh-flow-orchestrators",
		consumerName:  "worker-1",
		runAdvancer:   advancer,
		blockTime:     time.Millisecond,
		claimLimit:    1,
	}

	orchestrator.handleMessage(context.Background(), redis.XMessage{
		ID: "1-1",
		Values: map[string]any{
			"runId": "run_456",
			"type":  "run.pending",
		},
	})

	if len(client.acks) != 0 {
		t.Fatalf("expected no ack on unexpected error, got %#v", client.acks)
	}
}

func TestRedisStreamOrchestratorAcknowledgesStateConflict(t *testing.T) {
	client := &stubRedisStreamClient{}
	advancer := &stubRunAdvancer{err: ErrRunStateConflict}
	orchestrator := &redisStreamOrchestrator{
		client:        client,
		stream:        "ralleh-flow:runs",
		consumerGroup: "ralleh-flow-orchestrators",
		consumerName:  "worker-1",
		runAdvancer:   advancer,
		blockTime:     time.Millisecond,
		claimLimit:    1,
	}

	orchestrator.handleMessage(context.Background(), redis.XMessage{
		ID: "1-2",
		Values: map[string]any{
			"runId": "run_789",
			"type":  "run.pending",
		},
	})

	if len(client.acks) != 1 || client.acks[0] != "1-2" {
		t.Fatalf("expected ack for state conflict, got %#v", client.acks)
	}
}
