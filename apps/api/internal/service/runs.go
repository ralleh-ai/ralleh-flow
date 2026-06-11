package service

type TimelineEvent struct {
	At     string `json:"at"`
	Type   string `json:"type"`
	Detail string `json:"detail"`
}

type StepRecord struct {
	RunID      string `json:"runId"`
	StepID     string `json:"stepId"`
	Status     string `json:"status"`
	WorkerID   string `json:"workerId"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Agent      string `json:"agent,omitempty"`
}

type HandoffRecord struct {
	RunID             string `json:"runId"`
	StepID            string `json:"stepId"`
	Status            string `json:"status"`
	Kind              string `json:"kind,omitempty"`
	Agent             string `json:"agent,omitempty"`
	WorkerID          string `json:"workerId"`
	SessionID         string `json:"sessionId,omitempty"`
	UpstreamRunID     string `json:"upstreamRunId,omitempty"`
	DispatchAttemptAt string `json:"dispatchAttemptAt,omitempty"`
	FinishedAt        string `json:"finishedAt,omitempty"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

type RunRecord struct {
	ID           string          `json:"id"`
	WorkflowID   string          `json:"workflowId"`
	Status       string          `json:"status"`
	CurrentStep  string          `json:"currentStep"`
	Branch       string          `json:"branch"`
	WorktreePath string          `json:"worktreePath"`
	CreatedAt    string          `json:"createdAt"`
	Timeline     []TimelineEvent `json:"timeline"`
	Steps        []StepRecord    `json:"steps"`
	Handoffs     []HandoffRecord `json:"handoffs"`
}

type CreateRunInput struct {
	WorkflowID string            `json:"workflowId"`
	Variables  map[string]string `json:"variables"`
}
