package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ralleh-ai/ralleh-flow/apps/api/internal/config"
)

func TestHealthz(t *testing.T) {
	server := NewServer(config.Config{APIAddr: ":0", AllowedOrigins: "http://localhost:4300"})
	req := httptest.NewRequest(http.MethodGet, "/v1/healthz", nil)
	res := httptest.NewRecorder()

	server.Handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if ok, _ := body["ok"].(bool); !ok {
		t.Fatalf("expected ok=true, got %#v", body)
	}
}

func TestReadyzUsesMemoryCoordinatorByDefault(t *testing.T) {
	server := NewServer(config.Config{APIAddr: ":0", AllowedOrigins: "http://localhost:4300"})
	res := httptest.NewRecorder()
	server.Handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/v1/readyz", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"coordination":{"mode":"memory"}`) {
		t.Fatalf("expected memory coordination mode, got %s", res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"events":{"mode":"memory"}`) {
		t.Fatalf("expected memory event mode, got %s", res.Body.String())
	}
}

func TestReadyzFailsWhenRedisConfiguredButUnavailable(t *testing.T) {
	server := NewServer(config.Config{APIAddr: ":0", AllowedOrigins: "http://localhost:4300", RedisAddr: "127.0.0.1:1"})
	res := httptest.NewRecorder()
	server.Handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/v1/readyz", nil))

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d (%s)", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"mode":"redis"`) {
		t.Fatalf("expected redis coordination mode, got %s", res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `connection refused`) {
		t.Fatalf("expected redis connection error detail, got %s", res.Body.String())
	}
}

func TestApproveApprovalEndpointAdvancesRun(t *testing.T) {
	repoRoot := t.TempDir()
	workflowPath := filepath.Join(repoRoot, "workflows", "examples", "feature-development", "workflow.yaml")
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	workflowYAML := `metadata:
  id: feature-development
  name: Feature Development
  version: 0.1.0
steps:
  - id: research
    kind: agent_task
  - id: review
    kind: human_approval
    approverPolicy: reviewer
  - id: publish
    kind: agent_task
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	initGitRepoWithCommit(t, repoRoot)

	app := NewApp(config.Config{
		APIAddr:        ":0",
		AllowedOrigins: "http://localhost:4300",
		RepoRoot:       repoRoot,
		DBPath:         filepath.Join(repoRoot, "data", "ralleh-flow.db"),
	})

	body := []byte(`{"workflowId":"feature-development"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", createRes.Code, createRes.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created run: %v", err)
	}
	runID, _ := created["id"].(string)

	advanceReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/advance", runID), nil)
	advanceRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(advanceRes, advanceReq)
	if advanceRes.Code != http.StatusOK {
		t.Fatalf("advance expected 200, got %d (%s)", advanceRes.Code, advanceRes.Body.String())
	}

	dispatchBody := []byte(`{"sessionId":"session:research"}`)
	dispatchReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/dispatch-step", runID), bytes.NewReader(dispatchBody))
	dispatchReq.Header.Set("Content-Type", "application/json")
	dispatchRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(dispatchRes, dispatchReq)
	if dispatchRes.Code != http.StatusOK {
		t.Fatalf("dispatch expected 200, got %d (%s)", dispatchRes.Code, dispatchRes.Body.String())
	}

	completeBody := []byte(`{"summary":"Research completed"}`)
	completeReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/complete-step", runID), bytes.NewReader(completeBody))
	completeReq.Header.Set("Content-Type", "application/json")
	completeRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(completeRes, completeReq)
	if completeRes.Code != http.StatusOK {
		t.Fatalf("complete expected 200, got %d (%s)", completeRes.Code, completeRes.Body.String())
	}

	approvalsReq := httptest.NewRequest(http.MethodGet, "/v1/approvals", nil)
	approvalsRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(approvalsRes, approvalsReq)
	if approvalsRes.Code != http.StatusOK {
		t.Fatalf("approvals expected 200, got %d (%s)", approvalsRes.Code, approvalsRes.Body.String())
	}
	var approvalsBody map[string][]map[string]any
	if err := json.Unmarshal(approvalsRes.Body.Bytes(), &approvalsBody); err != nil {
		t.Fatalf("decode approvals: %v", err)
	}
	items := approvalsBody["items"]
	if len(items) != 1 {
		t.Fatalf("expected 1 approval, got %#v", approvalsBody)
	}
	approvalID, _ := items[0]["id"].(string)

	approveBody := []byte(`{"decidedBy":"rick","rationale":"Ship it"}`)
	approveReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/approvals/%s/approve", approvalID), bytes.NewReader(approveBody))
	approveReq.Header.Set("Content-Type", "application/json")
	approveRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(approveRes, approveReq)
	if approveRes.Code != http.StatusOK {
		t.Fatalf("approve expected 200, got %d (%s)", approveRes.Code, approveRes.Body.String())
	}
	if !strings.Contains(approveRes.Body.String(), `"status":"pending"`) {
		t.Fatalf("expected pending run after approval, got %s", approveRes.Body.String())
	}
	if !strings.Contains(approveRes.Body.String(), `"currentStep":"publish"`) {
		t.Fatalf("expected publish step after approval, got %s", approveRes.Body.String())
	}
}

