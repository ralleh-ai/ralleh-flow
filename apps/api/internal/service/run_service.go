package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrRunNotFound              = errors.New("run not found")
	ErrApprovalNotFound         = errors.New("approval not found")
	ErrWorkflowMissing          = errors.New("workflow not found")
	ErrInvalidRunInput          = errors.New("invalid run input")
	ErrMissingWorkflowVariables = errors.New("missing required workflow variables")
	ErrExecutionRepoNotReady    = errors.New("execution repository is not ready")
	ErrRunStateConflict         = errors.New("run state transition conflict")
)

var runIDCounter atomic.Uint64

func matchesOptionalHandoffValue(expected, actual string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return true
	}
	return expected == strings.TrimSpace(actual)
}

func validateHandoffCallback(step StepRecord, handoff HandoffRecord, sessionID, upstreamRunID string) error {
	if step.Kind == "agent_task" && handoff.Status != "dispatched" {
		return fmt.Errorf("%w: agent_task callback requires dispatched handoff proof", ErrRunStateConflict)
	}
	if !matchesOptionalHandoffValue(sessionID, handoff.SessionID) {
		return fmt.Errorf("%w: callback sessionId does not match dispatched handoff", ErrRunStateConflict)
	}
	if !matchesOptionalHandoffValue(upstreamRunID, handoff.UpstreamRunID) {
		return fmt.Errorf("%w: callback upstreamRunId does not match dispatched handoff", ErrRunStateConflict)
	}
	return nil
}

type StepCompletionInput struct {
	Summary         string
	CheckpointLabel string
	SessionID       string
	UpstreamRunID   string
}

type StepFailureInput struct {
	Reason        string
	SessionID     string
	UpstreamRunID string
}

type HandoffDispatchInput struct {
	SessionID string
	Note      string
}

type ApprovalDecisionInput struct {
	DecidedBy string
	Rationale string
}

const runStatusChangesRequested = "changes_requested"
const runStatusWaitingForApproval = "waiting_for_approval"

type RunService struct {
	store       *RunStore
	workflows   WorkflowService
	repoRoot    string
	coordinator Coordinator
	eventBus    EventBus
	dispatcher  Dispatcher
	leaseTTL    time.Duration
	workerID    string
}

func NewRunService(dbPath, repoRoot string, workflows WorkflowService) *RunService {
	root := strings.TrimSpace(repoRoot)
	if root == "" {
		root = discoverRepoRoot()
	}

	return &RunService{
		store:       NewRunStore(dbPath),
		workflows:   workflows,
		repoRoot:    root,
		coordinator: NewInMemoryCoordinator(),
		eventBus:    NewInMemoryEventBus(),
		dispatcher:  NewNoopDispatcher(),
		leaseTTL:    30 * time.Second,
		workerID:    fmt.Sprintf("run-service-%d", time.Now().UnixNano()),
	}
}

func (s *RunService) WithCoordinator(coordinator Coordinator) *RunService {
	if coordinator != nil {
		s.coordinator = coordinator
	}
	return s
}

func (s *RunService) WithEventBus(eventBus EventBus) *RunService {
	if eventBus != nil {
		s.eventBus = eventBus
	}
	return s
}

func (s *RunService) WorkerID() string {
	return s.workerID
}

func (s *RunService) WithDispatcher(dispatcher Dispatcher) *RunService {
	if dispatcher != nil {
		s.dispatcher = dispatcher
	}
	return s
}

func (s *RunService) List(ctx context.Context) ([]RunRecord, error) {
	return s.store.List(ctx)
}

func (s *RunService) ListApprovals(ctx context.Context) ([]ApprovalRecord, error) {
	return s.store.ListApprovals(ctx)
}

func (s *RunService) ApproveApproval(ctx context.Context, approvalID string, input ApprovalDecisionInput) (RunRecord, error) {
	approval, run, workflow, _, err := s.loadApprovalContext(ctx, approvalID)
	if err != nil {
		return RunRecord{}, err
	}

	runOwner := fmt.Sprintf("%s:%s:approve", s.workerID, run.ID)
	runLease, err := s.coordinator.AcquireRunLease(ctx, run.ID, runOwner, s.leaseTTL)
	if err != nil {
		return RunRecord{}, err
	}
	defer runLease.Release(ctx)

	decidedAt := time.Now().UTC().Format(time.RFC3339)
	decidedBy := strings.TrimSpace(input.DecidedBy)
	if decidedBy == "" {
		decidedBy = "operator"
	}
	rationale := strings.TrimSpace(input.Rationale)

	approvalGrantedEvent := TimelineEvent{At: decidedAt, Type: "approval.granted", Detail: fmt.Sprintf("Approval granted for step %s", approval.StepID)}
	timelineEvents := []TimelineEvent{approvalGrantedEvent}
	publishedEvents := []TimelineEvent{approvalGrantedEvent}
	transitionStatus := "completed"
	transitionStepID := ""

	if nextStep, ok := nextWorkflowStep(workflow, approval.StepID); ok {
		transitionStatus = "pending"
		transitionStepID = nextStep.ID
		runPendingEvent := TimelineEvent{At: decidedAt, Type: "run.pending", Detail: fmt.Sprintf("Run %s queued next step %s after approval", run.ID, nextStep.ID)}
		timelineEvents = append(timelineEvents, runPendingEvent)
		publishedEvents = append(publishedEvents, runPendingEvent)
	} else {
		runCompletedEvent := TimelineEvent{At: decidedAt, Type: "run.completed", Detail: fmt.Sprintf("Run %s completed", run.ID)}
		timelineEvents = append(timelineEvents, runCompletedEvent)
		publishedEvents = append(publishedEvents, runCompletedEvent)
	}

	if err := s.store.DecideApprovalAndTransitionRun(ctx, approval.ID, run.ID, "pending", "approved", decidedBy, rationale, decidedAt, "waiting_for_approval", transitionStatus, transitionStepID, timelineEvents); err != nil {
		return RunRecord{}, err
	}

	for _, event := range publishedEvents {
		if err := s.eventBus.PublishRunEvent(ctx, run.ID, event); err != nil {
			return RunRecord{}, err
		}
	}

	return s.Get(ctx, run.ID)
}

func (s *RunService) RejectApproval(ctx context.Context, approvalID string, input ApprovalDecisionInput) (RunRecord, error) {
	approval, run, _, _, err := s.loadApprovalContext(ctx, approvalID)
	if err != nil {
		return RunRecord{}, err
	}

	runOwner := fmt.Sprintf("%s:%s:reject", s.workerID, run.ID)
	runLease, err := s.coordinator.AcquireRunLease(ctx, run.ID, runOwner, s.leaseTTL)
	if err != nil {
		return RunRecord{}, err
	}
	defer runLease.Release(ctx)

	decidedAt := time.Now().UTC().Format(time.RFC3339)
	decidedBy := strings.TrimSpace(input.DecidedBy)
	if decidedBy == "" {
		decidedBy = "operator"
	}
	rationale := strings.TrimSpace(input.Rationale)

	approvalRejectedEvent := TimelineEvent{At: decidedAt, Type: "approval.rejected", Detail: fmt.Sprintf("Approval rejected for step %s", approval.StepID)}
	runFailedEvent := TimelineEvent{At: decidedAt, Type: "run.failed", Detail: fmt.Sprintf("Run %s failed during approval for step %s", run.ID, approval.StepID)}
	if err := s.store.DecideApprovalAndTransitionRun(ctx, approval.ID, run.ID, "pending", "rejected", decidedBy, rationale, decidedAt, "waiting_for_approval", "failed", approval.StepID, []TimelineEvent{approvalRejectedEvent, runFailedEvent}); err != nil {
		return RunRecord{}, err
	}

	for _, event := range []TimelineEvent{approvalRejectedEvent, runFailedEvent} {
		if err := s.eventBus.PublishRunEvent(ctx, run.ID, event); err != nil {
			return RunRecord{}, err
		}
	}

	return s.Get(ctx, run.ID)
}

