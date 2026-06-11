package service

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type stubDispatcher struct {
	result DispatchResult
	err    error
	mode   string
	last   DispatchRequest
}

func (d *stubDispatcher) DispatchAgentTask(ctx context.Context, req DispatchRequest) (DispatchResult, error) {
	_ = ctx
	d.last = req
	if d.err != nil {
		return DispatchResult{}, d.err
	}
	return d.result, nil
}

func (d *stubDispatcher) Mode() string {
	if d.mode == "" {
		return "stub"
	}
	return d.mode
}

func TestGenerateRunIDUniqueForSameTimestamp(t *testing.T) {
	now := time.Date(2026, time.June, 11, 8, 12, 0, 123456789, time.UTC)

	first := generateRunID(now)
	second := generateRunID(now)

	if first == second {
		t.Fatalf("expected unique run ids for same timestamp, got identical values: %s", first)
	}
}

func TestRunServiceCreateRequiresGitExecutionRepo(t *testing.T) {
	repoRoot := t.TempDir()
	writeWorkflowFixture(t, repoRoot)

	service := NewRunService(filepath.Join(repoRoot, "data", "ralleh-flow.db"), repoRoot, NewWorkflowService(repoRoot))

	_, err := service.Create(context.Background(), CreateRunInput{
		WorkflowID: "feature-development",
		Variables: map[string]string{
			"feature_name": "Flow polish",
			"target_repo":  "ralleh-flow",
		},
	})
	if !errors.Is(err, ErrExecutionRepoNotReady) {
		t.Fatalf("expected ErrExecutionRepoNotReady, got %v", err)
	}
}