func TestRequestChangesApprovalEndpointPausesRun(t *testing.T) {
	repoRoot := t.TempDir()
	workflowPath := filepath.Join(repoRoot, "workflows", "examples", "feature-development", "workflow.yaml")
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	workflowYAML := `metadata:
  id: feature-development
  name: Feature Development
  version: 0.1.0
steps:
  - id: research
    kind: agent_task
  - id: review
    kind: human_approval
    approverPolicy: reviewer
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	initGitRepoWithCommit(t, repoRoot)

	app := NewApp(config.Config{
		APIAddr:        ":0",
		AllowedOrigins: "http://localhost:4300",
		RepoRoot:       repoRoot,
		DBPath:         filepath.Join(repoRoot, "data", "ralleh-flow.db"),
	})

	body := []byte(`{"workflowId":"feature-development"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", createRes.Code, createRes.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created run: %v", err)
	}
	runID, _ := created["id"].(string)

	advanceReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/advance", runID), nil)
	advanceRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(advanceRes, advanceReq)
	if advanceRes.Code != http.StatusOK {
		t.Fatalf("advance expected 200, got %d (%s)", advanceRes.Code, advanceRes.Body.String())
	}

	dispatchBody := []byte(`{"sessionId":"session:research"}`)
	dispatchReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/dispatch-step", runID), bytes.NewReader(dispatchBody))
	dispatchReq.Header.Set("Content-Type", "application/json")
	dispatchRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(dispatchRes, dispatchReq)
	if dispatchRes.Code != http.StatusOK {
		t.Fatalf("dispatch expected 200, got %d (%s)", dispatchRes.Code, dispatchRes.Body.String())
	}

	completeBody := []byte(`{"summary":"Research completed"}`)
	completeReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/complete-step", runID), bytes.NewReader(completeBody))
	completeReq.Header.Set("Content-Type", "application/json")
	completeRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(completeRes, completeReq)
	if completeRes.Code != http.StatusOK {
		t.Fatalf("complete expected 200, got %d (%s)", completeRes.Code, completeRes.Body.String())
	}

	approvalsReq := httptest.NewRequest(http.MethodGet, "/v1/approvals", nil)
	approvalsRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(approvalsRes, approvalsReq)
	if approvalsRes.Code != http.StatusOK {
		t.Fatalf("approvals expected 200, got %d (%s)", approvalsRes.Code, approvalsRes.Body.String())
	}
	var approvalsBody map[string][]map[string]any
	if err := json.Unmarshal(approvalsRes.Body.Bytes(), &approvalsBody); err != nil {
		t.Fatalf("decode approvals: %v", err)
	}
	items := approvalsBody["items"]
	if len(items) != 1 {
		t.Fatalf("expected 1 approval, got %#v", approvalsBody)
	}
	approvalID, _ := items[0]["id"].(string)

	changesBody := []byte(`{"decidedBy":"rick","rationale":"Needs tighter review notes"}`)
	changesReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/approvals/%s/request-changes", approvalID), bytes.NewReader(changesBody))
	changesReq.Header.Set("Content-Type", "application/json")
	changesRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(changesRes, changesReq)
	if changesRes.Code != http.StatusOK {
		t.Fatalf("request changes expected 200, got %d (%s)", changesRes.Code, changesRes.Body.String())
	}
	if !strings.Contains(changesRes.Body.String(), `"status":"changes_requested"`) {
		t.Fatalf("expected changes_requested run after request changes, got %s", changesRes.Body.String())
	}
	if !strings.Contains(changesRes.Body.String(), `"currentStep":"review"`) {
		t.Fatalf("expected review step to remain current after request changes, got %s", changesRes.Body.String())
	}
}