func (s *RunService) RequestApprovalChanges(ctx context.Context, approvalID string, input ApprovalDecisionInput) (RunRecord, error) {
	approval, run, _, _, err := s.loadApprovalContext(ctx, approvalID)
	if err != nil {
		return RunRecord{}, err
	}

	runOwner := fmt.Sprintf("%s:%s:request-changes", s.workerID, run.ID)
	runLease, err := s.coordinator.AcquireRunLease(ctx, run.ID, runOwner, s.leaseTTL)
	if err != nil {
		return RunRecord{}, err
	}
	defer runLease.Release(ctx)

	decidedAt := time.Now().UTC().Format(time.RFC3339)
	decidedBy := strings.TrimSpace(input.DecidedBy)
	if decidedBy == "" {
		decidedBy = "operator"
	}
	rationale := strings.TrimSpace(input.Rationale)
	if rationale == "" {
		rationale = "Changes requested"
	}

	changesRequestedEvent := TimelineEvent{At: decidedAt, Type: "approval.changes_requested", Detail: fmt.Sprintf("Changes requested for step %s", approval.StepID)}
	runPausedEvent := TimelineEvent{At: decidedAt, Type: "run.changes_requested", Detail: fmt.Sprintf("Run %s paused for requested changes on step %s", run.ID, approval.StepID)}
	if err := s.store.DecideApprovalAndTransitionRun(ctx, approval.ID, run.ID, "pending", runStatusChangesRequested, decidedBy, rationale, decidedAt, runStatusWaitingForApproval, runStatusChangesRequested, approval.StepID, []TimelineEvent{changesRequestedEvent, runPausedEvent}); err != nil {
		return RunRecord{}, err
	}

	for _, event := range []TimelineEvent{changesRequestedEvent, runPausedEvent} {
		if err := s.eventBus.PublishRunEvent(ctx, run.ID, event); err != nil {
			return RunRecord{}, err
		}
	}

	return s.Get(ctx, run.ID)
}

func (s *RunService) ResumeRun(ctx context.Context, runID string) (RunRecord, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return RunRecord{}, ErrInvalidRunInput
	}

	run, found, err := s.store.Get(ctx, runID)
	if err != nil {
		return RunRecord{}, err
	}
	if !found {
		return RunRecord{}, ErrRunNotFound
	}
	if run.Status != runStatusChangesRequested {
		return RunRecord{}, fmt.Errorf("%w: run %s is not awaiting requested changes", ErrRunStateConflict, run.ID)
	}
	if strings.TrimSpace(run.CurrentStep) == "" {
		return RunRecord{}, fmt.Errorf("%w: run %s has no approval step to resume", ErrRunStateConflict, run.ID)
	}

	workflow, found, err := s.workflows.Get(run.WorkflowID)
	if err != nil {
		return RunRecord{}, err
	}
	if !found {
		return RunRecord{}, ErrWorkflowMissing
	}
	step, found := workflowStepByID(workflow, run.CurrentStep)
	if !found {
		return RunRecord{}, fmt.Errorf("%w: workflow step %s missing from workflow %s", ErrRunStateConflict, run.CurrentStep, run.WorkflowID)
	}
	if step.Kind != "human_approval" {
		return RunRecord{}, fmt.Errorf("%w: run %s current step %s is not a human approval gate", ErrRunStateConflict, run.ID, step.ID)
	}

	approval, found, err := s.store.GetApprovalByRunAndStep(ctx, run.ID, step.ID)
	if err != nil {
		return RunRecord{}, err
	}
	if !found {
		return RunRecord{}, ErrApprovalNotFound
	}
	if approval.Status != runStatusChangesRequested {
		return RunRecord{}, fmt.Errorf("%w: approval %s is not awaiting requested changes", ErrRunStateConflict, approval.ID)
	}

	runOwner := fmt.Sprintf("%s:%s:resume", s.workerID, runID)
	runLease, err := s.coordinator.AcquireRunLease(ctx, runID, runOwner, s.leaseTTL)
	if err != nil {
		return RunRecord{}, err
	}
	defer runLease.Release(ctx)

	resumedAt := time.Now().UTC().Format(time.RFC3339)
	approvalReopenedEvent := TimelineEvent{At: resumedAt, Type: "approval.reopened", Detail: fmt.Sprintf("Approval reopened for step %s", approval.StepID)}
	runResumedEvent := TimelineEvent{At: resumedAt, Type: "run.waiting_for_approval", Detail: fmt.Sprintf("Run %s resumed to approval gate %s", run.ID, approval.StepID)}
	if err := s.store.ReopenApprovalAndResumeRun(ctx, approval.ID, run.ID, runStatusChangesRequested, "pending", resumedAt, runStatusChangesRequested, runStatusWaitingForApproval, approval.StepID, []TimelineEvent{approvalReopenedEvent, runResumedEvent}); err != nil {
		return RunRecord{}, err
	}

	for _, event := range []TimelineEvent{approvalReopenedEvent, runResumedEvent} {
		if err := s.eventBus.PublishRunEvent(ctx, run.ID, event); err != nil {
			return RunRecord{}, err
		}
	}

	return s.Get(ctx, run.ID)
}

func (s *RunService) Get(ctx context.Context, runID string) (RunRecord, error) {
	run, found, err := s.store.Get(ctx, runID)
	if err != nil {
		return RunRecord{}, err
	}
	if !found {
		return RunRecord{}, ErrRunNotFound
	}
	return run, nil
}

func (s *RunService) AdvancePendingRun(ctx context.Context, runID string) (RunRecord, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return RunRecord{}, ErrInvalidRunInput
	}

	run, found, err := s.store.Get(ctx, runID)
	if err != nil {
		return RunRecord{}, err
	}
	if !found {
		return RunRecord{}, ErrRunNotFound
	}

	workflow, found, err := s.workflows.Get(run.WorkflowID)
	if err != nil {
		return RunRecord{}, err
	}
	if !found {
		return RunRecord{}, ErrWorkflowMissing
	}
	step, found := workflowStepByID(workflow, run.CurrentStep)
	if !found {
		return RunRecord{}, fmt.Errorf("%w: step %s missing from workflow %s", ErrRunStateConflict, run.CurrentStep, run.WorkflowID)
	}

	runOwner := fmt.Sprintf("%s:%s:advance", s.workerID, runID)
	runLease, err := s.coordinator.AcquireRunLease(ctx, runID, runOwner, s.leaseTTL)
	if err != nil {
		return RunRecord{}, err
	}
	defer runLease.Release(ctx)

	startedAt := time.Now().UTC().Format(time.RFC3339)
	runStartedEvent := TimelineEvent{
		At:     startedAt,
		Type:   "run.started",
		Detail: fmt.Sprintf("Run orchestration claimed by %s", s.workerID),
	}
	if err := s.store.CompareAndSwapRunState(ctx, runID, "pending", "running", run.CurrentStep, runStartedEvent); err != nil {
		return RunRecord{}, err
	}

	stepRecord := StepRecord{
		RunID:     runID,
		StepID:    step.ID,
		Status:    "running",
		WorkerID:  s.workerID,
		StartedAt: startedAt,
		Kind:      step.Kind,
		Agent:     step.Agent,
	}
	handoffRecord := HandoffRecord{
		RunID:         runID,
		StepID:        step.ID,
		Status:        "claimed",
		Kind:          step.Kind,
		Agent:         step.Agent,
		WorkerID:      s.workerID,
		UpstreamRunID: "",
		FinishedAt:    "",
		CreatedAt:     startedAt,
		UpdatedAt:     startedAt,
	}
	stepStartedEvent := TimelineEvent{
		At:     startedAt,
		Type:   "step.started",
		Detail: fmt.Sprintf("Step %s claimed by %s", step.ID, s.workerID),
	}
	handoffEvent := TimelineEvent{
		At:     startedAt,
		Type:   "step.handoff.claimed",
		Detail: fmt.Sprintf("Dispatch handoff claimed for step %s", step.ID),
	}
	if err := s.writeStepCheckpoint(run.ID, step.ID+"-claimed", map[string]any{
		"runId":      run.ID,
		"workflowId": workflow.ID,
		"stepId":     step.ID,
		"status":     "claimed",
		"kind":       step.Kind,
		"agent":      step.Agent,
		"workerId":   s.workerID,
		"createdAt":  startedAt,
	}); err != nil {
		return RunRecord{}, err
	}
	if err := s.store.StartStep(ctx, runID, stepRecord, handoffRecord, []TimelineEvent{stepStartedEvent, handoffEvent}); err != nil {
		return RunRecord{}, err
	}

	for _, event := range []TimelineEvent{runStartedEvent, stepStartedEvent, handoffEvent} {
		if err := s.eventBus.PublishRunEvent(ctx, runID, event); err != nil {
			return RunRecord{}, err
		}
	}

	return s.Get(ctx, runID)
}

