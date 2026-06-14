package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
)

type scriptedDispatchOutcome struct {
	result DispatchResult
	err    error
}

type scriptedDispatcher struct {
	mode     string
	outcomes []scriptedDispatchOutcome
	requests []DispatchRequest
}

func (d *scriptedDispatcher) DispatchAgentTask(_ context.Context, req DispatchRequest) (DispatchResult, error) {
	d.requests = append(d.requests, req)
	if len(d.outcomes) == 0 {
		return DispatchResult{SessionID: fmt.Sprintf("sim:%s:%s", req.Run.ID, req.Step.StepID)}, nil
	}
	outcome := d.outcomes[0]
	d.outcomes = d.outcomes[1:]
	if outcome.err != nil {
		return DispatchResult{}, outcome.err
	}
	return outcome.result, nil
}

func (d *scriptedDispatcher) Mode() string {
	if d.mode == "" {
		return "scripted"
	}
	return d.mode
}

type simulationHarness struct {
	t       *testing.T
	service *RunService
	run     RunRecord
}

func newSimulationHarness(t *testing.T, outcomes []scriptedDispatchOutcome) *simulationHarness {
	t.Helper()
	repoRoot := t.TempDir()
	writeWorkflowFixture(t, repoRoot)
	initGitRepoWithCommit(t, repoRoot)

	dispatcher := &scriptedDispatcher{outcomes: outcomes}
	service := NewRunService(filepath.Join(repoRoot, "data", "ralleh-flow.db"), repoRoot, NewWorkflowService(repoRoot)).WithDispatcher(dispatcher)

	run, err := service.Create(context.Background(), CreateRunInput{
		WorkflowID: "feature-development",
		Variables: map[string]string{
			"feature_name": "simulation-harness",
			"target_repo":  "ralleh-flow",
		},
	})
	if err != nil {
		t.Fatalf("create run: %v", err)
	}

	return &simulationHarness{t: t, service: service, run: run}
}

func (h *simulationHarness) advance() {
	h.t.Helper()
	run, err := h.service.AdvancePendingRun(context.Background(), h.run.ID)
	if err != nil {
		h.t.Fatalf("advance run: %v", err)
	}
	h.run = run
}

func (h *simulationHarness) dispatch() error {
	h.t.Helper()
	run, err := h.service.DispatchActiveStep(context.Background(), h.run.ID, HandoffDispatchInput{})
	if err != nil {
		return err
	}
	h.run = run
	return nil
}

func (h *simulationHarness) complete(summary string) {
	h.t.Helper()
	run, err := h.service.CompleteActiveStep(context.Background(), h.run.ID, StepCompletionInput{Summary: summary})
	if err != nil {
		h.t.Fatalf("complete step: %v", err)
	}
	h.run = run
}

func (h *simulationHarness) pendingApprovalID() string {
	h.t.Helper()
	approvals, err := h.service.ListApprovals(context.Background())
	if err != nil {
		h.t.Fatalf("list approvals: %v", err)
	}
	for _, approval := range approvals {
		if approval.RunID == h.run.ID && approval.Status == "pending" {
			return approval.ID
		}
	}
	h.t.Fatalf("pending approval not found for run %s", h.run.ID)
	return ""
}

func hasTimelineEvent(run RunRecord, eventType string) bool {
	for _, event := range run.Timeline {
		if event.Type == eventType {
			return true
		}
	}
	return false
}

func TestWorkflowSimulationHarnessSequentialHappyPath(t *testing.T) {
	h := newSimulationHarness(t, nil)

	h.advance()
	if err := h.dispatch(); err != nil {
		t.Fatalf("dispatch research: %v", err)
	}
	h.complete("research complete")

	h.advance()
	if err := h.dispatch(); err != nil {
		t.Fatalf("dispatch implement: %v", err)
	}
	h.complete("implement complete")

	approvalID := h.pendingApprovalID()
	run, err := h.service.ApproveApproval(context.Background(), approvalID, ApprovalDecisionInput{DecidedBy: "qa"})
	if err != nil {
		t.Fatalf("approve step: %v", err)
	}
	h.run = run

	if h.run.Status != "completed" {
		t.Fatalf("expected completed run, got %q", h.run.Status)
	}
	for _, eventType := range []string{"step.handoff.dispatched", "approval.requested", "approval.granted", "run.completed"} {
		if !hasTimelineEvent(h.run, eventType) {
			t.Fatalf("expected timeline to contain %s, got %#v", eventType, h.run.Timeline)
		}
	}
}