func TestRejectApprovalEndpointFailsRun(t *testing.T) {
	repoRoot := t.TempDir()
	workflowPath := filepath.Join(repoRoot, "workflows", "examples", "feature-development", "workflow.yaml")
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	workflowYAML := `metadata:
  id: feature-development
  name: Feature Development
  version: 0.1.0
steps:
  - id: research
    kind: agent_task
  - id: review
    kind: human_approval
    approverPolicy: reviewer
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
	initGitRepoWithCommit(t, repoRoot)

	app := NewApp(config.Config{
		APIAddr:        ":0",
		AllowedOrigins: "http://localhost:4300",
		RepoRoot:       repoRoot,
		DBPath:         filepath.Join(repoRoot, "data", "ralleh-flow.db"),
	})

	body := []byte(`{"workflowId":"feature-development"}`)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", createRes.Code, createRes.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created run: %v", err)
	}
	runID, _ := created["id"].(string)

	advanceReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/advance", runID), nil)
	advanceRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(advanceRes, advanceReq)
	if advanceRes.Code != http.StatusOK {
		t.Fatalf("advance expected 200, got %d (%s)", advanceRes.Code, advanceRes.Body.String())
	}

	dispatchBody := []byte(`{"sessionId":"session:research"}`)
	dispatchReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/dispatch-step", runID), bytes.NewReader(dispatchBody))
	dispatchReq.Header.Set("Content-Type", "application/json")
	dispatchRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(dispatchRes, dispatchReq)
	if dispatchRes.Code != http.StatusOK {
		t.Fatalf("dispatch expected 200, got %d (%s)", dispatchRes.Code, dispatchRes.Body.String())
	}

	completeBody := []byte(`{"summary":"Research completed"}`)
	completeReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/complete-step", runID), bytes.NewReader(completeBody))
	completeReq.Header.Set("Content-Type", "application/json")
	completeRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(completeRes, completeReq)
	if completeRes.Code != http.StatusOK {
		t.Fatalf("complete expected 200, got %d (%s)", completeRes.Code, completeRes.Body.String())
	}

	approvalsReq := httptest.NewRequest(http.MethodGet, "/v1/approvals", nil)
	approvalsRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(approvalsRes, approvalsReq)
	if approvalsRes.Code != http.StatusOK {
		t.Fatalf("approvals expected 200, got %d (%s)", approvalsRes.Code, approvalsRes.Body.String())
	}
	var approvalsBody map[string][]map[string]any
	if err := json.Unmarshal(approvalsRes.Body.Bytes(), &approvalsBody); err != nil {
		t.Fatalf("decode approvals: %v", err)
	}
	items := approvalsBody["items"]
	if len(items) != 1 {
		t.Fatalf("expected 1 approval, got %#v", approvalsBody)
	}
	approvalID, _ := items[0]["id"].(string)

	rejectBody := []byte(`{"decidedBy":"rick","rationale":"Needs work"}`)
	rejectReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/approvals/%s/reject", approvalID), bytes.NewReader(rejectBody))
	rejectReq.Header.Set("Content-Type", "application/json")
	rejectRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(rejectRes, rejectReq)
	if rejectRes.Code != http.StatusOK {
		t.Fatalf("reject expected 200, got %d (%s)", rejectRes.Code, rejectRes.Body.String())
	}
	if !strings.Contains(rejectRes.Body.String(), `"status":"failed"`) {
		t.Fatalf("expected failed run after rejection, got %s", rejectRes.Body.String())
	}
	if !strings.Contains(rejectRes.Body.String(), `"currentStep":"review"`) {
		t.Fatalf("expected review step to remain current after rejection, got %s", rejectRes.Body.String())
	}
}

func TestCreateRunAndListAndGet(t *testing.T) {
	repoRoot := t.TempDir()
	workflowPath := filepath.Join(repoRoot, "workflows", "examples", "feature-development", "workflow.yaml")
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	workflowYAML := `metadata:
  id: feature-development
  name: Feature Development
  version: 0.1.0
  description: Build a feature end-to-end.
steps:
  - id: research
    kind: agent_task
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	initGitRepoWithCommit(t, repoRoot)

	server := NewServer(config.Config{
		APIAddr:        ":0",
		AllowedOrigins: "http://localhost:4300",
		RepoRoot:       repoRoot,
		DBPath:         filepath.Join(repoRoot, "data", "ralleh-flow.db"),
	})

	body := []byte(`{"workflowId":"feature-development"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	server.Handler.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", res.Code, res.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created run: %v", err)
	}

	runID, _ := created["id"].(string)
	if runID == "" {
		t.Fatalf("expected run id, got %#v", created)
	}

	branch, _ := created["branch"].(string)
	if !strings.Contains(branch, runID) {
		t.Fatalf("expected branch to include run id %q, got %q", runID, branch)
	}

	worktreePath, _ := created["worktreePath"].(string)
	if !strings.Contains(worktreePath, filepath.Join("runtime", "runs", runID)) {
		t.Fatalf("expected worktreePath to be namespaced by run id %q, got %q", runID, worktreePath)
	}

	for _, dir := range []string{"logs", "snapshots", "handoffs", "artifacts"} {
		path := filepath.Join(repoRoot, "runtime", "runs", runID, dir)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}

	gitDir, err := os.ReadFile(filepath.Join(worktreePath, ".git"))
	if err != nil {
		t.Fatalf("expected worktree .git file: %v", err)
	}
	if !strings.Contains(string(gitDir), ".git/worktrees/") {
		t.Fatalf("expected git worktree metadata, got %s", string(gitDir))
	}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/runs", nil)
	listRes := httptest.NewRecorder()
	server.Handler.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("expected list 200, got %d", listRes.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/runs/%s", runID), nil)
	getRes := httptest.NewRecorder()
	server.Handler.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("expected get 200, got %d (%s)", getRes.Code, getRes.Body.String())
	}

	var fetched map[string]any
	if err := json.Unmarshal(getRes.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("decode fetched run: %v", err)
	}

	timeline, ok := fetched["timeline"].([]any)
	if !ok || len(timeline) < 3 {
		t.Fatalf("expected persisted timeline events, got %#v", fetched["timeline"])
	}
	if status, _ := fetched["status"].(string); status != "pending" {
		t.Fatalf("expected created run status pending, got %#v", fetched["status"])
	}
}

func TestAdvanceRunReturnsPersistedHandoffRecords(t *testing.T) {
	repoRoot := t.TempDir()
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
    agent: carmack
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	initGitRepoWithCommit(t, repoRoot)

	server := NewServer(config.Config{
		APIAddr:        ":0",
		AllowedOrigins: "http://localhost:4300",
		RepoRoot:       repoRoot,
		DBPath:         filepath.Join(repoRoot, "data", "ralleh-flow.db"),
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader([]byte(`{"workflowId":"feature-development","variables":{"feature_name":"Flow Polish","target_repo":"ralleh-flow"}}`)))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	server.Handler.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", createRes.Code, createRes.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created run: %v", err)
	}
	runID, _ := created["id"].(string)

	advanceReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/advance", runID), nil)
	advanceRes := httptest.NewRecorder()
	server.Handler.ServeHTTP(advanceRes, advanceReq)
	if advanceRes.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", advanceRes.Code, advanceRes.Body.String())
	}
	if !strings.Contains(advanceRes.Body.String(), `"handoffs":[{`) {
		t.Fatalf("expected handoffs in response, got %s", advanceRes.Body.String())
	}
	if !strings.Contains(advanceRes.Body.String(), `"status":"claimed"`) {
		t.Fatalf("expected claimed handoff status, got %s", advanceRes.Body.String())
	}
}

func TestListApprovalsReturnsPersistedApprovalRequests(t *testing.T) {
	repoRoot := t.TempDir()
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
    agent: carmack
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

	initGitRepoWithCommit(t, repoRoot)

	server := NewServer(config.Config{
		APIAddr:        ":0",
		AllowedOrigins: "http://localhost:4300",
		RepoRoot:       repoRoot,
		DBPath:         filepath.Join(repoRoot, "data", "ralleh-flow.db"),
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader([]byte(`{"workflowId":"feature-development","variables":{"feature_name":"Flow Polish","target_repo":"ralleh-flow"}}`)))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	server.Handler.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", createRes.Code, createRes.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created run: %v", err)
	}
	runID, _ := created["id"].(string)

	for _, sessionID := range []string{"session:research", "session:implement"} {
		advanceReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/advance", runID), nil)
		advanceRes := httptest.NewRecorder()
		server.Handler.ServeHTTP(advanceRes, advanceReq)
		if advanceRes.Code != http.StatusOK {
			t.Fatalf("expected advance 200, got %d (%s)", advanceRes.Code, advanceRes.Body.String())
		}

		dispatchReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/dispatch-step", runID), bytes.NewReader([]byte(fmt.Sprintf(`{"sessionId":"%s"}`, sessionID))))
		dispatchReq.Header.Set("Content-Type", "application/json")
		dispatchRes := httptest.NewRecorder()
		server.Handler.ServeHTTP(dispatchRes, dispatchReq)
		if dispatchRes.Code != http.StatusOK {
			t.Fatalf("expected dispatch 200, got %d (%s)", dispatchRes.Code, dispatchRes.Body.String())
		}

		completeReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/complete-step", runID), bytes.NewReader([]byte(`{"summary":"done"}`)))
		completeReq.Header.Set("Content-Type", "application/json")
		completeRes := httptest.NewRecorder()
		server.Handler.ServeHTTP(completeRes, completeReq)
		if completeRes.Code != http.StatusOK {
			t.Fatalf("expected complete 200, got %d (%s)", completeRes.Code, completeRes.Body.String())
		}
	}

	approvalsReq := httptest.NewRequest(http.MethodGet, "/v1/approvals", nil)
	approvalsRes := httptest.NewRecorder()
	server.Handler.ServeHTTP(approvalsRes, approvalsReq)
	if approvalsRes.Code != http.StatusOK {
		t.Fatalf("expected approvals 200, got %d (%s)", approvalsRes.Code, approvalsRes.Body.String())
	}
	if !strings.Contains(approvalsRes.Body.String(), `"items":[{`) {
		t.Fatalf("expected approval items in response, got %s", approvalsRes.Body.String())
	}
	if !strings.Contains(approvalsRes.Body.String(), `"runId":"`+runID+`"`) {
		t.Fatalf("expected approval run id %q, got %s", runID, approvalsRes.Body.String())
	}
	if !strings.Contains(approvalsRes.Body.String(), `"stepId":"review"`) {
		t.Fatalf("expected approval step review, got %s", approvalsRes.Body.String())
	}
	if !strings.Contains(approvalsRes.Body.String(), `"status":"pending"`) {
		t.Fatalf("expected pending approval, got %s", approvalsRes.Body.String())
	}
	if !strings.Contains(approvalsRes.Body.String(), `"approverPolicy":"reviewer"`) {
		t.Fatalf("expected reviewer policy, got %s", approvalsRes.Body.String())
	}
}

func TestNewAppUsesDefaultDispatcherForDispatchStep(t *testing.T) {
	repoRoot := t.TempDir()
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
    agent: carmack
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	initGitRepoWithCommit(t, repoRoot)

	app := NewApp(config.Config{
		APIAddr:        ":0",
		AllowedOrigins: "http://localhost:4300",
		RepoRoot:       repoRoot,
		DBPath:         filepath.Join(repoRoot, "data", "ralleh-flow.db"),
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader([]byte(`{"workflowId":"feature-development","variables":{"feature_name":"Flow Polish","target_repo":"ralleh-flow"}}`)))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", createRes.Code, createRes.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created run: %v", err)
	}
	runID, _ := created["id"].(string)

	advanceReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/advance", runID), nil)
	advanceRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(advanceRes, advanceReq)
	if advanceRes.Code != http.StatusOK {
		t.Fatalf("expected advance 200, got %d (%s)", advanceRes.Code, advanceRes.Body.String())
	}

	dispatchReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/dispatch-step", runID), bytes.NewReader([]byte(`{"note":"default dispatcher"}`)))
	dispatchReq.Header.Set("Content-Type", "application/json")
	dispatchRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(dispatchRes, dispatchReq)
	if dispatchRes.Code != http.StatusOK {
		t.Fatalf("expected dispatch 200, got %d (%s)", dispatchRes.Code, dispatchRes.Body.String())
	}
	if !strings.Contains(dispatchRes.Body.String(), `"status":"dispatched"`) {
		t.Fatalf("expected dispatched status, got %s", dispatchRes.Body.String())
	}
	if !strings.Contains(dispatchRes.Body.String(), `"sessionId":"pending:`) {
		t.Fatalf("expected noop dispatcher fallback session binding, got %s", dispatchRes.Body.String())
	}
}

func TestDispatchStepPersistsSessionBinding(t *testing.T) {
	repoRoot := t.TempDir()
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
    agent: carmack
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	initGitRepoWithCommit(t, repoRoot)

	server := NewServer(config.Config{
		APIAddr:        ":0",
		AllowedOrigins: "http://localhost:4300",
		RepoRoot:       repoRoot,
		DBPath:         filepath.Join(repoRoot, "data", "ralleh-flow.db"),
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader([]byte(`{"workflowId":"feature-development","variables":{"feature_name":"Flow Polish","target_repo":"ralleh-flow"}}`)))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	server.Handler.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", createRes.Code, createRes.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created run: %v", err)
	}
	runID, _ := created["id"].(string)

	advanceReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/advance", runID), nil)
	advanceRes := httptest.NewRecorder()
	server.Handler.ServeHTTP(advanceRes, advanceReq)
	if advanceRes.Code != http.StatusOK {
		t.Fatalf("expected advance 200, got %d (%s)", advanceRes.Code, advanceRes.Body.String())
	}

	dispatchReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/dispatch-step", runID), bytes.NewReader([]byte(`{"sessionId":"session:abc123","note":"dispatch bound"}`)))
	dispatchReq.Header.Set("Content-Type", "application/json")
	dispatchRes := httptest.NewRecorder()
	server.Handler.ServeHTTP(dispatchRes, dispatchReq)
	if dispatchRes.Code != http.StatusOK {
		t.Fatalf("expected dispatch 200, got %d (%s)", dispatchRes.Code, dispatchRes.Body.String())
	}
	if !strings.Contains(dispatchRes.Body.String(), `"status":"dispatched"`) {
		t.Fatalf("expected dispatched handoff status, got %s", dispatchRes.Body.String())
	}
	if !strings.Contains(dispatchRes.Body.String(), `"sessionId":"session:abc123"`) {
		t.Fatalf("expected session binding, got %s", dispatchRes.Body.String())
	}
}

func TestAdvanceRunPromotesPendingRunToRunning(t *testing.T) {
	repoRoot := t.TempDir()
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
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	initGitRepoWithCommit(t, repoRoot)

	server := NewServer(config.Config{
		APIAddr:        ":0",
		AllowedOrigins: "http://localhost:4300",
		RepoRoot:       repoRoot,
		DBPath:         filepath.Join(repoRoot, "data", "ralleh-flow.db"),
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader([]byte(`{"workflowId":"feature-development","variables":{"feature_name":"Flow Polish","target_repo":"ralleh-flow"}}`)))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	server.Handler.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", createRes.Code, createRes.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created run: %v", err)
	}
	runID, _ := created["id"].(string)
	if runID == "" {
		t.Fatalf("expected run id, got %#v", created)
	}

	advanceReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/advance", runID), nil)
	advanceRes := httptest.NewRecorder()
	server.Handler.ServeHTTP(advanceRes, advanceReq)
	if advanceRes.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", advanceRes.Code, advanceRes.Body.String())
	}
	if !strings.Contains(advanceRes.Body.String(), `"status":"running"`) {
		t.Fatalf("expected running status, got %s", advanceRes.Body.String())
	}
	if !strings.Contains(advanceRes.Body.String(), `run.started`) {
		t.Fatalf("expected run.started event, got %s", advanceRes.Body.String())
	}
}

func TestAdvanceRunRejectsSecondAdvance(t *testing.T) {
	repoRoot := t.TempDir()
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
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	initGitRepoWithCommit(t, repoRoot)

	server := NewServer(config.Config{
		APIAddr:        ":0",
		AllowedOrigins: "http://localhost:4300",
		RepoRoot:       repoRoot,
		DBPath:         filepath.Join(repoRoot, "data", "ralleh-flow.db"),
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader([]byte(`{"workflowId":"feature-development","variables":{"feature_name":"Flow Polish","target_repo":"ralleh-flow"}}`)))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	server.Handler.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", createRes.Code, createRes.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created run: %v", err)
	}
	runID, _ := created["id"].(string)

	firstAdvanceReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/advance", runID), nil)
	firstAdvanceRes := httptest.NewRecorder()
	server.Handler.ServeHTTP(firstAdvanceRes, firstAdvanceReq)
	if firstAdvanceRes.Code != http.StatusOK {
		t.Fatalf("expected first advance 200, got %d (%s)", firstAdvanceRes.Code, firstAdvanceRes.Body.String())
	}

	secondAdvanceReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/advance", runID), nil)
	secondAdvanceRes := httptest.NewRecorder()
	server.Handler.ServeHTTP(secondAdvanceRes, secondAdvanceReq)
	if secondAdvanceRes.Code != http.StatusConflict {
		t.Fatalf("expected second advance 409, got %d (%s)", secondAdvanceRes.Code, secondAdvanceRes.Body.String())
	}
	if !strings.Contains(secondAdvanceRes.Body.String(), "run state transition conflict") {
		t.Fatalf("expected run state conflict, got %s", secondAdvanceRes.Body.String())
	}
}