func (s *RunService) DispatchActiveStep(ctx context.Context, runID string, input HandoffDispatchInput) (RunRecord, error) {
	run, step, workflow, err := s.loadActiveStepContext(ctx, runID)
	if err != nil {
		return RunRecord{}, err
	}

	runOwner := fmt.Sprintf("%s:%s:dispatch", s.workerID, runID)
	runLease, err := s.coordinator.AcquireRunLease(ctx, runID, runOwner, s.leaseTTL)
	if err != nil {
		return RunRecord{}, err
	}
	defer runLease.Release(ctx)

	dispatchedAt := time.Now().UTC().Format(time.RFC3339)
	handoff, found := handoffRecordByStepID(run, step.StepID)
	if !found {
		return RunRecord{}, fmt.Errorf("%w: no handoff recorded for step %s", ErrRunStateConflict, step.StepID)
	}
	if s.dispatcher == nil {
		s.dispatcher = NewNoopDispatcher()
	}
	handoffDocPath, err := s.writeHandoffDocument(run, workflow, step, handoff, strings.TrimSpace(input.Note))
	if err != nil {
		return RunRecord{}, err
	}
	requestedSessionID := strings.TrimSpace(input.SessionID)
	if requestedSessionID != "" {
		handoff.SessionID = requestedSessionID
	}
	dispatchResult, err := s.dispatcher.DispatchAgentTask(ctx, DispatchRequest{
		Run:                 run,
		Step:                step,
		Workflow:            workflow,
		Handoff:             handoff,
		Note:                input.Note,
		RepoRoot:            s.repoRoot,
		Worktree:            run.WorktreePath,
		HandoffDocumentPath: handoffDocPath,
	})
	if err != nil {
		return RunRecord{}, err
	}
	sessionID := strings.TrimSpace(dispatchResult.SessionID)
	if sessionID == "" {
		sessionID = requestedSessionID
	}
	if sessionID == "" {
		sessionID = fmt.Sprintf("pending:%s:%s", run.ID, step.StepID)
	}
	note := strings.TrimSpace(dispatchResult.Note)
	if note == "" {
		note = strings.TrimSpace(input.Note)
	}
	upstreamRunID := strings.TrimSpace(dispatchResult.UpstreamRunID)
	if err := s.writeStepCheckpoint(run.ID, step.StepID+"-dispatched", map[string]any{
		"runId":               run.ID,
		"workflowId":          workflow.ID,
		"stepId":              step.StepID,
		"status":              "dispatched",
		"kind":                step.Kind,
		"agent":               step.Agent,
		"workerId":            step.WorkerID,
		"sessionId":           sessionID,
		"upstreamRunId":       upstreamRunID,
		"dispatchAttemptAt":   dispatchedAt,
		"handoffDocumentPath": handoffDocPath,
		"note":                note,
		"dispatcherMode":      s.dispatcher.Mode(),
	}); err != nil {
		return RunRecord{}, err
	}
	dispatchEvent := TimelineEvent{At: dispatchedAt, Type: "step.handoff.dispatched", Detail: fmt.Sprintf("Dispatch handoff bound for step %s to %s", step.StepID, sessionID)}
	if err := s.store.MarkHandoffDispatched(ctx, run.ID, step.StepID, sessionID, upstreamRunID, dispatchedAt, "claimed", []TimelineEvent{dispatchEvent}); err != nil {
		return RunRecord{}, err
	}
	if err := s.eventBus.PublishRunEvent(ctx, run.ID, dispatchEvent); err != nil {
		return RunRecord{}, err
	}
	return s.Get(ctx, run.ID)
}

func (s *RunService) CompleteActiveStep(ctx context.Context, runID string, input StepCompletionInput) (RunRecord, error) {
	run, step, workflow, err := s.loadActiveStepContext(ctx, runID)
	if err != nil {
		return RunRecord{}, err
	}

	runOwner := fmt.Sprintf("%s:%s:complete", s.workerID, runID)
	runLease, err := s.coordinator.AcquireRunLease(ctx, runID, runOwner, s.leaseTTL)
	if err != nil {
		return RunRecord{}, err
	}
	defer runLease.Release(ctx)

	finishedAt := time.Now().UTC().Format(time.RFC3339)
	checkpointLabel := sanitizeCheckpointLabel(input.CheckpointLabel)
	if checkpointLabel == "" {
		checkpointLabel = step.StepID
	}
	handoff, _ := handoffRecordByStepID(run, step.StepID)
	if err := validateHandoffCallback(step, handoff, input.SessionID, input.UpstreamRunID); err != nil {
		return RunRecord{}, err
	}
	if err := s.writeStepCheckpoint(run.ID, checkpointLabel, map[string]any{
		"runId":         run.ID,
		"workflowId":    workflow.ID,
		"stepId":        step.StepID,
		"status":        "completed",
		"workerId":      step.WorkerID,
		"sessionId":     handoff.SessionID,
		"upstreamRunId": handoff.UpstreamRunID,
		"summary":       strings.TrimSpace(input.Summary),
		"finishedAt":    finishedAt,
	}); err != nil {
		return RunRecord{}, err
	}

	handoffEvent := TimelineEvent{At: finishedAt, Type: "step.handoff.completed", Detail: fmt.Sprintf("Dispatch handoff completed for step %s", step.StepID)}
	stepCompletedEvent := TimelineEvent{At: finishedAt, Type: "step.completed", Detail: fmt.Sprintf("Step %s completed", step.StepID)}
	transitionStatus := "completed"
	transitionStepID := ""
	publishedEvents := []TimelineEvent{stepCompletedEvent}
	timelineEvents := []TimelineEvent{handoffEvent, stepCompletedEvent}
	var approvalRecord *ApprovalRecord

	if nextStep, ok := nextWorkflowStep(workflow, step.StepID); ok {
		transitionStepID = nextStep.ID
		if nextStep.Kind == "human_approval" {
			transitionStatus = "waiting_for_approval"
			approvalManifestLabel := sanitizeCheckpointLabel(nextStep.ID + "-approval-request")
			approvalManifestPath := s.stepCheckpointPath(run.ID, approvalManifestLabel)
			approvalRecord = &ApprovalRecord{
				ID:               fmt.Sprintf("approval_%s_%s", run.ID, sanitizeBranchFragment(nextStep.ID)),
				RunID:            run.ID,
				StepID:           nextStep.ID,
				Kind:             nextStep.Kind,
				Status:           "pending",
				ApproverPolicy:   nextStep.ApproverPolicy,
				RequestedBy:      step.WorkerID,
				EvidenceManifest: approvalManifestPath,
				CreatedAt:        finishedAt,
			}
			if err := s.writeStepCheckpoint(run.ID, approvalManifestLabel, map[string]any{
				"approvalId":       approvalRecord.ID,
				"runId":            run.ID,
				"workflowId":       workflow.ID,
				"stepId":           nextStep.ID,
				"status":           approvalRecord.Status,
				"kind":             approvalRecord.Kind,
				"approverPolicy":   approvalRecord.ApproverPolicy,
				"requestedBy":      approvalRecord.RequestedBy,
				"requestedAt":      approvalRecord.CreatedAt,
				"sourceStepId":     step.StepID,
				"sourceCheckpoint": s.stepCheckpointPath(run.ID, checkpointLabel),
				"handoffDocument":  filepath.Join(run.WorktreePath, "HANDOFF.md"),
				"evidenceManifest": approvalManifestPath,
				"worktreePath":     run.WorktreePath,
				"summary":          strings.TrimSpace(input.Summary),
			}); err != nil {
				return RunRecord{}, err
			}
			approvalRequestedEvent := TimelineEvent{At: finishedAt, Type: "approval.requested", Detail: fmt.Sprintf("Approval requested for step %s", nextStep.ID)}
			timelineEvents = append(timelineEvents, approvalRequestedEvent)
			publishedEvents = append(publishedEvents, approvalRequestedEvent)
		} else {
			transitionStatus = "pending"
			runPendingEvent := TimelineEvent{At: finishedAt, Type: "run.pending", Detail: fmt.Sprintf("Run %s queued next step %s", run.ID, nextStep.ID)}
			timelineEvents = append(timelineEvents, runPendingEvent)
			publishedEvents = append(publishedEvents, runPendingEvent)
		}
	} else {
		runCompletedEvent := TimelineEvent{At: finishedAt, Type: "run.completed", Detail: fmt.Sprintf("Run %s completed", run.ID)}
		timelineEvents = append(timelineEvents, runCompletedEvent)
		publishedEvents = append(publishedEvents, runCompletedEvent)
	}

	if approvalRecord != nil {
		if err := s.store.CompleteStepAndTransitionRunWithApproval(ctx, run.ID, step.StepID, "completed", "completed", finishedAt, "running", transitionStatus, transitionStepID, timelineEvents, *approvalRecord); err != nil {
			return RunRecord{}, err
		}
	} else if err := s.store.CompleteStepAndTransitionRun(ctx, run.ID, step.StepID, "completed", "completed", finishedAt, "running", transitionStatus, transitionStepID, timelineEvents); err != nil {
		return RunRecord{}, err
	}
	for _, event := range publishedEvents {
		if err := s.eventBus.PublishRunEvent(ctx, run.ID, event); err != nil {
			return RunRecord{}, err
		}
	}
	return s.Get(ctx, run.ID)
}