func TestWorkflowSimulationHarnessApprovalChangesResume(t *testing.T) {
	h := newSimulationHarness(t, nil)

	h.advance()
	if err := h.dispatch(); err != nil {
		t.Fatalf("dispatch research: %v", err)
	}
	h.complete("research complete")
	h.advance()
	if err := h.dispatch(); err != nil {
		t.Fatalf("dispatch implement: %v", err)
	}
	h.complete("implement complete")

	approvalID := h.pendingApprovalID()
	run, err := h.service.RequestApprovalChanges(context.Background(), approvalID, ApprovalDecisionInput{DecidedBy: "qa", Rationale: "needs hardening"})
	if err != nil {
		t.Fatalf("request changes: %v", err)
	}
	if run.Status != runStatusChangesRequested {
		t.Fatalf("expected %s status, got %q", runStatusChangesRequested, run.Status)
	}

	run, err = h.service.ResumeRun(context.Background(), h.run.ID)
	if err != nil {
		t.Fatalf("resume run: %v", err)
	}
	if run.Status != runStatusWaitingForApproval {
		t.Fatalf("expected %s status after resume, got %q", runStatusWaitingForApproval, run.Status)
	}

	approvalID = h.pendingApprovalID()
	run, err = h.service.ApproveApproval(context.Background(), approvalID, ApprovalDecisionInput{DecidedBy: "qa"})
	if err != nil {
		t.Fatalf("approve resumed approval: %v", err)
	}
	h.run = run

	if !hasTimelineEvent(h.run, "approval.changes_requested") || !hasTimelineEvent(h.run, "approval.reopened") {
		t.Fatalf("expected changes_requested and reopened events, got %#v", h.run.Timeline)
	}
}

func TestWorkflowSimulationHarnessDispatchFailureMalformedAndStall(t *testing.T) {
	t.Run("dispatch failure keeps handoff claimed", func(t *testing.T) {
		h := newSimulationHarness(t, []scriptedDispatchOutcome{{err: errors.New("dispatch unavailable")}})
		h.advance()
		err := h.dispatch()
		if err == nil || err.Error() != "dispatch unavailable" {
			t.Fatalf("expected dispatch unavailable error, got %v", err)
		}

		run, getErr := h.service.Get(context.Background(), h.run.ID)
		if getErr != nil {
			t.Fatalf("get run: %v", getErr)
		}
		if run.Status != "running" {
			t.Fatalf("expected running after dispatch failure, got %q", run.Status)
		}
		handoff, found := handoffRecordByStepID(run, "research")
		if !found || handoff.Status != "claimed" {
			t.Fatalf("expected claimed handoff after dispatch failure, got %#v", run.Handoffs)
		}
	})

	t.Run("malformed callback is rejected", func(t *testing.T) {
		h := newSimulationHarness(t, nil)
		h.advance()
		if err := h.dispatch(); err != nil {
			t.Fatalf("dispatch research: %v", err)
		}
		_, err := h.service.CompleteActiveStep(context.Background(), h.run.ID, StepCompletionInput{Summary: "done", SessionID: "wrong-session"})
		if !errors.Is(err, ErrRunStateConflict) {
			t.Fatalf("expected ErrRunStateConflict on malformed callback session, got %v", err)
		}
	})

	t.Run("stall remains running and duplicate advance conflicts", func(t *testing.T) {
		h := newSimulationHarness(t, nil)
		h.advance()
		if err := h.dispatch(); err != nil {
			t.Fatalf("dispatch research: %v", err)
		}
		_, err := h.service.AdvancePendingRun(context.Background(), h.run.ID)
		if !errors.Is(err, ErrRunStateConflict) {
			t.Fatalf("expected ErrRunStateConflict on duplicate pending event while running, got %v", err)
		}
		run, getErr := h.service.Get(context.Background(), h.run.ID)
		if getErr != nil {
			t.Fatalf("get run: %v", getErr)
		}
		if run.Status != "running" {
			t.Fatalf("expected running during stall, got %q", run.Status)
		}
	})
}

func TestWorkflowSimulationHarnessDuplicateAdvanceConflictThenRecovers(t *testing.T) {
	h := newSimulationHarness(t, nil)

	h.advance()
	if _, err := h.service.AdvancePendingRun(context.Background(), h.run.ID); !errors.Is(err, ErrRunStateConflict) {
		t.Fatalf("expected ErrRunStateConflict on duplicate advance, got %v", err)
	}

	if err := h.dispatch(); err != nil {
		t.Fatalf("dispatch research: %v", err)
	}
	h.complete("research complete")

	h.advance()
	if err := h.dispatch(); err != nil {
		t.Fatalf("dispatch implement: %v", err)
	}
	h.complete("implement complete")

	approvalID := h.pendingApprovalID()
	run, err := h.service.ApproveApproval(context.Background(), approvalID, ApprovalDecisionInput{DecidedBy: "qa"})
	if err != nil {
		t.Fatalf("approve final gate: %v", err)
	}
	if run.Status != "completed" {
		t.Fatalf("expected completed status after recovery, got %q", run.Status)
	}
}

func TestWorkflowSimulationHarnessParallelFanOutJoinScaffold(t *testing.T) {
	t.Skip("workflow model is linear today; fan-out/join requires explicit DAG/parallel step metadata and join semantics in RunService")
}