func TestCreateRunWorkflowMissing(t *testing.T) {
	repoRoot := t.TempDir()
	server := NewServer(config.Config{
		APIAddr:        ":0",
		AllowedOrigins: "http://localhost:4300",
		RepoRoot:       repoRoot,
		DBPath:         filepath.Join(repoRoot, "data", "ralleh-flow.db"),
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader([]byte(`{"workflowId":"missing"}`)))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	server.Handler.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (%s)", res.Code, res.Body.String())
	}
}

func TestCreateRunRequiresGitExecutionRepo(t *testing.T) {
	repoRoot := t.TempDir()
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
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	server := NewServer(config.Config{
		APIAddr:        ":0",
		AllowedOrigins: "http://localhost:4300",
		RepoRoot:       repoRoot,
		DBPath:         filepath.Join(repoRoot, "data", "ralleh-flow.db"),
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader([]byte(`{"workflowId":"feature-development","variables":{"feature_name":"Flow Polish","target_repo":"ralleh-flow"}}`)))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	server.Handler.ServeHTTP(res, req)

	if res.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d (%s)", res.Code, res.Body.String())
	}

	if !strings.Contains(res.Body.String(), "execution repository is not ready") {
		t.Fatalf("expected git readiness error, got %s", res.Body.String())
	}
}