func (s *RunService) FailActiveStep(ctx context.Context, runID string, input StepFailureInput) (RunRecord, error) {
	run, step, _, err := s.loadActiveStepContext(ctx, runID)
	if err != nil {
		return RunRecord{}, err
	}

	runOwner := fmt.Sprintf("%s:%s:fail", s.workerID, runID)
	runLease, err := s.coordinator.AcquireRunLease(ctx, runID, runOwner, s.leaseTTL)
	if err != nil {
		return RunRecord{}, err
	}
	defer runLease.Release(ctx)

	finishedAt := time.Now().UTC().Format(time.RFC3339)
	handoff, _ := handoffRecordByStepID(run, step.StepID)
	if err := validateHandoffCallback(step, handoff, input.SessionID, input.UpstreamRunID); err != nil {
		return RunRecord{}, err
	}
	if err := s.writeStepCheckpoint(run.ID, step.StepID+"-failed", map[string]any{
		"runId":         run.ID,
		"stepId":        step.StepID,
		"status":        "failed",
		"workerId":      step.WorkerID,
		"sessionId":     handoff.SessionID,
		"upstreamRunId": handoff.UpstreamRunID,
		"reason":        strings.TrimSpace(input.Reason),
		"finishedAt":    finishedAt,
	}); err != nil {
		return RunRecord{}, err
	}
	handoffEvent := TimelineEvent{At: finishedAt, Type: "step.handoff.failed", Detail: fmt.Sprintf("Dispatch handoff failed for step %s", step.StepID)}
	stepFailedEvent := TimelineEvent{At: finishedAt, Type: "step.failed", Detail: fmt.Sprintf("Step %s failed: %s", step.StepID, strings.TrimSpace(input.Reason))}
	runFailedEvent := TimelineEvent{At: finishedAt, Type: "run.failed", Detail: fmt.Sprintf("Run %s failed: %s", run.ID, strings.TrimSpace(input.Reason))}
	if err := s.store.CompleteStepAndTransitionRun(ctx, run.ID, step.StepID, "failed", "failed", finishedAt, "running", "failed", step.StepID, []TimelineEvent{handoffEvent, stepFailedEvent, runFailedEvent}); err != nil {
		return RunRecord{}, err
	}
	for _, event := range []TimelineEvent{stepFailedEvent, runFailedEvent} {
		if err := s.eventBus.PublishRunEvent(ctx, run.ID, event); err != nil {
			return RunRecord{}, err
		}
	}
	return s.Get(ctx, run.ID)
}

func (s *RunService) loadActiveStepContext(ctx context.Context, runID string) (RunRecord, StepRecord, WorkflowDetail, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return RunRecord{}, StepRecord{}, WorkflowDetail{}, ErrInvalidRunInput
	}
	run, found, err := s.store.Get(ctx, runID)
	if err != nil {
		return RunRecord{}, StepRecord{}, WorkflowDetail{}, err
	}
	if !found {
		return RunRecord{}, StepRecord{}, WorkflowDetail{}, ErrRunNotFound
	}
	workflow, found, err := s.workflows.Get(run.WorkflowID)
	if err != nil {
		return RunRecord{}, StepRecord{}, WorkflowDetail{}, err
	}
	if !found {
		return RunRecord{}, StepRecord{}, WorkflowDetail{}, ErrWorkflowMissing
	}
	step, found := activeStepRecord(run)
	if !found {
		return RunRecord{}, StepRecord{}, WorkflowDetail{}, fmt.Errorf("%w: no active step recorded for run %s", ErrRunStateConflict, runID)
	}
	return run, step, workflow, nil
}

func (s *RunService) loadApprovalContext(ctx context.Context, approvalID string) (ApprovalRecord, RunRecord, WorkflowDetail, WorkflowStep, error) {
	approvalID = strings.TrimSpace(approvalID)
	if approvalID == "" {
		return ApprovalRecord{}, RunRecord{}, WorkflowDetail{}, WorkflowStep{}, ErrInvalidRunInput
	}

	approval, found, err := s.store.GetApproval(ctx, approvalID)
	if err != nil {
		return ApprovalRecord{}, RunRecord{}, WorkflowDetail{}, WorkflowStep{}, err
	}
	if !found {
		return ApprovalRecord{}, RunRecord{}, WorkflowDetail{}, WorkflowStep{}, ErrApprovalNotFound
	}

	run, found, err := s.store.Get(ctx, approval.RunID)
	if err != nil {
		return ApprovalRecord{}, RunRecord{}, WorkflowDetail{}, WorkflowStep{}, err
	}
	if !found {
		return ApprovalRecord{}, RunRecord{}, WorkflowDetail{}, WorkflowStep{}, ErrRunNotFound
	}

	workflow, found, err := s.workflows.Get(run.WorkflowID)
	if err != nil {
		return ApprovalRecord{}, RunRecord{}, WorkflowDetail{}, WorkflowStep{}, err
	}
	if !found {
		return ApprovalRecord{}, RunRecord{}, WorkflowDetail{}, WorkflowStep{}, ErrWorkflowMissing
	}

	step, found := workflowStepByID(workflow, approval.StepID)
	if !found {
		return ApprovalRecord{}, RunRecord{}, WorkflowDetail{}, WorkflowStep{}, fmt.Errorf("%w: workflow step %s missing for approval %s", ErrRunStateConflict, approval.StepID, approval.ID)
	}
	if step.Kind != "human_approval" {
		return ApprovalRecord{}, RunRecord{}, WorkflowDetail{}, WorkflowStep{}, fmt.Errorf("%w: approval %s does not target a human_approval step", ErrRunStateConflict, approval.ID)
	}
	if run.Status != "waiting_for_approval" {
		return ApprovalRecord{}, RunRecord{}, WorkflowDetail{}, WorkflowStep{}, fmt.Errorf("%w: run %s is not waiting for approval", ErrRunStateConflict, run.ID)
	}
	if strings.TrimSpace(run.CurrentStep) != strings.TrimSpace(approval.StepID) {
		return ApprovalRecord{}, RunRecord{}, WorkflowDetail{}, WorkflowStep{}, fmt.Errorf("%w: run %s current step does not match approval %s", ErrRunStateConflict, run.ID, approval.ID)
	}
	if approval.Status != "pending" {
		return ApprovalRecord{}, RunRecord{}, WorkflowDetail{}, WorkflowStep{}, fmt.Errorf("%w: approval %s is not pending", ErrRunStateConflict, approval.ID)
	}

	return approval, run, workflow, step, nil
}

