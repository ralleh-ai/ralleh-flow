package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/ralleh-ai/ralleh-flow/apps/api/internal/config"
	"github.com/ralleh-ai/ralleh-flow/apps/api/internal/service"
)

func NewServer(cfg config.Config) *http.Server {
	return NewApp(cfg).Server
}

func newServerWithRuntime(cfg config.Config, workflowService service.WorkflowService, runService *service.RunService, coordinator service.Coordinator, eventBus service.EventBus) *http.Server {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(cors(cfg.AllowedOrigins))

	r.Get("/v1/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "ralleh-flow-api"})
	})

	r.Get("/v1/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if err := coordinator.Ready(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"ok":    false,
				"ready": false,
				"coordination": map[string]any{
					"mode":  coordinator.Mode(),
					"error": err.Error(),
				},
			})
			return
		}
		if err := eventBus.Ready(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"ok":    false,
				"ready": false,
				"coordination": map[string]any{
					"mode": coordinator.Mode(),
				},
				"events": map[string]any{
					"mode":  eventBus.Mode(),
					"error": err.Error(),
				},
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok":    true,
			"ready": true,
			"coordination": map[string]any{
				"mode": coordinator.Mode(),
			},
			"events": map[string]any{
				"mode": eventBus.Mode(),
			},
		})
	})

	r.Get("/v1/workflows", func(w http.ResponseWriter, r *http.Request) {
		items, err := workflowService.List()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	})

	r.Get("/v1/workflows/{workflowId}", func(w http.ResponseWriter, r *http.Request) {
		workflowID := chi.URLParam(r, "workflowId")
		item, found, err := workflowService.Get(workflowID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "workflow not found"})
			return
		}

		writeJSON(w, http.StatusOK, item)
	})

	r.Post("/v1/workflows/{workflowId}/validate", func(w http.ResponseWriter, r *http.Request) {
		workflowID := chi.URLParam(r, "workflowId")
		result, found, err := workflowService.Validate(workflowID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "workflow not found"})
			return
		}

		writeJSON(w, http.StatusOK, result)
	})

	r.Post("/v1/workflows/{workflowId}/dry-run", func(w http.ResponseWriter, r *http.Request) {
		workflowID := chi.URLParam(r, "workflowId")
		var input struct {
			Variables map[string]string `json:"variables"`
		}
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil && !errors.Is(err, io.EOF) {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
				return
			}
		}

		result, found, err := workflowService.DryRun(workflowID, input.Variables)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "workflow not found"})
			return
		}

		writeJSON(w, http.StatusOK, result)
	})

	r.Get("/v1/runs", func(w http.ResponseWriter, r *http.Request) {
		items, err := runService.List(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	})

	r.Get("/v1/approvals", func(w http.ResponseWriter, r *http.Request) {
		items, err := runService.ListApprovals(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"items": items})
	})

	r.Post("/v1/approvals/{approvalId}/approve", func(w http.ResponseWriter, r *http.Request) {
		approvalID := chi.URLParam(r, "approvalId")
		var input struct {
			DecidedBy string `json:"decidedBy"`
			Rationale string `json:"rationale"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
			return
		}
		run, err := runService.ApproveApproval(r.Context(), approvalID, service.ApprovalDecisionInput{DecidedBy: input.DecidedBy, Rationale: input.Rationale})
		if err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidRunInput):
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "approvalId is required"})
			case errors.Is(err, service.ErrApprovalNotFound):
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "approval not found"})
			case errors.Is(err, service.ErrRunLeaseHeld), errors.Is(err, service.ErrRunStateConflict):
				writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			}
			return
		}
		writeJSON(w, http.StatusOK, run)
	})

	r.Post("/v1/approvals/{approvalId}/reject", func(w http.ResponseWriter, r *http.Request) {
		approvalID := chi.URLParam(r, "approvalId")
		var input struct {
			DecidedBy string `json:"decidedBy"`
			Rationale string `json:"rationale"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
			return
		}
		run, err := runService.RejectApproval(r.Context(), approvalID, service.ApprovalDecisionInput{DecidedBy: input.DecidedBy, Rationale: input.Rationale})
		if err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidRunInput):
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "approvalId is required"})
			case errors.Is(err, service.ErrApprovalNotFound):
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "approval not found"})
			case errors.Is(err, service.ErrRunLeaseHeld), errors.Is(err, service.ErrRunStateConflict):
				writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			}
			return
		}
		writeJSON(w, http.StatusOK, run)
	})

	r.Post("/v1/runs", func(w http.ResponseWriter, r *http.Request) {
		var input service.CreateRunInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
			return
		}

		run, err := runService.Create(r.Context(), input)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidRunInput):
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "workflowId is required"})
			case errors.Is(err, service.ErrMissingWorkflowVariables):
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			case errors.Is(err, service.ErrWorkflowMissing):
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "workflow not found"})
			case errors.Is(err, service.ErrExecutionRepoNotReady), errors.Is(err, service.ErrRepositoryLocked), errors.Is(err, service.ErrRunLeaseHeld):
				writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			}
			return
		}

		writeJSON(w, http.StatusCreated, run)
	})

	r.Get("/v1/runs/{runId}", func(w http.ResponseWriter, r *http.Request) {
		runID := chi.URLParam(r, "runId")
		run, err := runService.Get(r.Context(), runID)
		if err != nil {
			if errors.Is(err, service.ErrRunNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "run not found"})
				return
			}

			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, run)
	})

	r.Post("/v1/runs/{runId}/advance", func(w http.ResponseWriter, r *http.Request) {
		runID := chi.URLParam(r, "runId")
		run, err := runService.AdvancePendingRun(r.Context(), runID)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidRunInput):
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "runId is required"})
			case errors.Is(err, service.ErrRunNotFound):
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "run not found"})
			case errors.Is(err, service.ErrRunLeaseHeld), errors.Is(err, service.ErrRunStateConflict):
				writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			}
			return
		}

		writeJSON(w, http.StatusOK, run)
	})

	r.Post("/v1/runs/{runId}/dispatch-step", func(w http.ResponseWriter, r *http.Request) {
		runID := chi.URLParam(r, "runId")
		var input struct {
			SessionID string `json:"sessionId"`
			Note      string `json:"note"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
			return
		}
		run, err := runService.DispatchActiveStep(r.Context(), runID, service.HandoffDispatchInput{SessionID: input.SessionID, Note: input.Note})
		if err != nil {
			var dispatchErr *service.DispatchError
			switch {
			case errors.Is(err, service.ErrInvalidRunInput):
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "runId is required"})
			case errors.Is(err, service.ErrRunNotFound):
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "run not found"})
			case errors.Is(err, service.ErrRunLeaseHeld), errors.Is(err, service.ErrRunStateConflict):
				writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
			case errors.As(err, &dispatchErr):
				writeJSON(w, http.StatusBadGateway, map[string]any{"error": dispatchErr.Error()})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			}
			return
		}
		writeJSON(w, http.StatusOK, run)
	})

	r.Post("/v1/runs/{runId}/complete-step", func(w http.ResponseWriter, r *http.Request) {
		runID := chi.URLParam(r, "runId")
		var input struct {
			Summary         string `json:"summary"`
			CheckpointLabel string `json:"checkpointLabel"`
			SessionID       string `json:"sessionId"`
			UpstreamRunID   string `json:"upstreamRunId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
			return
		}
		run, err := runService.CompleteActiveStep(r.Context(), runID, service.StepCompletionInput{Summary: input.Summary, CheckpointLabel: input.CheckpointLabel, SessionID: input.SessionID, UpstreamRunID: input.UpstreamRunID})
		if err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidRunInput):
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "runId is required"})
			case errors.Is(err, service.ErrRunNotFound):
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "run not found"})
			case errors.Is(err, service.ErrRunLeaseHeld), errors.Is(err, service.ErrRunStateConflict):
				writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			}
			return
		}
		writeJSON(w, http.StatusOK, run)
	})

	r.Post("/v1/runs/{runId}/fail-step", func(w http.ResponseWriter, r *http.Request) {
		runID := chi.URLParam(r, "runId")
		var input struct {
			Reason        string `json:"reason"`
			SessionID     string `json:"sessionId"`
			UpstreamRunID string `json:"upstreamRunId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
			return
		}
		run, err := runService.FailActiveStep(r.Context(), runID, service.StepFailureInput{Reason: input.Reason, SessionID: input.SessionID, UpstreamRunID: input.UpstreamRunID})
		if err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidRunInput):
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "runId is required"})
			case errors.Is(err, service.ErrRunNotFound):
				writeJSON(w, http.StatusNotFound, map[string]any{"error": "run not found"})
			case errors.Is(err, service.ErrRunLeaseHeld), errors.Is(err, service.ErrRunStateConflict):
				writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			}
			return
		}
		writeJSON(w, http.StatusOK, run)
	})

	return &http.Server{Addr: cfg.APIAddr, Handler: r}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func cors(allowedOrigins string) func(http.Handler) http.Handler {
	allowed := strings.Split(allowedOrigins, ",")
	allowedMap := make(map[string]struct{}, len(allowed))
	for _, origin := range allowed {
		trimmed := strings.TrimSpace(origin)
		if trimmed != "" {
			allowedMap[trimmed] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if _, ok := allowedMap[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