func TestGetWorkflowDetail(t *testing.T) {
	repoRoot := t.TempDir()
	workflowPath := filepath.Join(repoRoot, "workflows", "examples", "feature-development", "workflow.yaml")
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	workflowYAML := `metadata:
  id: feature-development
  name: Feature Development
  version: 0.1.0
  description: Build a feature end-to-end.
variables:
  - key: feature_name
    type: string
    required: true
    description: Human-readable feature name.
  - key: target_repo
    type: repository
    required: true
    description: Repository target for execution.
steps:
  - id: research
    kind: agent_task
  - id: implement
    kind: agent_task
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	server := NewServer(config.Config{APIAddr: ":0", AllowedOrigins: "http://localhost:4300", RepoRoot: repoRoot})

	res := httptest.NewRecorder()
	server.Handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/v1/workflows/feature-development", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", res.Code, res.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if got, _ := body["id"].(string); got != "feature-development" {
		t.Fatalf("expected id feature-development, got %#v", body["id"])
	}

	variables, ok := body["variables"].([]any)
	if !ok || len(variables) != 2 {
		t.Fatalf("expected 2 variables, got %#v", body["variables"])
	}

	steps, ok := body["steps"].([]any)
	if !ok || len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %#v", body["steps"])
	}
}

func TestGetWorkflowDetailNotFound(t *testing.T) {
	repoRoot := t.TempDir()
	server := NewServer(config.Config{APIAddr: ":0", AllowedOrigins: "http://localhost:4300", RepoRoot: repoRoot})

	res := httptest.NewRecorder()
	server.Handler.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/v1/workflows/missing", nil))

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (%s)", res.Code, res.Body.String())
	}
}

func TestValidateWorkflow(t *testing.T) {
	repoRoot := t.TempDir()
	workflowPath := filepath.Join(repoRoot, "workflows", "examples", "feature-development", "workflow.yaml")
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	workflowYAML := `metadata:
  id: feature-development
  name: Feature Development
steps:
  - id: research
    kind: agent_task
  - id: review
    kind: human_approval
    approverPolicy: reviewer
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	server := NewServer(config.Config{APIAddr: ":0", AllowedOrigins: "http://localhost:4300", RepoRoot: repoRoot})

	res := httptest.NewRecorder()
	server.Handler.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/v1/workflows/feature-development/validate", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", res.Code, res.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if valid, _ := body["valid"].(bool); !valid {
		t.Fatalf("expected valid=true, got %#v", body)
	}

	errors, ok := body["errors"].([]any)
	if !ok || len(errors) != 0 {
		t.Fatalf("expected no errors, got %#v", body["errors"])
	}
}

func TestValidateWorkflowReportsErrors(t *testing.T) {
	repoRoot := t.TempDir()
	workflowPath := filepath.Join(repoRoot, "workflows", "examples", "broken", "workflow.yaml")
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	workflowYAML := `metadata:
  id: broken
steps:
  - id: review
    kind: human_approval
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	server := NewServer(config.Config{APIAddr: ":0", AllowedOrigins: "http://localhost:4300", RepoRoot: repoRoot})

	res := httptest.NewRecorder()
	server.Handler.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/v1/workflows/broken/validate", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", res.Code, res.Body.String())
	}

	if !strings.Contains(res.Body.String(), "missing_approver_policy") {
		t.Fatalf("expected approver policy validation error, got %s", res.Body.String())
	}
}

func TestDryRunWorkflow(t *testing.T) {
	repoRoot := t.TempDir()
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
  - id: review
    kind: human_approval
    approverPolicy: reviewer
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	server := NewServer(config.Config{APIAddr: ":0", AllowedOrigins: "http://localhost:4300", RepoRoot: repoRoot})

	body := bytes.NewReader([]byte(`{"variables":{"feature_name":"Flow polish"}}`))
	res := httptest.NewRecorder()
	server.Handler.ServeHTTP(res, httptest.NewRequest(http.MethodPost, "/v1/workflows/feature-development/dry-run", body))

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", res.Code, res.Body.String())
	}

	if !strings.Contains(res.Body.String(), "target_repo") {
		t.Fatalf("expected missing variable in dry-run output, got %s", res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "execution_not_available") {
		t.Fatalf("expected execution warning in dry-run output, got %s", res.Body.String())
	}
}

func TestCreateRunRequiresWorkflowVariables(t *testing.T) {
	repoRoot := t.TempDir()
	workflowPath := filepath.Join(repoRoot, "workflows", "examples", "feature-development", "workflow.yaml")
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o755); err != nil {
		t.Fatalf("mkdir workflow dir: %v", err)
	}

	workflowYAML := `metadata:
  id: feature-development
  name: Feature Development
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
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	server := NewServer(config.Config{
		APIAddr:        ":0",
		AllowedOrigins: "http://localhost:4300",
		RepoRoot:       repoRoot,
		DBPath:         filepath.Join(repoRoot, "data", "ralleh-flow.db"),
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader([]byte(`{"workflowId":"feature-development","variables":{"feature_name":"Flow Polish"}}`)))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	server.Handler.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", res.Code, res.Body.String())
	}

	if !strings.Contains(res.Body.String(), "missing required workflow variables") {
		t.Fatalf("expected required variables error, got %s", res.Body.String())
	}
}