func (s *RunService) writeStepCheckpoint(runID, label string, payload map[string]any) error {
	checkpointDir := filepath.Join(s.repoRoot, "runtime", "runs", runID, "handoffs")
	if err := os.MkdirAll(checkpointDir, 0o755); err != nil {
		return fmt.Errorf("create checkpoint dir: %w", err)
	}
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}
	body = append(body, '\n')
	checkpointPath := s.stepCheckpointPath(runID, label)
	if err := os.WriteFile(checkpointPath, body, 0o644); err != nil {
		return fmt.Errorf("write checkpoint: %w", err)
	}
	return nil
}

func (s *RunService) stepCheckpointPath(runID, label string) string {
	return filepath.Join(s.repoRoot, "runtime", "runs", runID, "handoffs", label+".json")
}

func (s *RunService) writeHandoffDocument(run RunRecord, workflow WorkflowDetail, step StepRecord, handoff HandoffRecord, note string) (string, error) {
	docPath := filepath.Join(run.WorktreePath, "HANDOFF.md")
	content := buildHandoffDocument(run, workflow, step, handoff, note, docPath)
	if err := os.WriteFile(docPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write handoff document: %w", err)
	}
	return docPath, nil
}

func buildHandoffDocument(run RunRecord, workflow WorkflowDetail, step StepRecord, handoff HandoffRecord, note, docPath string) string {
	sections := []string{
		"# HANDOFF",
		"",
		"## Run",
		fmt.Sprintf("- Run ID: %s", run.ID),
		fmt.Sprintf("- Workflow: %s", workflow.ID),
		fmt.Sprintf("- Step: %s", step.StepID),
		fmt.Sprintf("- Kind: %s", step.Kind),
		fmt.Sprintf("- Agent: %s", firstNonEmptyHandoff(step.Agent, handoff.Agent, "unassigned")),
		fmt.Sprintf("- Branch: %s", run.Branch),
		fmt.Sprintf("- Worktree: %s", run.WorktreePath),
	}
	if workflow.Description != "" {
		sections = append(sections, fmt.Sprintf("- Workflow description: %s", workflow.Description))
	}
	sections = append(sections, "", "## Expectations", "- Work only inside this run worktree.", "- Treat the workflow runtime as source of truth for state transitions.", "- Record outputs needed for completion/failure callbacks.")
	if strings.TrimSpace(note) != "" {
		sections = append(sections, "", "## Operator note", note)
	}
	sections = append(sections, "", "## Files", fmt.Sprintf("- This document: %s", docPath))
	return strings.Join(sections, "\n") + "\n"
}

func firstNonEmptyHandoff(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (s *RunService) Create(ctx context.Context, input CreateRunInput) (RunRecord, error) {
	workflowID := strings.TrimSpace(input.WorkflowID)
	if workflowID == "" {
		return RunRecord{}, ErrInvalidRunInput
	}

	workflow, found, err := s.workflows.Get(workflowID)
	if err != nil {
		return RunRecord{}, err
	}
	if !found {
		return RunRecord{}, ErrWorkflowMissing
	}

	variables := normalizeInputVariables(input.Variables)
	missingKeys := missingRequiredVariableKeys(workflow, variables)
	if len(missingKeys) > 0 {
		return RunRecord{}, fmt.Errorf("%w: %s", ErrMissingWorkflowVariables, strings.Join(missingKeys, ", "))
	}

	now := time.Now().UTC()
	runID := generateRunID(now)
	runOwner := fmt.Sprintf("%s:%s", s.workerID, runID)
	runLease, err := s.coordinator.AcquireRunLease(ctx, runID, runOwner, s.leaseTTL)
	if err != nil {
		return RunRecord{}, err
	}
	defer runLease.Release(ctx)

	runRoot := filepath.Join(s.repoRoot, "runtime", "runs", runID)
	workspacePath := filepath.Join(runRoot, "workspace")
	branchName := fmt.Sprintf("workflow/%s/%s", sanitizeBranchFragment(workflowID), runID)

	run := RunRecord{
		ID:           runID,
		WorkflowID:   workflow.ID,
		Status:       "preparing",
		CurrentStep:  "",
		Branch:       branchName,
		WorktreePath: workspacePath,
		CreatedAt:    now.Format(time.RFC3339),
		Timeline:     []TimelineEvent{},
	}

	if err := prepareRuntimeDirs(runRoot); err != nil {
		return RunRecord{}, err
	}

	repoLock, err := s.coordinator.AcquireRepoLock(ctx, s.repoRoot, runOwner, s.leaseTTL)
	if err != nil {
		_ = os.RemoveAll(runRoot)
		return RunRecord{}, err
	}

	if err := prepareGitWorktree(ctx, s.repoRoot, branchName, workspacePath); err != nil {
		_ = repoLock.Release(ctx)
		_ = os.RemoveAll(runRoot)
		return RunRecord{}, err
	}
	_ = repoLock.Release(ctx)

	if err := s.store.Create(ctx, run); err != nil {
		cleanupPreparedRun(ctx, s.repoRoot, branchName, workspacePath, runRoot)
		return RunRecord{}, err
	}

	createdEvent := TimelineEvent{At: run.CreatedAt, Type: "run.created", Detail: "Run metadata persisted"}
	preparedEvent := TimelineEvent{At: time.Now().UTC().Format(time.RFC3339), Type: "run.prepared", Detail: "Git worktree and runtime directories prepared"}
	pendingEvent := TimelineEvent{At: time.Now().UTC().Format(time.RFC3339), Type: "run.pending", Detail: "Run is queued for orchestration"}
	if err := s.store.AppendTimeline(ctx, runID, []TimelineEvent{createdEvent, preparedEvent}); err != nil {
		_ = s.store.Delete(ctx, runID)
		cleanupPreparedRun(ctx, s.repoRoot, branchName, workspacePath, runRoot)
		return RunRecord{}, err
	}
	if err := s.store.TransitionRunState(ctx, runID, "pending", firstWorkflowStepID(workflow), pendingEvent); err != nil {
		_ = s.store.Delete(ctx, runID)
		cleanupPreparedRun(ctx, s.repoRoot, branchName, workspacePath, runRoot)
		return RunRecord{}, err
	}

	run.Status = "pending"
	run.CurrentStep = firstWorkflowStepID(workflow)
	run.Timeline = []TimelineEvent{createdEvent, preparedEvent, pendingEvent}

	for _, event := range run.Timeline {
		if err := s.eventBus.PublishRunEvent(ctx, runID, event); err != nil {
			_ = s.store.Delete(ctx, runID)
			cleanupPreparedRun(ctx, s.repoRoot, branchName, workspacePath, runRoot)
			return RunRecord{}, err
		}
	}

	return run, nil
}

func prepareRuntimeDirs(runRoot string) error {
	for _, dir := range []string{"logs", "snapshots", "handoffs", "artifacts"} {
		if err := os.MkdirAll(filepath.Join(runRoot, dir), 0o755); err != nil {
			return fmt.Errorf("create %s directory: %w", dir, err)
		}
	}
	return nil
}

func prepareGitWorktree(ctx context.Context, repoRoot, branchName, workspacePath string) error {
	if err := ensureGitRepoReady(ctx, repoRoot); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "worktree", "add", "-b", branchName, workspacePath, "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", ErrExecutionRepoNotReady, strings.TrimSpace(string(output)))
	}

	return nil
}

func ensureGitRepoReady(ctx context.Context, repoRoot string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-parse", "--verify", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			trimmed = "repository must be a git repo with at least one commit"
		}
		return fmt.Errorf("%w: %s", ErrExecutionRepoNotReady, trimmed)
	}

	return nil
}

func cleanupPreparedRun(ctx context.Context, repoRoot, branchName, workspacePath, runRoot string) {
	_ = exec.CommandContext(ctx, "git", "-C", repoRoot, "worktree", "remove", "--force", workspacePath).Run()
	_ = exec.CommandContext(ctx, "git", "-C", repoRoot, "branch", "-D", branchName).Run()
	_ = os.RemoveAll(runRoot)
}