func TestRunServiceCreatePreparesIsolatedGitWorktree(t *testing.T) {
	repoRoot := t.TempDir()
	writeWorkflowFixture(t, repoRoot)
	initGitRepoWithCommit(t, repoRoot)

	service := NewRunService(filepath.Join(repoRoot, "data", "ralleh-flow.db"), repoRoot, NewWorkflowService(repoRoot))

	run, err := service.Create(context.Background(), CreateRunInput{
		WorkflowID: "feature-development",
		Variables: map[string]string{
			"feature_name": "Flow polish",
			"target_repo":  "ralleh-flow",
		},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	if !strings.Contains(run.Branch, run.ID) {
		t.Fatalf("expected branch to contain run id, got %q", run.Branch)
	}
	if run.Status != "pending" {
		t.Fatalf("expected run status pending after preparation, got %q", run.Status)
	}
	if len(run.Timeline) < 3 {
		t.Fatalf("expected at least 3 timeline events, got %#v", run.Timeline)
	}
	last := run.Timeline[len(run.Timeline)-1]
	if last.Type != "run.pending" {
		t.Fatalf("expected last event run.pending, got %#v", last)
	}

	gitDirPath := filepath.Join(run.WorktreePath, ".git")
	gitDir, err := os.ReadFile(gitDirPath)
	if err != nil {
		t.Fatalf("read worktree .git file: %v", err)
	}
	if !strings.Contains(string(gitDir), ".git/worktrees/") {
		t.Fatalf("expected worktree gitdir, got %s", string(gitDir))
	}

	for _, dir := range []string{"logs", "snapshots", "handoffs", "artifacts"} {
		path := filepath.Join(repoRoot, "runtime", "runs", run.ID, dir)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
}

func TestRunStoreTransitionRunStateUpdatesStatusAndTimelineAtomically(t *testing.T) {
	repoRoot := t.TempDir()
	store := NewRunStore(filepath.Join(repoRoot, "data", "ralleh-flow.db"))
	ctx := context.Background()
	run := RunRecord{
		ID:           "run_test_transition",
		WorkflowID:   "feature-development",
		Status:       "preparing",
		CurrentStep:  "",
		Branch:       "workflow/feature-development/run_test_transition",
		WorktreePath: filepath.Join(repoRoot, "runtime", "runs", "run_test_transition", "workspace"),
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	if err := store.Create(ctx, run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := store.AppendTimeline(ctx, run.ID, []TimelineEvent{{At: run.CreatedAt, Type: "run.created", Detail: "created"}}); err != nil {
		t.Fatalf("append initial timeline: %v", err)
	}

	event := TimelineEvent{At: time.Now().UTC().Format(time.RFC3339), Type: "run.pending", Detail: "Run is queued for orchestration"}
	if err := store.TransitionRunState(ctx, run.ID, "pending", "research", event); err != nil {
		t.Fatalf("transition run state: %v", err)
	}

	stored, found, err := store.Get(ctx, run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if !found {
		t.Fatal("expected stored run to exist")
	}
	if stored.Status != "pending" {
		t.Fatalf("expected status pending, got %q", stored.Status)
	}
	if stored.CurrentStep != "research" {
		t.Fatalf("expected current step research, got %q", stored.CurrentStep)
	}
	if len(stored.Timeline) != 2 {
		t.Fatalf("expected 2 timeline events, got %#v", stored.Timeline)
	}
	if stored.Timeline[1].Type != "run.pending" {
		t.Fatalf("expected second event run.pending, got %#v", stored.Timeline[1])
	}
}

func TestRunStoreStartStepRecordsActiveStepAtomically(t *testing.T) {
	repoRoot := t.TempDir()
	store := NewRunStore(filepath.Join(repoRoot, "data", "ralleh-flow.db"))
	ctx := context.Background()
	run := RunRecord{
		ID:           "run_test_step_start",
		WorkflowID:   "feature-development",
		Status:       "running",
		CurrentStep:  "research",
		Branch:       "workflow/feature-development/run_test_step_start",
		WorktreePath: filepath.Join(repoRoot, "runtime", "runs", "run_test_step_start", "workspace"),
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	if err := store.Create(ctx, run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	step := StepRecord{
		RunID:     run.ID,
		StepID:    "research",
		Status:    "running",
		WorkerID:  "worker-123",
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		Kind:      "agent_task",
		Agent:     "carmack",
	}
	handoff := HandoffRecord{
		RunID:     run.ID,
		StepID:    step.StepID,
		Status:    "claimed",
		Kind:      step.Kind,
		Agent:     step.Agent,
		WorkerID:  step.WorkerID,
		CreatedAt: step.StartedAt,
		UpdatedAt: step.StartedAt,
	}
	if err := store.StartStep(ctx, run.ID, step, handoff, []TimelineEvent{{At: step.StartedAt, Type: "step.started", Detail: "Step research claimed by worker-123"}}); err != nil {
		t.Fatalf("start step: %v", err)
	}

	stored, found, err := store.Get(ctx, run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if !found {
		t.Fatal("expected stored run to exist")
	}
	if len(stored.Steps) != 1 {
		t.Fatalf("expected one step record, got %#v", stored.Steps)
	}
	if stored.Steps[0].StepID != "research" || stored.Steps[0].WorkerID != "worker-123" {
		t.Fatalf("unexpected step record %#v", stored.Steps[0])
	}
	if len(stored.Timeline) != 1 || stored.Timeline[0].Type != "step.started" {
		t.Fatalf("expected step.started timeline, got %#v", stored.Timeline)
	}
}

func TestRunStoreCompareAndSwapRunStateRejectsUnexpectedStatus(t *testing.T) {
	repoRoot := t.TempDir()
	store := NewRunStore(filepath.Join(repoRoot, "data", "ralleh-flow.db"))
	ctx := context.Background()
	run := RunRecord{
		ID:           "run_test_compare_and_swap",
		WorkflowID:   "feature-development",
		Status:       "pending",
		CurrentStep:  "research",
		Branch:       "workflow/feature-development/run_test_compare_and_swap",
		WorktreePath: filepath.Join(repoRoot, "runtime", "runs", "run_test_compare_and_swap", "workspace"),
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	if err := store.Create(ctx, run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	event := TimelineEvent{At: time.Now().UTC().Format(time.RFC3339), Type: "run.started", Detail: "started"}
	if err := store.CompareAndSwapRunState(ctx, run.ID, "preparing", "running", run.CurrentStep, event); !errors.Is(err, ErrRunStateConflict) {
		t.Fatalf("expected ErrRunStateConflict, got %v", err)
	}

	stored, found, err := store.Get(ctx, run.ID)
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if !found {
		t.Fatal("expected stored run to exist")
	}
	if stored.Status != "pending" {
		t.Fatalf("expected status to remain pending, got %q", stored.Status)
	}
	if len(stored.Timeline) != 0 {
		t.Fatalf("expected no timeline mutation on conflict, got %#v", stored.Timeline)
	}
}

func TestRunServiceAdvancePendingRunMarksRunRunning(t *testing.T) {
	repoRoot := t.TempDir()
	writeWorkflowFixture(t, repoRoot)
	initGitRepoWithCommit(t, repoRoot)

	service := NewRunService(filepath.Join(repoRoot, "data", "ralleh-flow.db"), repoRoot, NewWorkflowService(repoRoot))

	run, err := service.Create(context.Background(), CreateRunInput{
		WorkflowID: "feature-development",
		Variables: map[string]string{
			"feature_name": "Flow polish",
			"target_repo":  "ralleh-flow",
		},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	advanced, err := service.AdvancePendingRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("advance pending run: %v", err)
	}
	if advanced.Status != "running" {
		t.Fatalf("expected running status, got %q", advanced.Status)
	}
	if advanced.CurrentStep != "research" {
		t.Fatalf("expected current step research, got %q", advanced.CurrentStep)
	}
	if len(advanced.Steps) != 1 {
		t.Fatalf("expected 1 active step record, got %#v", advanced.Steps)
	}
	if advanced.Steps[0].StepID != "research" {
		t.Fatalf("expected active step research, got %#v", advanced.Steps[0])
	}
	if advanced.Steps[0].Status != "running" {
		t.Fatalf("expected step status running, got %#v", advanced.Steps[0])
	}
	if advanced.Steps[0].WorkerID == "" {
		t.Fatalf("expected worker id on step record, got %#v", advanced.Steps[0])
	}
	if len(advanced.Timeline) < 6 {
		t.Fatalf("expected at least six timeline events, got %#v", advanced.Timeline)
	}
	lastThree := advanced.Timeline[len(advanced.Timeline)-3:]
	if lastThree[0].Type != "run.started" || lastThree[1].Type != "step.started" || lastThree[2].Type != "step.handoff.claimed" {
		t.Fatalf("expected final lifecycle events run.started -> step.started -> step.handoff.claimed, got %#v", lastThree)
	}
}

func TestRunServiceAdvancePendingRunPersistsStepHandoff(t *testing.T) {
	repoRoot := t.TempDir()
	writeWorkflowFixture(t, repoRoot)
	initGitRepoWithCommit(t, repoRoot)

	service := NewRunService(filepath.Join(repoRoot, "data", "ralleh-flow.db"), repoRoot, NewWorkflowService(repoRoot))

	run, err := service.Create(context.Background(), CreateRunInput{
		WorkflowID: "feature-development",
		Variables: map[string]string{
			"feature_name": "Flow polish",
			"target_repo":  "ralleh-flow",
		},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	advanced, err := service.AdvancePendingRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("advance pending run: %v", err)
	}
	if len(advanced.Handoffs) != 1 {
		t.Fatalf("expected one handoff record, got %#v", advanced.Handoffs)
	}
	handoff := advanced.Handoffs[0]
	if handoff.StepID != "research" {
		t.Fatalf("expected research handoff, got %#v", handoff)
	}
	if handoff.Status != "claimed" {
		t.Fatalf("expected claimed handoff, got %#v", handoff)
	}
	if handoff.Kind != "agent_task" {
		t.Fatalf("expected handoff kind agent_task, got %#v", handoff)
	}
	if handoff.WorkerID == "" {
		t.Fatalf("expected worker id on handoff, got %#v", handoff)
	}

	handoffPath := filepath.Join(repoRoot, "runtime", "runs", run.ID, "handoffs", "research-claimed.json")
	contents, err := os.ReadFile(handoffPath)
	if err != nil {
		t.Fatalf("read handoff artifact: %v", err)
	}
	if !strings.Contains(string(contents), "claimed") || !strings.Contains(string(contents), "research") {
		t.Fatalf("expected claimed handoff payload, got %s", string(contents))
	}
}

func TestRunServiceDispatchActiveStepUsesDispatcherAdapter(t *testing.T) {
	repoRoot := t.TempDir()
	writeWorkflowFixture(t, repoRoot)
	initGitRepoWithCommit(t, repoRoot)

	dispatcher := &stubDispatcher{
		mode:   "stub-dispatcher",
		result: DispatchResult{SessionID: "session:from-dispatcher", Note: "adapter decided binding"},
	}
	service := NewRunService(filepath.Join(repoRoot, "data", "ralleh-flow.db"), repoRoot, NewWorkflowService(repoRoot)).WithDispatcher(dispatcher)

	run, err := service.Create(context.Background(), CreateRunInput{
		WorkflowID: "feature-development",
		Variables: map[string]string{
			"feature_name": "Flow polish",
			"target_repo":  "ralleh-flow",
		},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if _, err := service.AdvancePendingRun(context.Background(), run.ID); err != nil {
		t.Fatalf("advance pending run: %v", err)
	}

	dispatched, err := service.DispatchActiveStep(context.Background(), run.ID, HandoffDispatchInput{Note: "use adapter"})
	if err != nil {
		t.Fatalf("dispatch active step: %v", err)
	}
	if dispatcher.last.Run.ID != run.ID || dispatcher.last.Step.StepID != "research" {
		t.Fatalf("expected dispatcher to receive run/step context, got %#v", dispatcher.last)
	}
	if len(dispatched.Handoffs) != 1 || dispatched.Handoffs[0].SessionID != "session:from-dispatcher" {
		t.Fatalf("expected dispatcher-provided session binding, got %#v", dispatched.Handoffs)
	}
	handoffDoc := filepath.Join(repoRoot, "runtime", "runs", run.ID, "workspace", "HANDOFF.md")
	docContents, err := os.ReadFile(handoffDoc)
	if err != nil {
		t.Fatalf("read handoff document: %v", err)
	}
	if !strings.Contains(string(docContents), "# HANDOFF") || !strings.Contains(string(docContents), "use adapter") {
		t.Fatalf("expected handoff document contents, got %s", string(docContents))
	}
	if dispatcher.last.HandoffDocumentPath != handoffDoc {
		t.Fatalf("expected dispatcher to receive handoff doc path, got %#v", dispatcher.last)
	}

	handoffPath := filepath.Join(repoRoot, "runtime", "runs", run.ID, "handoffs", "research-dispatched.json")
	contents, err := os.ReadFile(handoffPath)
	if err != nil {
		t.Fatalf("read dispatched handoff artifact: %v", err)
	}
	if !strings.Contains(string(contents), "stub-dispatcher") || !strings.Contains(string(contents), "session:from-dispatcher") {
		t.Fatalf("expected dispatcher artifact payload, got %s", string(contents))
	}
}

func TestRunServiceDispatchActiveStepPersistsSessionBinding(t *testing.T) {
	repoRoot := t.TempDir()
	writeWorkflowFixture(t, repoRoot)
	initGitRepoWithCommit(t, repoRoot)

	service := NewRunService(filepath.Join(repoRoot, "data", "ralleh-flow.db"), repoRoot, NewWorkflowService(repoRoot))

	run, err := service.Create(context.Background(), CreateRunInput{
		WorkflowID: "feature-development",
		Variables: map[string]string{
			"feature_name": "Flow polish",
			"target_repo":  "ralleh-flow",
		},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if _, err := service.AdvancePendingRun(context.Background(), run.ID); err != nil {
		t.Fatalf("advance pending run: %v", err)
	}

	dispatched, err := service.DispatchActiveStep(context.Background(), run.ID, HandoffDispatchInput{
		SessionID: "session:abc123",
		Note:      "OpenClaw handoff created",
	})
	if err != nil {
		t.Fatalf("dispatch active step: %v", err)
	}
	if len(dispatched.Handoffs) != 1 {
		t.Fatalf("expected one handoff record, got %#v", dispatched.Handoffs)
	}
	handoff := dispatched.Handoffs[0]
	if handoff.Status != "dispatched" {
		t.Fatalf("expected dispatched handoff, got %#v", handoff)
	}
	if handoff.SessionID != "session:abc123" {
		t.Fatalf("expected session binding, got %#v", handoff)
	}
	if handoff.UpstreamRunID != "" {
		t.Fatalf("expected empty upstream run id for noop dispatch, got %#v", handoff)
	}
	if got := dispatched.Timeline[len(dispatched.Timeline)-1].Type; got != "step.handoff.dispatched" {
		t.Fatalf("expected final event step.handoff.dispatched, got %#v", dispatched.Timeline)
	}

	handoffPath := filepath.Join(repoRoot, "runtime", "runs", run.ID, "handoffs", "research-dispatched.json")
	contents, err := os.ReadFile(handoffPath)
	if err != nil {
		t.Fatalf("read dispatched handoff artifact: %v", err)
	}
	if !strings.Contains(string(contents), "session:abc123") || !strings.Contains(string(contents), "dispatched") {
		t.Fatalf("expected dispatched handoff payload, got %s", string(contents))
	}
}

func TestRunServiceDispatchActiveStepPersistsUpstreamRunID(t *testing.T) {
	repoRoot := t.TempDir()
	writeWorkflowFixture(t, repoRoot)
	initGitRepoWithCommit(t, repoRoot)

	dispatcher := &stubDispatcher{
		mode:   "openclaw-hooks",
		result: DispatchResult{SessionID: "hook:ralleh-flow:run-123:research", UpstreamRunID: "hook-run-999", Note: "hook dispatched"},
	}
	service := NewRunService(filepath.Join(repoRoot, "data", "ralleh-flow.db"), repoRoot, NewWorkflowService(repoRoot)).WithDispatcher(dispatcher)

	run, err := service.Create(context.Background(), CreateRunInput{
		WorkflowID: "feature-development",
		Variables: map[string]string{
			"feature_name": "Flow polish",
			"target_repo":  "ralleh-flow",
		},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if _, err := service.AdvancePendingRun(context.Background(), run.ID); err != nil {
		t.Fatalf("advance pending run: %v", err)
	}

	dispatched, err := service.DispatchActiveStep(context.Background(), run.ID, HandoffDispatchInput{Note: "hook dispatch"})
	if err != nil {
		t.Fatalf("dispatch active step: %v", err)
	}
	if len(dispatched.Handoffs) != 1 {
		t.Fatalf("expected one handoff record, got %#v", dispatched.Handoffs)
	}
	if dispatched.Handoffs[0].UpstreamRunID != "hook-run-999" {
		t.Fatalf("expected upstream run id persisted, got %#v", dispatched.Handoffs[0])
	}

	handoffPath := filepath.Join(repoRoot, "runtime", "runs", run.ID, "handoffs", "research-dispatched.json")
	contents, err := os.ReadFile(handoffPath)
	if err != nil {
		t.Fatalf("read dispatched handoff artifact: %v", err)
	}
	if !strings.Contains(string(contents), "hook-run-999") || !strings.Contains(string(contents), "HANDOFF.md") {
		t.Fatalf("expected upstream run id and handoff doc path in artifact, got %s", string(contents))
	}
}

func TestRunServiceDispatchActiveStepRejectsDuplicateDispatch(t *testing.T) {
	repoRoot := t.TempDir()
	writeWorkflowFixture(t, repoRoot)
	initGitRepoWithCommit(t, repoRoot)

	service := NewRunService(filepath.Join(repoRoot, "data", "ralleh-flow.db"), repoRoot, NewWorkflowService(repoRoot))

	run, err := service.Create(context.Background(), CreateRunInput{
		WorkflowID: "feature-development",
		Variables: map[string]string{
			"feature_name": "Flow polish",
			"target_repo":  "ralleh-flow",
		},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if _, err := service.AdvancePendingRun(context.Background(), run.ID); err != nil {
		t.Fatalf("advance pending run: %v", err)
	}
	if _, err := service.DispatchActiveStep(context.Background(), run.ID, HandoffDispatchInput{SessionID: "session:first"}); err != nil {
		t.Fatalf("first dispatch: %v", err)
	}
	if _, err := service.DispatchActiveStep(context.Background(), run.ID, HandoffDispatchInput{SessionID: "session:second"}); !errors.Is(err, ErrRunStateConflict) {
		t.Fatalf("expected ErrRunStateConflict on duplicate dispatch, got %v", err)
	}
}

func TestRunServiceCompleteActiveStepQueuesNextAgentTask(t *testing.T) {
	repoRoot := t.TempDir()
	writeWorkflowFixture(t, repoRoot)
	initGitRepoWithCommit(t, repoRoot)

	service := NewRunService(filepath.Join(repoRoot, "data", "ralleh-flow.db"), repoRoot, NewWorkflowService(repoRoot))

	run, err := service.Create(context.Background(), CreateRunInput{
		WorkflowID: "feature-development",
		Variables: map[string]string{
			"feature_name": "Flow polish",
			"target_repo":  "ralleh-flow",
		},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if _, err := service.AdvancePendingRun(context.Background(), run.ID); err != nil {
		t.Fatalf("advance pending run: %v", err)
	}
	if _, err := service.DispatchActiveStep(context.Background(), run.ID, HandoffDispatchInput{
		SessionID: "session:complete-test",
		Note:      "dispatch before complete",
	}); err != nil {
		t.Fatalf("dispatch active step: %v", err)
	}

	completed, err := service.CompleteActiveStep(context.Background(), run.ID, StepCompletionInput{Summary: "Research completed", CheckpointLabel: "research-summary"})
	if err != nil {
		t.Fatalf("complete active step: %v", err)
	}
	if completed.Status != "pending" {
		t.Fatalf("expected run status pending for next agent step, got %q", completed.Status)
	}
	if completed.CurrentStep != "implement" {
		t.Fatalf("expected current step implement, got %q", completed.CurrentStep)
	}
	if len(completed.Steps) != 1 {
		t.Fatalf("expected one completed step record before next advance, got %#v", completed.Steps)
	}
	if completed.Steps[0].Status != "completed" {
		t.Fatalf("expected completed step record, got %#v", completed.Steps[0])
	}
	if completed.Steps[0].FinishedAt == "" {
		t.Fatalf("expected finished_at to be set, got %#v", completed.Steps[0])
	}
	if got := completed.Timeline[len(completed.Timeline)-1].Type; got != "run.pending" {
		t.Fatalf("expected final event run.pending, got %#v", completed.Timeline)
	}
	if len(completed.Handoffs) != 1 || completed.Handoffs[0].Status != "completed" || completed.Handoffs[0].FinishedAt == "" {
		t.Fatalf("expected completed handoff record, got %#v", completed.Handoffs)
	}
	if completed.Timeline[len(completed.Timeline)-2].Type != "step.completed" || completed.Timeline[len(completed.Timeline)-3].Type != "step.handoff.completed" {
		t.Fatalf("expected handoff then step completion before requeue, got %#v", completed.Timeline)
	}

	checkpointPath := filepath.Join(repoRoot, "runtime", "runs", run.ID, "handoffs", "research-summary.json")
	contents, err := os.ReadFile(checkpointPath)
	if err != nil {
		t.Fatalf("read checkpoint: %v", err)
	}
	if !strings.Contains(string(contents), "Research completed") || !strings.Contains(string(contents), "session:complete-test") {
		t.Fatalf("expected checkpoint summary and session in %s, got %s", checkpointPath, string(contents))
	}
}

func TestRunServiceCompleteActiveStepRequestsApprovalForHumanGate(t *testing.T) {
	repoRoot := t.TempDir()
	writeWorkflowFixture(t, repoRoot)
	initGitRepoWithCommit(t, repoRoot)

	service := NewRunService(filepath.Join(repoRoot, "data", "ralleh-flow.db"), repoRoot, NewWorkflowService(repoRoot))

	run, err := service.Create(context.Background(), CreateRunInput{
		WorkflowID: "feature-development",
		Variables: map[string]string{
			"feature_name": "Flow polish",
			"target_repo":  "ralleh-flow",
		},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if _, err := service.AdvancePendingRun(context.Background(), run.ID); err != nil {
		t.Fatalf("advance research step: %v", err)
	}
	if _, err := service.DispatchActiveStep(context.Background(), run.ID, HandoffDispatchInput{SessionID: "session:research"}); err != nil {
		t.Fatalf("dispatch research step: %v", err)
	}
	if _, err := service.CompleteActiveStep(context.Background(), run.ID, StepCompletionInput{Summary: "Research completed"}); err != nil {
		t.Fatalf("complete research step: %v", err)
	}
	if _, err := service.AdvancePendingRun(context.Background(), run.ID); err != nil {
		t.Fatalf("advance implement step: %v", err)
	}
	if _, err := service.DispatchActiveStep(context.Background(), run.ID, HandoffDispatchInput{SessionID: "session:implement"}); err != nil {
		t.Fatalf("dispatch implement step: %v", err)
	}

	completed, err := service.CompleteActiveStep(context.Background(), run.ID, StepCompletionInput{Summary: "Implementation completed", CheckpointLabel: "implement-summary"})
	if err != nil {
		t.Fatalf("complete implement step: %v", err)
	}
	if completed.Status != "waiting_for_approval" {
		t.Fatalf("expected waiting_for_approval, got %q", completed.Status)
	}
	if completed.CurrentStep != "review" {
		t.Fatalf("expected current step review, got %q", completed.CurrentStep)
	}
	if got := completed.Timeline[len(completed.Timeline)-1].Type; got != "approval.requested" {
		t.Fatalf("expected final event approval.requested, got %#v", completed.Timeline)
	}
}

func TestRunServiceFailActiveStepMarksRunFailed(t *testing.T) {
	repoRoot := t.TempDir()
	writeWorkflowFixture(t, repoRoot)
	initGitRepoWithCommit(t, repoRoot)

	service := NewRunService(filepath.Join(repoRoot, "data", "ralleh-flow.db"), repoRoot, NewWorkflowService(repoRoot))

	run, err := service.Create(context.Background(), CreateRunInput{
		WorkflowID: "feature-development",
		Variables: map[string]string{
			"feature_name": "Flow polish",
			"target_repo":  "ralleh-flow",
		},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if _, err := service.AdvancePendingRun(context.Background(), run.ID); err != nil {
		t.Fatalf("advance pending run: %v", err)
	}
	if _, err := service.DispatchActiveStep(context.Background(), run.ID, HandoffDispatchInput{
		SessionID: "session:fail-test",
		Note:      "dispatch before fail",
	}); err != nil {
		t.Fatalf("dispatch active step: %v", err)
	}

	failed, err := service.FailActiveStep(context.Background(), run.ID, StepFailureInput{Reason: "agent dispatch not wired yet"})
	if err != nil {
		t.Fatalf("fail active step: %v", err)
	}
	if failed.Status != "failed" {
		t.Fatalf("expected failed run status, got %q", failed.Status)
	}
	if len(failed.Steps) != 1 || failed.Steps[0].Status != "failed" {
		t.Fatalf("expected failed step record, got %#v", failed.Steps)
	}
	if failed.Timeline[len(failed.Timeline)-1].Type != "run.failed" {
		t.Fatalf("expected final event run.failed, got %#v", failed.Timeline)
	}
	if len(failed.Handoffs) != 1 || failed.Handoffs[0].Status != "failed" || failed.Handoffs[0].FinishedAt == "" {
		t.Fatalf("expected failed handoff record, got %#v", failed.Handoffs)
	}
	if failed.Timeline[len(failed.Timeline)-3].Type != "step.handoff.failed" {
		t.Fatalf("expected handoff failure event before step/run failure, got %#v", failed.Timeline)
	}
	failurePath := filepath.Join(repoRoot, "runtime", "runs", run.ID, "handoffs", "research-failed.json")
	contents, err := os.ReadFile(failurePath)
	if err != nil {
		t.Fatalf("read failure checkpoint: %v", err)
	}
	if !strings.Contains(string(contents), "agent dispatch not wired yet") || !strings.Contains(string(contents), "session:fail-test") {
		t.Fatalf("expected failure checkpoint payload, got %s", string(contents))
	}
}

func TestRunServiceCompleteActiveStepRequiresDispatchedProofForAgentTask(t *testing.T) {
	repoRoot := t.TempDir()
	writeWorkflowFixture(t, repoRoot)
	initGitRepoWithCommit(t, repoRoot)

	service := NewRunService(filepath.Join(repoRoot, "data", "ralleh-flow.db"), repoRoot, NewWorkflowService(repoRoot))

	run, err := service.Create(context.Background(), CreateRunInput{
		WorkflowID: "feature-development",
		Variables: map[string]string{
			"feature_name": "Flow polish",
			"target_repo":  "ralleh-flow",
		},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if _, err := service.AdvancePendingRun(context.Background(), run.ID); err != nil {
		t.Fatalf("advance pending run: %v", err)
	}

	_, err = service.CompleteActiveStep(context.Background(), run.ID, StepCompletionInput{Summary: "done"})
	if !errors.Is(err, ErrRunStateConflict) {
		t.Fatalf("expected ErrRunStateConflict when completing without dispatch proof, got %v", err)
	}
	if !strings.Contains(err.Error(), "requires dispatched handoff proof") {
		t.Fatalf("expected dispatched-proof detail, got %v", err)
	}
}

func TestRunServiceFailActiveStepRequiresDispatchedProofForAgentTask(t *testing.T) {
	repoRoot := t.TempDir()
	writeWorkflowFixture(t, repoRoot)
	initGitRepoWithCommit(t, repoRoot)

	service := NewRunService(filepath.Join(repoRoot, "data", "ralleh-flow.db"), repoRoot, NewWorkflowService(repoRoot))

	run, err := service.Create(context.Background(), CreateRunInput{
		WorkflowID: "feature-development",
		Variables: map[string]string{
			"feature_name": "Flow polish",
			"target_repo":  "ralleh-flow",
		},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if _, err := service.AdvancePendingRun(context.Background(), run.ID); err != nil {
		t.Fatalf("advance pending run: %v", err)
	}

	_, err = service.FailActiveStep(context.Background(), run.ID, StepFailureInput{Reason: "boom"})
	if !errors.Is(err, ErrRunStateConflict) {
		t.Fatalf("expected ErrRunStateConflict when failing without dispatch proof, got %v", err)
	}
	if !strings.Contains(err.Error(), "requires dispatched handoff proof") {
		t.Fatalf("expected dispatched-proof detail, got %v", err)
	}
}

func TestRunServiceCompleteActiveStepRejectsMismatchedCallbackSession(t *testing.T) {
	repoRoot := t.TempDir()
	writeWorkflowFixture(t, repoRoot)
	initGitRepoWithCommit(t, repoRoot)

	dispatcher := &stubDispatcher{
		mode:   "openclaw-hooks",
		result: DispatchResult{SessionID: "session:expected", UpstreamRunID: "hook-run-expected", Note: "hook dispatched"},
	}
	service := NewRunService(filepath.Join(repoRoot, "data", "ralleh-flow.db"), repoRoot, NewWorkflowService(repoRoot)).WithDispatcher(dispatcher)

	run, err := service.Create(context.Background(), CreateRunInput{
		WorkflowID: "feature-development",
		Variables: map[string]string{
			"feature_name": "Flow polish",
			"target_repo":  "ralleh-flow",
		},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if _, err := service.AdvancePendingRun(context.Background(), run.ID); err != nil {
		t.Fatalf("advance pending run: %v", err)
	}
	if _, err := service.DispatchActiveStep(context.Background(), run.ID, HandoffDispatchInput{}); err != nil {
		t.Fatalf("dispatch active step: %v", err)
	}

	_, err = service.CompleteActiveStep(context.Background(), run.ID, StepCompletionInput{
		Summary:       "done",
		SessionID:     "session:wrong",
		UpstreamRunID: "hook-run-expected",
	})
	if !errors.Is(err, ErrRunStateConflict) {
		t.Fatalf("expected ErrRunStateConflict for mismatched session, got %v", err)
	}
}

func TestRunServiceFailActiveStepRejectsMismatchedCallbackUpstreamRunID(t *testing.T) {
	repoRoot := t.TempDir()
	writeWorkflowFixture(t, repoRoot)
	initGitRepoWithCommit(t, repoRoot)

	dispatcher := &stubDispatcher{
		mode:   "openclaw-hooks",
		result: DispatchResult{SessionID: "session:expected", UpstreamRunID: "hook-run-expected", Note: "hook dispatched"},
	}
	service := NewRunService(filepath.Join(repoRoot, "data", "ralleh-flow.db"), repoRoot, NewWorkflowService(repoRoot)).WithDispatcher(dispatcher)

	run, err := service.Create(context.Background(), CreateRunInput{
		WorkflowID: "feature-development",
		Variables: map[string]string{
			"feature_name": "Flow polish",
			"target_repo":  "ralleh-flow",
		},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if _, err := service.AdvancePendingRun(context.Background(), run.ID); err != nil {
		t.Fatalf("advance pending run: %v", err)
	}
	if _, err := service.DispatchActiveStep(context.Background(), run.ID, HandoffDispatchInput{}); err != nil {
		t.Fatalf("dispatch active step: %v", err)
	}

	_, err = service.FailActiveStep(context.Background(), run.ID, StepFailureInput{
		Reason:        "boom",
		SessionID:     "session:expected",
		UpstreamRunID: "hook-run-wrong",
	})
	if !errors.Is(err, ErrRunStateConflict) {
		t.Fatalf("expected ErrRunStateConflict for mismatched upstream run id, got %v", err)
	}
}

func TestRunServiceAdvancePendingRunRejectsAlreadyRunningRun(t *testing.T) {
	repoRoot := t.TempDir()
	writeWorkflowFixture(t, repoRoot)
	initGitRepoWithCommit(t, repoRoot)

	service := NewRunService(filepath.Join(repoRoot, "data", "ralleh-flow.db"), repoRoot, NewWorkflowService(repoRoot))

	run, err := service.Create(context.Background(), CreateRunInput{
		WorkflowID: "feature-development",
		Variables: map[string]string{
			"feature_name": "Flow polish",
			"target_repo":  "ralleh-flow",
		},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if _, err := service.AdvancePendingRun(context.Background(), run.ID); err != nil {
		t.Fatalf("initial advance pending run: %v", err)
	}
	if _, err := service.AdvancePendingRun(context.Background(), run.ID); !errors.Is(err, ErrRunStateConflict) {
		t.Fatalf("expected ErrRunStateConflict on second advance, got %v", err)
	}
}

func TestRunServiceCreateFailsWhenRepoLockHeld(t *testing.T) {
	repoRoot := t.TempDir()
	writeWorkflowFixture(t, repoRoot)
	initGitRepoWithCommit(t, repoRoot)

	coord := NewInMemoryCoordinator()
	lease, err := coord.AcquireRepoLock(context.Background(), repoRoot, "external-worker", time.Minute)
	if err != nil {
		t.Fatalf("acquire repo lock: %v", err)
	}
	defer lease.Release(context.Background())

	service := NewRunService(filepath.Join(repoRoot, "data", "ralleh-flow.db"), repoRoot, NewWorkflowService(repoRoot)).WithCoordinator(coord)

	_, err = service.Create(context.Background(), CreateRunInput{
		WorkflowID: "feature-development",
		Variables: map[string]string{
			"feature_name": "Flow polish",
			"target_repo":  "ralleh-flow",
		},
	})
	if !errors.Is(err, ErrRepositoryLocked) {
		t.Fatalf("expected ErrRepositoryLocked, got %v", err)
	}
}

func writeWorkflowFixture(t *testing.T, repoRoot string) {
	t.Helper()
	workflowPath := filepath.Join(repoRoot, "workflows", "examples", "feature-development", "workflow.yaml")
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	workflowYAML := `metadata:
  id: feature-development
variables:
  - key: feature_name
    type: string
    required: true
  - key: target_repo
    type: repository
    required: true
steps:
  - id: research
    kind: agent_task
  - id: implement
    kind: agent_task
    agent: ralleh-dev
  - id: review
    kind: human_approval
    approverPolicy: reviewer
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
}

func initGitRepoWithCommit(t *testing.T, repoRoot string) {
	t.Helper()

	runCommand(t, repoRoot, "git", "init", "-b", "main")
	runCommand(t, repoRoot, "git", "config", "user.email", "test@example.com")
	runCommand(t, repoRoot, "git", "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}
	runCommand(t, repoRoot, "git", "add", "README.md", "workflows")
	runCommand(t, repoRoot, "git", "commit", "-m", "init")
}

func runCommand(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run %s %v: %v\n%s", name, args, err, string(output))
	}
}