func TestCreateRunWithWorkflowVariables(t *testing.T) {
	repoRoot := t.TempDir()
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
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	initGitRepoWithCommit(t, repoRoot)

	server := NewServer(config.Config{
		APIAddr:        ":0",
		AllowedOrigins: "http://localhost:4300",
		RepoRoot:       repoRoot,
		DBPath:         filepath.Join(repoRoot, "data", "ralleh-flow.db"),
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader([]byte(`{"workflowId":"feature-development","variables":{"feature_name":"Flow Polish","target_repo":"ralleh-flow"}}`)))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	server.Handler.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", res.Code, res.Body.String())
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

func TestDispatchStepUsesConfiguredOpenClawHookDispatcher(t *testing.T) {
	repoRoot := t.TempDir()
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
    agent: carmack
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	initGitRepoWithCommit(t, repoRoot)

	hookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer hook-secret" {
			t.Fatalf("expected bearer auth, got %q", got)
		}
		writeTestJSON(t, w, http.StatusOK, map[string]any{"ok": true, "runId": "hook-run-456"})
	}))
	defer hookServer.Close()

	app := NewApp(config.Config{
		APIAddr:                     ":0",
		AllowedOrigins:              "http://localhost:4300",
		RepoRoot:                    repoRoot,
		DBPath:                      filepath.Join(repoRoot, "data", "ralleh-flow.db"),
		OpenClawHookURL:             hookServer.URL,
		OpenClawHookToken:           "hook-secret",
		OpenClawSessionKeyPrefix:    "hook:ralleh-flow",
		OpenClawAgentTimeoutSeconds: 30,
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader([]byte(`{"workflowId":"feature-development","variables":{"feature_name":"Flow Polish","target_repo":"ralleh-flow"}}`)))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", createRes.Code, createRes.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created run: %v", err)
	}
	runID, _ := created["id"].(string)

	advanceReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/advance", runID), nil)
	advanceRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(advanceRes, advanceReq)
	if advanceRes.Code != http.StatusOK {
		t.Fatalf("expected advance 200, got %d (%s)", advanceRes.Code, advanceRes.Body.String())
	}

	dispatchReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/dispatch-step", runID), bytes.NewReader([]byte(`{"note":"hook dispatch"}`)))
	dispatchReq.Header.Set("Content-Type", "application/json")
	dispatchRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(dispatchRes, dispatchReq)
	if dispatchRes.Code != http.StatusOK {
		t.Fatalf("expected dispatch 200, got %d (%s)", dispatchRes.Code, dispatchRes.Body.String())
	}
	if !strings.Contains(dispatchRes.Body.String(), fmt.Sprintf(`"sessionId":"hook:ralleh-flow:%s:research"`, runID)) {
		t.Fatalf("expected configured hook session binding, got %s", dispatchRes.Body.String())
	}
	if !strings.Contains(dispatchRes.Body.String(), `"upstreamRunId":"hook-run-456"`) {
		t.Fatalf("expected upstream run id in response handoff, got %s", dispatchRes.Body.String())
	}
	handoffDoc := filepath.Join(repoRoot, "runtime", "runs", runID, "workspace", "HANDOFF.md")
	docContents, err := os.ReadFile(handoffDoc)
	if err != nil {
		t.Fatalf("read handoff document: %v", err)
	}
	if !strings.Contains(string(docContents), "# HANDOFF") || !strings.Contains(string(docContents), "feature-development") {
		t.Fatalf("expected handoff doc contents, got %s", string(docContents))
	}
	handoffPath := filepath.Join(repoRoot, "runtime", "runs", runID, "handoffs", "research-dispatched.json")
	contents, err := os.ReadFile(handoffPath)
	if err != nil {
		t.Fatalf("read dispatched handoff artifact: %v", err)
	}
	if !strings.Contains(string(contents), `hook-run-456`) {
		t.Fatalf("expected hook run id in artifact note, got %s", string(contents))
	}
}

func TestDispatchStepReturnsBadGatewayWhenOpenClawHookFails(t *testing.T) {
	repoRoot := t.TempDir()
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
    agent: carmack
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	initGitRepoWithCommit(t, repoRoot)

	hookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(t, w, http.StatusBadRequest, map[string]any{"ok": false, "error": "sessionKey is disabled for externally supplied hook payload values; set hooks.allowRequestSessionKey=true to enable"})
	}))
	defer hookServer.Close()

	app := NewApp(config.Config{
		APIAddr:                  ":0",
		AllowedOrigins:           "http://localhost:4300",
		RepoRoot:                 repoRoot,
		DBPath:                   filepath.Join(repoRoot, "data", "ralleh-flow.db"),
		OpenClawHookURL:          hookServer.URL,
		OpenClawHookToken:        "hook-secret",
		OpenClawSessionKeyPrefix: "hook:ralleh-flow",
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader([]byte(`{"workflowId":"feature-development","variables":{"feature_name":"Flow Polish","target_repo":"ralleh-flow"}}`)))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", createRes.Code, createRes.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created run: %v", err)
	}
	runID, _ := created["id"].(string)

	advanceReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/advance", runID), nil)
	advanceRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(advanceRes, advanceReq)
	if advanceRes.Code != http.StatusOK {
		t.Fatalf("expected advance 200, got %d (%s)", advanceRes.Code, advanceRes.Body.String())
	}

	dispatchReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/dispatch-step", runID), bytes.NewReader([]byte(`{"note":"hook dispatch"}`)))
	dispatchReq.Header.Set("Content-Type", "application/json")
	dispatchRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(dispatchRes, dispatchReq)
	if dispatchRes.Code != http.StatusBadGateway {
		t.Fatalf("expected dispatch 502, got %d (%s)", dispatchRes.Code, dispatchRes.Body.String())
	}
	if !strings.Contains(dispatchRes.Body.String(), `hooks.allowRequestSessionKey=true`) {
		t.Fatalf("expected explicit hook policy failure, got %s", dispatchRes.Body.String())
	}
}

func writeTestJSON(t *testing.T, w http.ResponseWriter, status int, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("encode JSON: %v", err)
	}
}

