# Ralleh Flow Code/Architecture/Security Review — 2026-06

Scope: active branch `isbe/approval-recovery-typecheck` through commit `132b08d`.

## 1) Discovery Summary

### Repository structure (current)

- Backend: `apps/api` (Go)
  - `cmd/ralleh-flow-api` entrypoint
  - `internal/api` HTTP handlers + tests
  - `internal/service` run/workflow/coordination/orchestration domain logic
  - `internal/repository` + `internal/db` persistence support
  - `internal/workflows` workflow package loading
- Frontend: `apps/web` (Nuxt)
  - `pages/` operator surfaces (dashboard, approvals, run detail, workflows)
  - `composables/useFlowApi.ts` API client contract
  - Playwright E2E in `tests/e2e`

### Verification gates observed recently

- `make api-test` ✅
- `pnpm test:ui` ✅ (16/16)
- `pnpm build` ✅ (Nuxt build; one transient cache ENOENT reproduced once and passed on rerun)

## 2) Architecture Assessment

### Strengths

- Clear monorepo separation between runtime API and operator UI.
- Run lifecycle and transitions are explicit in backend service layer.
- Git-oriented run isolation (branch/worktree/runtime dir) is in place.
- Approval lifecycle is first-class (persisted approvals + decision endpoints).
- Redis mode and in-memory fallback keep local/dev path simple while preserving production posture.

### Current boundary (important)

- The backend currently implements **stateful orchestration transitions** and callback-driven continuation.
- It does **not yet** implement fully autonomous step-kind workers as default execution runtime.
- This boundary is now explicitly documented and should remain explicit until worker execution lands.

## 3) Naming & Consistency Review

### Aligned

- Project and package naming consistently use `ralleh-flow` identity.
- API env keys are consistently prefixed (`RALLEH_FLOW_*`).
- Branch/worktree naming conventions are coherent with run identity semantics.

### Remaining consistency tasks

- Keep placeholder language in UI strictly aligned with actual backend behavior to avoid overpromising execution depth.
- Add a single naming glossary doc section for run/step/handoff/approval terms to avoid drift as integrations expand.

## 4) Code Quality Observations

### Go backend

- Style is mostly disciplined and auditable.
- Positive: explicit interfaces around coordination/orchestration/event dispatch modes.
- Improvement landed: stricter JSON request handling in `server.go`:
  - body-size cap (1 MiB)
  - unknown-field rejection
  - single-object JSON enforcement
  - explicit 413 path for oversize bodies
- Regression tests added for these controls in `server_test.go`.

### Frontend (Nuxt)

- Operator-focused page model and API contract are clear.
- E2E coverage includes happy path, empty state, and failure recovery surfaces.
- Improvement opportunity: move some run-detail computed logic into dedicated composables to reduce page-level complexity and aid maintainability.

## 5) Security & Operational Review

### Implemented hardening

- Request-body attack surface reduced at API layer (size + schema strictness).
- Readiness endpoint behavior indicates coordination mode and actual backend status.

### High-priority next controls

1. Add authn/authz middleware for mutation endpoints (`POST /v1/*`).
2. Add rate limiting for write/mutation APIs.
3. Add structured security audit logs for approvals and step mutations.
4. Add run retention/cleanup policy for worktrees/runtime directories.

## 6) Documentation vs Implementation Status

### Completed

- `docs/development-plan.md` updated to separate implemented behavior from active gaps.
- `docs/architecture.md` updated to clarify current execution boundary.
- `docs/README.md` rule (“docs must stay in sync with code”) is being enforced in practice on latest pass.

### Ongoing requirement

- Every runtime/contract change must update docs in the same PR and reference validating tests/scripts.

## 7) Platform Integration Points (Ralleh Ecosystem)

## 7.1 `ralleh-tasks` integration (task state + execution ledger)

Proposed contract:

- Flow publishes task lifecycle events:
  - `flow.run.created`
  - `flow.step.started`
  - `flow.step.completed`
  - `flow.step.failed`
  - `flow.approval.awaiting`
  - `flow.run.completed`
- Payload fields:
  - `runId`, `workflowId`, `stepId?`, `status`, `startedAt`, `finishedAt?`, `correlationId`, `source="ralleh-flow"`

Consumption model:

- `ralleh-tasks` stores and indexes external task progress while preserving Flow as orchestration authority.

## 7.2 `ralleh-keys` integration (secret resolution)

Proposed contract:

- Flow does not store raw secrets in run records.
- Workflow step references secret handles (e.g. `secrets.OPENAI_API_KEY`).
- Resolver interface:
  - `ResolveSecret(ctx, keyRef, runContext) -> value|error`
- `ralleh-keys` handles policy, access control, and audit trail.

## 7.3 Engram integration (memory/context retrieval)

Proposed contract:

- Optional read-only context injection into runs:
  - `GetContext(query, scope, maxTokens) -> contextBundle`
- Flow stores only retrieval metadata + references in timeline/artifact manifests, not full memory corpus blobs.
- Correlate by `runId` + `stepId` + `correlationId` for reproducible traceability.

## 8) Improvement Plan (priority order)

1. Naming/contract glossary + enforce consistent vocabulary in API/UI/docs.
2. Backend boundary cleanup:
   - isolate HTTP DTO decode/validate paths from service input models where still coupled.
3. Domain boundary strengthening:
   - explicit interfaces for secret resolution, task event publisher, context retriever.
4. Documentation hygiene:
   - keep architecture/development-plan truthy per release slice.
5. Security/operations:
   - authn/authz + rate limit + audit logging + retention lifecycle.

## 9) Verdict

Ralleh Flow is now a credible, disciplined MVP foundation with real orchestration state and strong operator visibility. It is no longer “pure scaffold.”

To align fully with long-term Ralleh platform standards, the next milestones should focus on:
- explicit cross-service contracts (`ralleh-tasks`, `ralleh-keys`, Engram),
- stronger API security controls,
- and keeping docs continuously truthful as execution depth expands.