func sanitizeBranchFragment(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "workflow"
	}
	builder := strings.Builder{}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}

	clean := strings.Trim(builder.String(), "-")
	if clean == "" {
		return "workflow"
	}
	return clean
}

func generateRunID(now time.Time) string {
	serial := runIDCounter.Add(1)
	return fmt.Sprintf("run_%s_%09d_%06d", now.Format("20060102T150405"), now.Nanosecond(), serial%1_000_000)
}

func handoffRecordByStepID(run RunRecord, stepID string) (HandoffRecord, bool) {
	for _, handoff := range run.Handoffs {
		if handoff.StepID == stepID {
			return handoff, true
		}
	}
	return HandoffRecord{}, false
}

func activeStepRecord(run RunRecord) (StepRecord, bool) {
	for _, step := range run.Steps {
		if step.Status == "running" {
			return step, true
		}
	}
	return StepRecord{}, false
}

func sanitizeCheckpointLabel(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ":", "-", "_", "-")
	value = replacer.Replace(value)
	filtered := make([]rune, 0, len(value))
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			filtered = append(filtered, r)
		}
	}
	return strings.Trim(string(filtered), "-")
}

func workflowStepByID(workflow WorkflowDetail, stepID string) (WorkflowStep, bool) {
	needle := strings.TrimSpace(stepID)
	if needle == "" {
		return WorkflowStep{}, false
	}
	for _, step := range workflow.Steps {
		if strings.TrimSpace(step.ID) == needle {
			return step, true
		}
	}
	return WorkflowStep{}, false
}

func firstWorkflowStepID(workflow WorkflowDetail) string {
	if len(workflow.Steps) == 0 {
		return ""
	}
	return strings.TrimSpace(workflow.Steps[0].ID)
}

func nextWorkflowStep(workflow WorkflowDetail, stepID string) (WorkflowStep, bool) {
	needle := strings.TrimSpace(stepID)
	if needle == "" {
		return WorkflowStep{}, false
	}
	for idx, step := range workflow.Steps {
		if strings.TrimSpace(step.ID) != needle {
			continue
		}
		nextIdx := idx + 1
		if nextIdx >= len(workflow.Steps) {
			return WorkflowStep{}, false
		}
		return workflow.Steps[nextIdx], true
	}
	return WorkflowStep{}, false
}

func normalizeInputVariables(variables map[string]string) map[string]string {
	if len(variables) == 0 {
		return map[string]string{}
	}

	normalized := make(map[string]string, len(variables))
	for key, value := range variables {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		normalized[trimmedKey] = strings.TrimSpace(value)
	}

	return normalized
}

func missingRequiredVariableKeys(workflow WorkflowDetail, variables map[string]string) []string {
	missing := make([]string, 0)
	for _, variable := range workflow.Variables {
		if !variable.Required {
			continue
		}

		if strings.TrimSpace(variables[variable.Key]) == "" {
			missing = append(missing, variable.Key)
		}
	}

	slices.Sort(missing)
	return missing
}

type RunStore struct {
	dbPath string

	mu       sync.Mutex
	migrated bool
}

func NewRunStore(dbPath string) *RunStore {
	return &RunStore{dbPath: dbPath}
}

func (s *RunStore) Create(ctx context.Context, run RunRecord) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.ExecContext(ctx, `
		INSERT INTO runs (id, workflow_id, status, current_step, branch, worktree_path, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, run.ID, run.WorkflowID, run.Status, run.CurrentStep, run.Branch, run.WorktreePath, run.CreatedAt)
	return err
}

func (s *RunStore) List(ctx context.Context) ([]RunRecord, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, `
		SELECT id, workflow_id, status, current_step, branch, worktree_path, created_at
		FROM runs
		ORDER BY created_at DESC, id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []RunRecord{}
	for rows.Next() {
		item := RunRecord{Timeline: []TimelineEvent{}}
		if err := rows.Scan(
			&item.ID,
			&item.WorkflowID,
			&item.Status,
			&item.CurrentStep,
			&item.Branch,
			&item.WorktreePath,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := s.attachTimeline(ctx, db, items); err != nil {
		return nil, err
	}
	if err := s.attachSteps(ctx, db, items); err != nil {
		return nil, err
	}
	if err := s.attachHandoffs(ctx, db, items); err != nil {
		return nil, err
	}

	return items, nil
}

func (s *RunStore) Get(ctx context.Context, runID string) (RunRecord, bool, error) {
	db, err := s.open()
	if err != nil {
		return RunRecord{}, false, err
	}
	defer db.Close()

	item := RunRecord{Timeline: []TimelineEvent{}, Steps: []StepRecord{}, Handoffs: []HandoffRecord{}}
	err = db.QueryRowContext(ctx, `
		SELECT id, workflow_id, status, current_step, branch, worktree_path, created_at
		FROM runs
		WHERE id = ?
	`, runID).Scan(
		&item.ID,
		&item.WorkflowID,
		&item.Status,
		&item.CurrentStep,
		&item.Branch,
		&item.WorktreePath,
		&item.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RunRecord{}, false, nil
		}
		return RunRecord{}, false, err
	}

	timeline, err := s.listTimeline(ctx, db, []string{runID})
	if err != nil {
		return RunRecord{}, false, err
	}
	item.Timeline = timeline[runID]
	steps, err := s.listSteps(ctx, db, []string{runID})
	if err != nil {
		return RunRecord{}, false, err
	}
	item.Steps = steps[runID]
	handoffs, err := s.listHandoffs(ctx, db, []string{runID})
	if err != nil {
		return RunRecord{}, false, err
	}
	item.Handoffs = handoffs[runID]

	return item, true, nil
}

func (s *RunStore) AppendTimeline(ctx context.Context, runID string, events []TimelineEvent) error {
	if len(events) == 0 {
		return nil
	}

	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := appendTimelineTx(ctx, tx, runID, events); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *RunStore) TransitionRunState(ctx context.Context, runID, status, currentStep string, event TimelineEvent) error {
	return s.CompareAndSwapRunState(ctx, runID, "", status, currentStep, event)
}

func (s *RunStore) StartStep(ctx context.Context, runID string, step StepRecord, handoff HandoffRecord, events []TimelineEvent) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO run_steps (run_id, step_id, status, worker_id, started_at, finished_at, kind, agent)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, runID, step.StepID, step.Status, step.WorkerID, step.StartedAt, step.FinishedAt, step.Kind, step.Agent); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO run_handoffs (run_id, step_id, status, kind, agent, worker_id, session_id, upstream_run_id, dispatch_attempt_at, finished_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, runID, handoff.StepID, handoff.Status, handoff.Kind, handoff.Agent, handoff.WorkerID, handoff.SessionID, handoff.UpstreamRunID, handoff.DispatchAttemptAt, handoff.FinishedAt, handoff.CreatedAt, handoff.UpdatedAt); err != nil {
		return err
	}
	if err := appendTimelineTx(ctx, tx, runID, events); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *RunStore) MarkHandoffDispatched(ctx context.Context, runID, stepID, sessionID, upstreamRunID, dispatchedAt, expectedStatus string, events []TimelineEvent) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE run_handoffs
		SET status = 'dispatched', session_id = ?, upstream_run_id = ?, dispatch_attempt_at = ?, updated_at = ?
		WHERE run_id = ? AND step_id = ? AND status = ?
	`, sessionID, upstreamRunID, dispatchedAt, dispatchedAt, runID, stepID, expectedStatus)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrRunStateConflict
	}

	if err := appendTimelineTx(ctx, tx, runID, events); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *RunStore) CompleteStepAndTransitionRun(ctx context.Context, runID, stepID, stepStatus, handoffStatus, finishedAt, expectedRunStatus, nextRunStatus, nextCurrentStep string, events []TimelineEvent) error {
	return s.completeStepAndTransitionRunTx(ctx, runID, stepID, stepStatus, handoffStatus, finishedAt, expectedRunStatus, nextRunStatus, nextCurrentStep, events, nil)
}

func (s *RunStore) CompleteStepAndTransitionRunWithApproval(ctx context.Context, runID, stepID, stepStatus, handoffStatus, finishedAt, expectedRunStatus, nextRunStatus, nextCurrentStep string, events []TimelineEvent, approval ApprovalRecord) error {
	return s.completeStepAndTransitionRunTx(ctx, runID, stepID, stepStatus, handoffStatus, finishedAt, expectedRunStatus, nextRunStatus, nextCurrentStep, events, &approval)
}

func (s *RunStore) completeStepAndTransitionRunTx(ctx context.Context, runID, stepID, stepStatus, handoffStatus, finishedAt, expectedRunStatus, nextRunStatus, nextCurrentStep string, events []TimelineEvent, approval *ApprovalRecord) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE run_steps
		SET status = ?, finished_at = ?
		WHERE run_id = ? AND step_id = ? AND status = 'running'
	`, stepStatus, finishedAt, runID, stepID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrRunStateConflict
	}

	result, err = tx.ExecContext(ctx, `
		UPDATE run_handoffs
		SET status = ?, finished_at = ?, updated_at = ?
		WHERE run_id = ? AND step_id = ? AND status IN ('claimed', 'dispatched')
	`, handoffStatus, finishedAt, finishedAt, runID, stepID)
	if err != nil {
		return err
	}
	rowsAffected, err = result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrRunStateConflict
	}

	query := `
		UPDATE runs
		SET status = ?, current_step = ?
		WHERE id = ?
	`
	args := []any{nextRunStatus, nextCurrentStep, runID}
	if strings.TrimSpace(expectedRunStatus) != "" {
		query += ` AND status = ?`
		args = append(args, expectedRunStatus)
	}
	result, err = tx.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rowsAffected, err = result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrRunStateConflict
	}

	if approval != nil {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO run_approvals (id, run_id, step_id, kind, status, approver_policy, requested_by, decided_by, rationale, evidence_manifest, created_at, decided_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, approval.ID, approval.RunID, approval.StepID, approval.Kind, approval.Status, approval.ApproverPolicy, approval.RequestedBy, approval.DecidedBy, approval.Rationale, approval.EvidenceManifest, approval.CreatedAt, approval.DecidedAt); err != nil {
			return err
		}
	}

	if err := appendTimelineTx(ctx, tx, runID, events); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *RunStore) ListApprovals(ctx context.Context) ([]ApprovalRecord, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, `
		SELECT id, run_id, step_id, kind, status, approver_policy, requested_by, decided_by, rationale, evidence_manifest, created_at, decided_at
		FROM run_approvals
		ORDER BY CASE WHEN status = 'pending' THEN 0 ELSE 1 END, created_at DESC, id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []ApprovalRecord{}
	for rows.Next() {
		var item ApprovalRecord
		if err := rows.Scan(&item.ID, &item.RunID, &item.StepID, &item.Kind, &item.Status, &item.ApproverPolicy, &item.RequestedBy, &item.DecidedBy, &item.Rationale, &item.EvidenceManifest, &item.CreatedAt, &item.DecidedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (s *RunStore) GetApproval(ctx context.Context, approvalID string) (ApprovalRecord, bool, error) {
	db, err := s.open()
	if err != nil {
		return ApprovalRecord{}, false, err
	}
	defer db.Close()

	var item ApprovalRecord
	err = db.QueryRowContext(ctx, `
		SELECT id, run_id, step_id, kind, status, approver_policy, requested_by, decided_by, rationale, evidence_manifest, created_at, decided_at
		FROM run_approvals
		WHERE id = ?
	`, approvalID).Scan(&item.ID, &item.RunID, &item.StepID, &item.Kind, &item.Status, &item.ApproverPolicy, &item.RequestedBy, &item.DecidedBy, &item.Rationale, &item.EvidenceManifest, &item.CreatedAt, &item.DecidedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ApprovalRecord{}, false, nil
		}
		return ApprovalRecord{}, false, err
	}

	return item, true, nil
}

func (s *RunStore) GetApprovalByRunAndStep(ctx context.Context, runID, stepID string) (ApprovalRecord, bool, error) {
	db, err := s.open()
	if err != nil {
		return ApprovalRecord{}, false, err
	}
	defer db.Close()

	var item ApprovalRecord
	err = db.QueryRowContext(ctx, `
		SELECT id, run_id, step_id, kind, status, approver_policy, requested_by, decided_by, rationale, evidence_manifest, created_at, decided_at
		FROM run_approvals
		WHERE run_id = ? AND step_id = ?
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, runID, stepID).Scan(&item.ID, &item.RunID, &item.StepID, &item.Kind, &item.Status, &item.ApproverPolicy, &item.RequestedBy, &item.DecidedBy, &item.Rationale, &item.EvidenceManifest, &item.CreatedAt, &item.DecidedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ApprovalRecord{}, false, nil
		}
		return ApprovalRecord{}, false, err
	}

	return item, true, nil
}