func TestCompleteStepQueuesNextStepInsteadOfCompletingRun(t *testing.T) {
	repoRoot := t.TempDir()
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
    agent: carmack
  - id: implement
    kind: agent_task
    agent: picasso
  - id: review
    kind: human_approval
    approverPolicy: reviewer
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	initGitRepoWithCommit(t, repoRoot)

	app := NewApp(config.Config{
		APIAddr:        ":0",
		AllowedOrigins: "http://localhost:4300",
		RepoRoot:       repoRoot,
		DBPath:         filepath.Join(repoRoot, "data", "ralleh-flow.db"),
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader([]byte(`{"workflowId":"feature-development","variables":{"feature_name":"Flow Polish","target_repo":"ralleh-flow"}}`)))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", createRes.Code, createRes.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created run: %v", err)
	}
	runID, _ := created["id"].(string)

	advanceReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/advance", runID), nil)
	advanceRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(advanceRes, advanceReq)
	if advanceRes.Code != http.StatusOK {
		t.Fatalf("expected advance 200, got %d (%s)", advanceRes.Code, advanceRes.Body.String())
	}

	dispatchReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/dispatch-step", runID), bytes.NewReader([]byte(`{"sessionId":"session:abc123","note":"dispatch bound"}`)))
	dispatchReq.Header.Set("Content-Type", "application/json")
	dispatchRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(dispatchRes, dispatchReq)
	if dispatchRes.Code != http.StatusOK {
		t.Fatalf("expected dispatch 200, got %d (%s)", dispatchRes.Code, dispatchRes.Body.String())
	}

	completeReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/complete-step", runID), bytes.NewReader([]byte(`{"summary":"done"}`)))
	completeReq.Header.Set("Content-Type", "application/json")
	completeRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(completeRes, completeReq)
	if completeRes.Code != http.StatusOK {
		t.Fatalf("expected complete 200, got %d (%s)", completeRes.Code, completeRes.Body.String())
	}
	if !strings.Contains(completeRes.Body.String(), `"status":"pending"`) {
		t.Fatalf("expected pending run status after first step completion, got %s", completeRes.Body.String())
	}
	if !strings.Contains(completeRes.Body.String(), `"currentStep":"implement"`) {
		t.Fatalf("expected currentStep implement after first step completion, got %s", completeRes.Body.String())
	}
	if !strings.Contains(completeRes.Body.String(), `"type":"run.pending"`) {
		t.Fatalf("expected run.pending event in response, got %s", completeRes.Body.String())
	}
}

func TestCompleteStepRequestsApprovalWhenNextStepIsHumanGate(t *testing.T) {
	repoRoot := t.TempDir()
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
    agent: carmack
  - id: implement
    kind: agent_task
    agent: picasso
  - id: review
    kind: human_approval
    approverPolicy: reviewer
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	initGitRepoWithCommit(t, repoRoot)

	app := NewApp(config.Config{
		APIAddr:        ":0",
		AllowedOrigins: "http://localhost:4300",
		RepoRoot:       repoRoot,
		DBPath:         filepath.Join(repoRoot, "data", "ralleh-flow.db"),
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader([]byte(`{"workflowId":"feature-development","variables":{"feature_name":"Flow Polish","target_repo":"ralleh-flow"}}`)))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", createRes.Code, createRes.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created run: %v", err)
	}
	runID, _ := created["id"].(string)

	for idx, sessionID := range []string{"session:research", "session:implement"} {
		advanceReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/advance", runID), nil)
		advanceRes := httptest.NewRecorder()
		app.Server.Handler.ServeHTTP(advanceRes, advanceReq)
		if advanceRes.Code != http.StatusOK {
			t.Fatalf("expected advance %d to return 200, got %d (%s)", idx+1, advanceRes.Code, advanceRes.Body.String())
		}

		dispatchReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/dispatch-step", runID), bytes.NewReader([]byte(fmt.Sprintf(`{"sessionId":"%s"}`, sessionID))))
		dispatchReq.Header.Set("Content-Type", "application/json")
		dispatchRes := httptest.NewRecorder()
		app.Server.Handler.ServeHTTP(dispatchRes, dispatchReq)
		if dispatchRes.Code != http.StatusOK {
			t.Fatalf("expected dispatch %d to return 200, got %d (%s)", idx+1, dispatchRes.Code, dispatchRes.Body.String())
		}

		completeReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/complete-step", runID), bytes.NewReader([]byte(`{"summary":"done"}`)))
		completeReq.Header.Set("Content-Type", "application/json")
		completeRes := httptest.NewRecorder()
		app.Server.Handler.ServeHTTP(completeRes, completeReq)
		if completeRes.Code != http.StatusOK {
			t.Fatalf("expected complete %d to return 200, got %d (%s)", idx+1, completeRes.Code, completeRes.Body.String())
		}
		if idx == 1 {
			if !strings.Contains(completeRes.Body.String(), `"status":"waiting_for_approval"`) {
				t.Fatalf("expected waiting_for_approval after implement completion, got %s", completeRes.Body.String())
			}
			if !strings.Contains(completeRes.Body.String(), `"currentStep":"review"`) {
				t.Fatalf("expected currentStep review after implement completion, got %s", completeRes.Body.String())
			}
			if !strings.Contains(completeRes.Body.String(), `"type":"approval.requested"`) {
				t.Fatalf("expected approval.requested event after implement completion, got %s", completeRes.Body.String())
			}
		}
	}
}

func TestCompleteStepRequiresDispatchedProofForAgentTask(t *testing.T) {
	repoRoot := t.TempDir()
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
    agent: carmack
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	initGitRepoWithCommit(t, repoRoot)

	app := NewApp(config.Config{
		APIAddr:        ":0",
		AllowedOrigins: "http://localhost:4300",
		RepoRoot:       repoRoot,
		DBPath:         filepath.Join(repoRoot, "data", "ralleh-flow.db"),
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader([]byte(`{"workflowId":"feature-development","variables":{"feature_name":"Flow Polish","target_repo":"ralleh-flow"}}`)))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", createRes.Code, createRes.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created run: %v", err)
	}
	runID, _ := created["id"].(string)

	advanceReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/advance", runID), nil)
	advanceRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(advanceRes, advanceReq)
	if advanceRes.Code != http.StatusOK {
		t.Fatalf("expected advance 200, got %d (%s)", advanceRes.Code, advanceRes.Body.String())
	}

	completeReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/complete-step", runID), bytes.NewReader([]byte(`{"summary":"done"}`)))
	completeReq.Header.Set("Content-Type", "application/json")
	completeRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(completeRes, completeReq)
	if completeRes.Code != http.StatusConflict {
		t.Fatalf("expected complete 409, got %d (%s)", completeRes.Code, completeRes.Body.String())
	}
	if !strings.Contains(completeRes.Body.String(), "requires dispatched handoff proof") {
		t.Fatalf("expected dispatched-proof detail, got %s", completeRes.Body.String())
	}
}

