package service

import (
	"context"
	"strings"
)

// TaskEventPublisher is the outbound integration boundary for ralleh-tasks.
//
// It receives lifecycle facts from ralleh-flow while flow remains source-of-truth
// for orchestration state.
type TaskEventPublisher interface {
	PublishTaskEvent(ctx context.Context, event TaskLifecycleEvent) error
	Mode() string
}

type TaskLifecycleEvent struct {
	RunID     string `json:"runId"`
	Type      string `json:"type"`
	At        string `json:"at"`
	Detail    string `json:"detail"`
	Source    string `json:"source"`
	WorkerID  string `json:"workerId,omitempty"`
	StepID    string `json:"stepId,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
}

type noopTaskEventPublisher struct{}

func NewNoopTaskEventPublisher() TaskEventPublisher {
	return &noopTaskEventPublisher{}
}

func (p *noopTaskEventPublisher) PublishTaskEvent(_ context.Context, _ TaskLifecycleEvent) error {
	return nil
}

func (p *noopTaskEventPublisher) Mode() string { return "noop" }

// SecretResolver is the outbound integration boundary for ralleh-keys.
//
// It resolves key references at execution time without persisting secret values
// in run metadata.
type SecretResolver interface {
	ResolveSecret(ctx context.Context, keyRef string, run RunRecord, step StepRecord) (string, error)
	Mode() string
}

type noopSecretResolver struct{}

func NewNoopSecretResolver() SecretResolver {
	return &noopSecretResolver{}
}

func (r *noopSecretResolver) ResolveSecret(_ context.Context, _ string, _ RunRecord, _ StepRecord) (string, error) {
	return "", nil
}

func (r *noopSecretResolver) Mode() string { return "noop" }

// ContextProvider is the outbound integration boundary for Engram retrieval.
//
// It provides optional external context bundles while flow stores only metadata
// references in runtime artifacts.
type ContextProvider interface {
	RetrieveContext(ctx context.Context, query string, run RunRecord, step StepRecord, maxTokens int) (ContextBundle, error)
	Mode() string
}

type ContextBundle struct {
	Query   string   `json:"query"`
	Entries []string `json:"entries"`
	Source  string   `json:"source"`
}

type noopContextProvider struct{}

func NewNoopContextProvider() ContextProvider {
	return &noopContextProvider{}
}

func (p *noopContextProvider) RetrieveContext(_ context.Context, query string, _ RunRecord, _ StepRecord, _ int) (ContextBundle, error) {
	return ContextBundle{Query: strings.TrimSpace(query), Entries: []string{}, Source: "none"}, nil
}

func (p *noopContextProvider) Mode() string { return "noop" }
