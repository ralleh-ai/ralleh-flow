package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenClawHookDispatcherDispatchesAgentTask(t *testing.T) {
	t.Parallel()

	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer hook-secret" {
			t.Fatalf("expected bearer auth, got %q", got)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		writeDispatcherTestJSON(t, w, http.StatusOK, map[string]any{"ok": true, "runId": "hook-run-123"})
	}))
	defer server.Close()

	dispatcher := NewOpenClawHookDispatcher(OpenClawHookDispatcherConfig{
		EndpointURL:      server.URL,
		Token:            "hook-secret",
		SessionKeyPrefix: "hook:ralleh-flow",
		Model:            "gemini-flash-lite",
		Thinking:         "medium",
	})

	result, err := dispatcher.DispatchAgentTask(context.Background(), DispatchRequest{
		Run:                 RunRecord{ID: "run-123", Branch: "workflow/feature-development/run-123"},
		Step:                StepRecord{StepID: "research", Kind: "agent_task", Agent: "carmack"},
		Workflow:            WorkflowDetail{ID: "feature-development", Description: "Build the slice cleanly."},
		Note:                "continue on slice",
		RepoRoot:            "/repo",
		Worktree:            "/repo/runtime/runs/run-123/workspace",
		HandoffDocumentPath: "/repo/runtime/runs/run-123/workspace/HANDOFF.md",
	})
	if err != nil {
		t.Fatalf("dispatch agent task: %v", err)
	}
	if result.SessionID != "hook:ralleh-flow:run-123:research" {
		t.Fatalf("expected generated session id, got %#v", result)
	}
	if result.UpstreamRunID != "hook-run-123" {
		t.Fatalf("expected upstream run id, got %#v", result)
	}
	if !strings.Contains(result.Note, "hook-run-123") {
		t.Fatalf("expected upstream run id in note, got %#v", result)
	}
	if dispatcher.Mode() != "openclaw-hooks" {
		t.Fatalf("expected openclaw-hooks mode, got %q", dispatcher.Mode())
	}
	if captured["agentId"] != "carmack" {
		t.Fatalf("expected step agent in payload, got %#v", captured)
	}
	if captured["sessionKey"] != "hook:ralleh-flow:run-123:research" {
		t.Fatalf("expected session key in payload, got %#v", captured)
	}
	if captured["deliver"] != false {
		t.Fatalf("expected deliver=false, got %#v", captured)
	}
	if captured["idempotencyKey"] != "ralleh-flow:run-123:research" {
		t.Fatalf("expected idempotency key, got %#v", captured)
	}
	message, _ := captured["message"].(string)
	if !strings.Contains(message, "Run ID: run-123") || !strings.Contains(message, "Operator note: continue on slice") || !strings.Contains(message, "Primary handoff doc: /repo/runtime/runs/run-123/workspace/HANDOFF.md") {
		t.Fatalf("expected structured message, got %q", message)
	}
}

func TestOpenClawHookDispatcherReturnsCleanFailureWhenHookRejects(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeDispatcherTestJSON(t, w, http.StatusBadRequest, map[string]any{"ok": false, "error": "sessionKey is disabled for externally supplied hook payload values; set hooks.allowRequestSessionKey=true to enable"})
	}))
	defer server.Close()

	dispatcher := NewOpenClawHookDispatcher(OpenClawHookDispatcherConfig{
		EndpointURL:      server.URL,
		Token:            "hook-secret",
		SessionKeyPrefix: "hook:ralleh-flow",
	})

	_, err := dispatcher.DispatchAgentTask(context.Background(), DispatchRequest{
		Run:      RunRecord{ID: "run-123"},
		Step:     StepRecord{StepID: "research", Kind: "agent_task", Agent: "carmack"},
		Workflow: WorkflowDetail{ID: "feature-development"},
	})
	if err == nil {
		t.Fatal("expected dispatch error")
	}
	dispatchErr, ok := err.(*DispatchError)
	if !ok {
		t.Fatalf("expected DispatchError, got %T (%v)", err, err)
	}
	if dispatchErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 status, got %#v", dispatchErr)
	}
	if !strings.Contains(dispatchErr.Error(), "hooks.allowRequestSessionKey=true") {
		t.Fatalf("expected explicit hook policy failure, got %v", dispatchErr)
	}
}

func writeDispatcherTestJSON(t *testing.T, w http.ResponseWriter, status int, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("encode payload: %v", err)
	}
}