func TestFailStepRequiresDispatchedProofForAgentTask(t *testing.T) {
	repoRoot := t.TempDir()
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
    agent: carmack
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	initGitRepoWithCommit(t, repoRoot)

	app := NewApp(config.Config{
		APIAddr:        ":0",
		AllowedOrigins: "http://localhost:4300",
		RepoRoot:       repoRoot,
		DBPath:         filepath.Join(repoRoot, "data", "ralleh-flow.db"),
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader([]byte(`{"workflowId":"feature-development","variables":{"feature_name":"Flow Polish","target_repo":"ralleh-flow"}}`)))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", createRes.Code, createRes.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created run: %v", err)
	}
	runID, _ := created["id"].(string)

	advanceReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/advance", runID), nil)
	advanceRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(advanceRes, advanceReq)
	if advanceRes.Code != http.StatusOK {
		t.Fatalf("expected advance 200, got %d (%s)", advanceRes.Code, advanceRes.Body.String())
	}

	failReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/fail-step", runID), bytes.NewReader([]byte(`{"reason":"boom"}`)))
	failReq.Header.Set("Content-Type", "application/json")
	failRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(failRes, failReq)
	if failRes.Code != http.StatusConflict {
		t.Fatalf("expected fail 409, got %d (%s)", failRes.Code, failRes.Body.String())
	}
	if !strings.Contains(failRes.Body.String(), "requires dispatched handoff proof") {
		t.Fatalf("expected dispatched-proof detail, got %s", failRes.Body.String())
	}
}

func TestCompleteStepRejectsMismatchedCallbackSession(t *testing.T) {
	repoRoot := t.TempDir()
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
    agent: carmack
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	initGitRepoWithCommit(t, repoRoot)

	hookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(t, w, http.StatusOK, map[string]any{"ok": true, "runId": "hook-run-456"})
	}))
	defer hookServer.Close()

	app := NewApp(config.Config{
		APIAddr:                  ":0",
		AllowedOrigins:           "http://localhost:4300",
		RepoRoot:                 repoRoot,
		DBPath:                   filepath.Join(repoRoot, "data", "ralleh-flow.db"),
		OpenClawHookURL:          hookServer.URL,
		OpenClawHookToken:        "hook-secret",
		OpenClawSessionKeyPrefix: "hook:ralleh-flow",
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader([]byte(`{"workflowId":"feature-development","variables":{"feature_name":"Flow Polish","target_repo":"ralleh-flow"}}`)))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", createRes.Code, createRes.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created run: %v", err)
	}
	runID, _ := created["id"].(string)

	advanceReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/advance", runID), nil)
	advanceRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(advanceRes, advanceReq)
	if advanceRes.Code != http.StatusOK {
		t.Fatalf("expected advance 200, got %d (%s)", advanceRes.Code, advanceRes.Body.String())
	}

	dispatchReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/dispatch-step", runID), nil)
	dispatchRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(dispatchRes, dispatchReq)
	if dispatchRes.Code != http.StatusOK {
		t.Fatalf("expected dispatch 200, got %d (%s)", dispatchRes.Code, dispatchRes.Body.String())
	}

	completeReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/complete-step", runID), bytes.NewReader([]byte(`{"summary":"done","sessionId":"session:wrong","upstreamRunId":"hook-run-456"}`)))
	completeReq.Header.Set("Content-Type", "application/json")
	completeRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(completeRes, completeReq)
	if completeRes.Code != http.StatusConflict {
		t.Fatalf("expected complete 409, got %d (%s)", completeRes.Code, completeRes.Body.String())
	}
	if !strings.Contains(completeRes.Body.String(), "callback sessionId does not match dispatched handoff") {
		t.Fatalf("expected mismatch detail, got %s", completeRes.Body.String())
	}
}

func TestFailStepRejectsMismatchedCallbackUpstreamRunID(t *testing.T) {
	repoRoot := t.TempDir()
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
    agent: carmack
`
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0o644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	initGitRepoWithCommit(t, repoRoot)

	hookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(t, w, http.StatusOK, map[string]any{"ok": true, "runId": "hook-run-456"})
	}))
	defer hookServer.Close()

	app := NewApp(config.Config{
		APIAddr:                  ":0",
		AllowedOrigins:           "http://localhost:4300",
		RepoRoot:                 repoRoot,
		DBPath:                   filepath.Join(repoRoot, "data", "ralleh-flow.db"),
		OpenClawHookURL:          hookServer.URL,
		OpenClawHookToken:        "hook-secret",
		OpenClawSessionKeyPrefix: "hook:ralleh-flow",
	})

	createReq := httptest.NewRequest(http.MethodPost, "/v1/runs", bytes.NewReader([]byte(`{"workflowId":"feature-development","variables":{"feature_name":"Flow Polish","target_repo":"ralleh-flow"}}`)))
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", createRes.Code, createRes.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(createRes.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created run: %v", err)
	}
	runID, _ := created["id"].(string)

	advanceReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/advance", runID), nil)
	advanceRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(advanceRes, advanceReq)
	if advanceRes.Code != http.StatusOK {
		t.Fatalf("expected advance 200, got %d (%s)", advanceRes.Code, advanceRes.Body.String())
	}

	dispatchReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/dispatch-step", runID), nil)
	dispatchRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(dispatchRes, dispatchReq)
	if dispatchRes.Code != http.StatusOK {
		t.Fatalf("expected dispatch 200, got %d (%s)", dispatchRes.Code, dispatchRes.Body.String())
	}

	failReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/v1/runs/%s/fail-step", runID), bytes.NewReader([]byte(`{"reason":"boom","sessionId":"hook:ralleh-flow:`+runID+`:research","upstreamRunId":"hook-run-wrong"}`)))
	failReq.Header.Set("Content-Type", "application/json")
	failRes := httptest.NewRecorder()
	app.Server.Handler.ServeHTTP(failRes, failReq)
	if failRes.Code != http.StatusConflict {
		t.Fatalf("expected fail 409, got %d (%s)", failRes.Code, failRes.Body.String())
	}
	if !strings.Contains(failRes.Body.String(), "callback upstreamRunId does not match dispatched handoff") {
		t.Fatalf("expected mismatch detail, got %s", failRes.Body.String())
	}
}