func (s *RunStore) DecideApprovalAndTransitionRun(ctx context.Context, approvalID, runID, expectedApprovalStatus, nextApprovalStatus, decidedBy, rationale, decidedAt, expectedRunStatus, nextRunStatus, nextCurrentStep string, events []TimelineEvent) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE run_approvals
		SET status = ?, decided_by = ?, rationale = ?, decided_at = ?
		WHERE id = ? AND run_id = ? AND status = ?
	`, nextApprovalStatus, decidedBy, rationale, decidedAt, approvalID, runID, expectedApprovalStatus)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM run_approvals WHERE id = ?)`, approvalID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrApprovalNotFound
		}
		return ErrRunStateConflict
	}

	result, err = tx.ExecContext(ctx, `
		UPDATE runs
		SET status = ?, current_step = ?
		WHERE id = ? AND status = ?
	`, nextRunStatus, nextCurrentStep, runID, expectedRunStatus)
	if err != nil {
		return err
	}
	rowsAffected, err = result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM runs WHERE id = ?)`, runID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrRunNotFound
		}
		return ErrRunStateConflict
	}

	if err := appendTimelineTx(ctx, tx, runID, events); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *RunStore) ReopenApprovalAndResumeRun(ctx context.Context, approvalID, runID, expectedApprovalStatus, nextApprovalStatus, decidedAt, expectedRunStatus, nextRunStatus, nextCurrentStep string, events []TimelineEvent) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		UPDATE run_approvals
		SET status = ?, decided_by = '', rationale = '', decided_at = ?
		WHERE id = ? AND run_id = ? AND status = ?
	`, nextApprovalStatus, decidedAt, approvalID, runID, expectedApprovalStatus)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM run_approvals WHERE id = ?)`, approvalID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrApprovalNotFound
		}
		return ErrRunStateConflict
	}

	result, err = tx.ExecContext(ctx, `
		UPDATE runs
		SET status = ?, current_step = ?
		WHERE id = ? AND status = ?
	`, nextRunStatus, nextCurrentStep, runID, expectedRunStatus)
	if err != nil {
		return err
	}
	rowsAffected, err = result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM runs WHERE id = ?)`, runID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrRunNotFound
		}
		return ErrRunStateConflict
	}

	if err := appendTimelineTx(ctx, tx, runID, events); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *RunStore) CompareAndSwapRunState(ctx context.Context, runID, expectedStatus, nextStatus, currentStep string, event TimelineEvent) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `
		UPDATE runs
		SET status = ?, current_step = ?
		WHERE id = ?
	`
	args := []any{nextStatus, currentStep, runID}
	if strings.TrimSpace(expectedStatus) != "" {
		query += ` AND status = ?`
		args = append(args, expectedStatus)
	}

	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM runs WHERE id = ?)`, runID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrRunNotFound
		}
		return ErrRunStateConflict
	}

	if err := appendTimelineTx(ctx, tx, runID, []TimelineEvent{event}); err != nil {
		return err
	}

	return tx.Commit()
}

func appendTimelineTx(ctx context.Context, tx *sql.Tx, runID string, events []TimelineEvent) error {
	var baseSeq int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq), 0) FROM run_timeline WHERE run_id = ?`, runID).Scan(&baseSeq); err != nil {
		return err
	}

	for index, event := range events {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO run_timeline (run_id, seq, at, type, detail)
			VALUES (?, ?, ?, ?, ?)
		`, runID, baseSeq+index+1, event.At, event.Type, event.Detail); err != nil {
			return err
		}
	}

	return nil
}

func (s *RunStore) Delete(ctx context.Context, runID string) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.ExecContext(ctx, `DELETE FROM runs WHERE id = ?`, runID)
	return err
}

func (s *RunStore) attachTimeline(ctx context.Context, db *sql.DB, items []RunRecord) error {
	if len(items) == 0 {
		return nil
	}

	runIDs := make([]string, 0, len(items))
	for _, item := range items {
		runIDs = append(runIDs, item.ID)
	}

	byRun, err := s.listTimeline(ctx, db, runIDs)
	if err != nil {
		return err
	}

	for index := range items {
		items[index].Timeline = byRun[items[index].ID]
	}

	return nil
}

func (s *RunStore) listTimeline(ctx context.Context, db *sql.DB, runIDs []string) (map[string][]TimelineEvent, error) {
	if len(runIDs) == 0 {
		return map[string][]TimelineEvent{}, nil
	}

	placeholders := make([]string, 0, len(runIDs))
	args := make([]any, 0, len(runIDs))
	for _, runID := range runIDs {
		placeholders = append(placeholders, "?")
		args = append(args, runID)
	}

	query := fmt.Sprintf(`
		SELECT run_id, at, type, detail
		FROM run_timeline
		WHERE run_id IN (%s)
		ORDER BY run_id, seq ASC
	`, strings.Join(placeholders, ","))

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byRun := make(map[string][]TimelineEvent, len(runIDs))
	for _, runID := range runIDs {
		byRun[runID] = []TimelineEvent{}
	}

	for rows.Next() {
		var runID string
		var event TimelineEvent
		if err := rows.Scan(&runID, &event.At, &event.Type, &event.Detail); err != nil {
			return nil, err
		}
		byRun[runID] = append(byRun[runID], event)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return byRun, nil
}

func (s *RunStore) attachHandoffs(ctx context.Context, db *sql.DB, items []RunRecord) error {
	if len(items) == 0 {
		return nil
	}

	runIDs := make([]string, 0, len(items))
	for _, item := range items {
		runIDs = append(runIDs, item.ID)
	}

	byRun, err := s.listHandoffs(ctx, db, runIDs)
	if err != nil {
		return err
	}

	for index := range items {
		items[index].Handoffs = byRun[items[index].ID]
	}

	return nil
}

func (s *RunStore) attachSteps(ctx context.Context, db *sql.DB, items []RunRecord) error {
	if len(items) == 0 {
		return nil
	}

	runIDs := make([]string, 0, len(items))
	for _, item := range items {
		runIDs = append(runIDs, item.ID)
	}

	byRun, err := s.listSteps(ctx, db, runIDs)
	if err != nil {
		return err
	}

	for index := range items {
		items[index].Steps = byRun[items[index].ID]
	}

	return nil
}

func (s *RunStore) listHandoffs(ctx context.Context, db *sql.DB, runIDs []string) (map[string][]HandoffRecord, error) {
	if len(runIDs) == 0 {
		return map[string][]HandoffRecord{}, nil
	}

	placeholders := make([]string, 0, len(runIDs))
	args := make([]any, 0, len(runIDs))
	for _, runID := range runIDs {
		placeholders = append(placeholders, "?")
		args = append(args, runID)
	}

	query := fmt.Sprintf(`
		SELECT run_id, step_id, status, kind, agent, worker_id, session_id, upstream_run_id, dispatch_attempt_at, finished_at, created_at, updated_at
		FROM run_handoffs
		WHERE run_id IN (%s)
		ORDER BY run_id, created_at ASC, step_id ASC
	`, strings.Join(placeholders, ","))

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byRun := make(map[string][]HandoffRecord, len(runIDs))
	for _, runID := range runIDs {
		byRun[runID] = []HandoffRecord{}
	}

	for rows.Next() {
		var item HandoffRecord
		if err := rows.Scan(&item.RunID, &item.StepID, &item.Status, &item.Kind, &item.Agent, &item.WorkerID, &item.SessionID, &item.UpstreamRunID, &item.DispatchAttemptAt, &item.FinishedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		byRun[item.RunID] = append(byRun[item.RunID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return byRun, nil
}

func (s *RunStore) listSteps(ctx context.Context, db *sql.DB, runIDs []string) (map[string][]StepRecord, error) {
	if len(runIDs) == 0 {
		return map[string][]StepRecord{}, nil
	}

	placeholders := make([]string, 0, len(runIDs))
	args := make([]any, 0, len(runIDs))
	for _, runID := range runIDs {
		placeholders = append(placeholders, "?")
		args = append(args, runID)
	}

	query := fmt.Sprintf(`
		SELECT run_id, step_id, status, worker_id, started_at, finished_at, kind, agent
		FROM run_steps
		WHERE run_id IN (%s)
		ORDER BY run_id, started_at ASC, step_id ASC
	`, strings.Join(placeholders, ","))

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byRun := make(map[string][]StepRecord, len(runIDs))
	for _, runID := range runIDs {
		byRun[runID] = []StepRecord{}
	}

	for rows.Next() {
		var step StepRecord
		if err := rows.Scan(&step.RunID, &step.StepID, &step.Status, &step.WorkerID, &step.StartedAt, &step.FinishedAt, &step.Kind, &step.Agent); err != nil {
			return nil, err
		}
		byRun[step.RunID] = append(byRun[step.RunID], step)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return byRun, nil
}

func (s *RunStore) open() (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(s.dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", s.dbPath)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		db.Close()
		return nil, err
	}

	if err := s.ensureSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func (s *RunStore) ensureSchema(db *sql.DB) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.migrated {
		return nil
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS runs (
			id TEXT PRIMARY KEY,
			workflow_id TEXT NOT NULL,
			status TEXT NOT NULL,
			current_step TEXT NOT NULL DEFAULT '',
			branch TEXT NOT NULL,
			worktree_path TEXT NOT NULL,
			created_at TEXT NOT NULL
		);
	`); err != nil {
		return err
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS run_timeline (
			run_id TEXT NOT NULL,
			seq INTEGER NOT NULL,
			at TEXT NOT NULL,
			type TEXT NOT NULL,
			detail TEXT NOT NULL,
			PRIMARY KEY (run_id, seq),
			FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
		);
	`); err != nil {
		return err
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS run_steps (
			run_id TEXT NOT NULL,
			step_id TEXT NOT NULL,
			status TEXT NOT NULL,
			worker_id TEXT NOT NULL,
			started_at TEXT NOT NULL,
			finished_at TEXT NOT NULL DEFAULT '',
			kind TEXT NOT NULL DEFAULT '',
			agent TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (run_id, step_id),
			FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
		);
	`); err != nil {
		return err
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS run_handoffs (
			run_id TEXT NOT NULL,
			step_id TEXT NOT NULL,
			status TEXT NOT NULL,
			kind TEXT NOT NULL DEFAULT '',
			agent TEXT NOT NULL DEFAULT '',
			worker_id TEXT NOT NULL,
			session_id TEXT NOT NULL DEFAULT '',
			upstream_run_id TEXT NOT NULL DEFAULT '',
			dispatch_attempt_at TEXT NOT NULL DEFAULT '',
			finished_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (run_id, step_id),
			FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
		);
	`); err != nil {
		return err
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS run_approvals (
			id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL,
			step_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			status TEXT NOT NULL,
			approver_policy TEXT NOT NULL DEFAULT '',
			requested_by TEXT NOT NULL DEFAULT '',
			decided_by TEXT NOT NULL DEFAULT '',
			rationale TEXT NOT NULL DEFAULT '',
			evidence_manifest TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			decided_at TEXT NOT NULL DEFAULT '',
			UNIQUE (run_id, step_id),
			FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
		);
	`); err != nil {
		return err
	}

	if _, err := db.Exec(`ALTER TABLE run_handoffs ADD COLUMN upstream_run_id TEXT NOT NULL DEFAULT '';`); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
		return err
	}
	if _, err := db.Exec(`ALTER TABLE run_handoffs ADD COLUMN finished_at TEXT NOT NULL DEFAULT '';`); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
		return err
	}

	s.migrated = true
	return nil
}
