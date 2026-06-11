# Ralleh Flow Development Plan

Version: draft 2

## 1. Product framing

Ralleh Flow should be built as a **Git-native orchestration control room** for AI-assisted work.

The charter is strong, but to make the product buildable we need a sharper MVP boundary:

### What MVP must prove

- a workflow package can be loaded from git-tracked YAML
- a run can be created with typed variables and assets
- a run gets an isolated branch + worktree + workspace
- steps can execute sequentially with checkpoint persistence
- steps can pause for approval and resume safely
- OpenClaw agents can be assigned work with explicit context handoffs
- operators can inspect timeline, logs, artifacts, snapshots, and costs in a browser UI under `/flow`

### What MVP should not try to finish yet

- multi-tenant org model
- complex visual builder with drag/drop graph editing
- marketplace/package registry
- Kubernetes-native runtime
- advanced schedule engine
- fully generic external integrations

## 1.1 Engineering discipline

Ralleh Flow should be built with a **test-first, clean-code discipline** from the beginning.

### TDD process
- every behavior change should begin with a failing test or fixture that proves the requirement
- implementation should be the minimum change needed to make the test pass
- refactoring is required after green to keep the design simple, explicit, and maintainable
- bug fixes must start with a regression test that fails before the fix
- if a slice cannot be tested automatically yet, the gap must be documented and closed in the next slice

### Testing expectations
- no backend feature is done without unit tests for core logic and integration coverage for persistence/runtime edges
- no UI feature is done without component/state coverage or at minimum a reproducible page-level smoke check
- fixtures should be treated as first-class assets for workflow validation, dry runs, approvals, and recovery scenarios
- CI should run format, lint, unit, and integration checks on every meaningful slice

### Clean-code expectations
- prefer small services with explicit contracts over large mixed-responsibility handlers
- keep state transitions centralized and deterministic
- favor idempotent operations and narrow interfaces over clever implicit behavior
- record invariants in code, tests, and docs at the moment they are introduced
- concurrency-sensitive code must be written for correctness first, then optimized with evidence

---

## 2. Recommended architecture

## 2.1 Monorepo layout

```text
ralleh-flow/
  apps/
    web/               # Nuxt UI mounted under /flow
    api/               # Go HTTP API + workflow runtime
  packages/
    contracts/         # shared JSON schemas, OpenAPI, event definitions
    ui/                # shared UI tokens/components
    fixtures/          # sample workflows/assets for dev/test
  deploy/
    docker-compose.yml
    caddy/
    systemd/
  docs/
  workflows/
    examples/
```

## 2.2 Runtime components

### Web UI
- Nuxt + TypeScript + Tailwind
- reverse-proxy mounting for `/flow`
- operator dashboard, workflow catalog, run explorer, approval inbox
- control-room information architecture similar to Ralleh Voice

### API / runtime
- Go service
- Chi router
- Zerolog
- Viper-style config discipline
- SQLite metadata store
- Redis Streams event bus
- Git/worktree orchestration layer
- OpenClaw dispatch adapter

### Persistence split

#### Git
Source of truth for:
- workflow packages
- prompts
- agent handoff templates
- workflow assets intended to be versioned
- generated artifacts selected for promotion

#### SQLite
Metadata for:
- workflow registry
- run records
- step records
- asset metadata
- approval records
- audit index
- cost summaries

#### Redis
Disposable runtime for:
- queued work
- step leases
- stream events
- cancellation signals
- active locks
- live dashboard feed

---

## 3. Key improvements to the charter\n
### 3.1 Split workflow definition from execution policy
A workflow YAML should define logical flow. A separate execution policy block or file should define:
- concurrency limits
- retry policy
- timeout policy
- approval requirements
- allowed agents
- allowed tool classes

Why: workflow authors should not accidentally encode unsafe operational policy into every template copy.

### 3.2 Add explicit step kinds
Do not treat every step as "run an agent". MVP should support these step kinds:
- `agent_task`
- `human_approval`
- `asset_transform`
- `shell_command` (guarded, internal only)
- `git_checkpoint`
- `artifact_publish`
- `noop`

Why: this keeps the engine honest and reduces special-case logic.

### 3.3 Add immutable snapshot manifests
Each checkpoint should write a small structured manifest:
- snapshot id
- workflow version
- run id
- git commit sha
- asset versions
- prompt versions
- step outputs
- cost summary to date

Why: replay/debug becomes reliable.

### 3.4 Separate event stream from audit log
Redis events are transient transport. Audit records should be normalized and persisted in SQLite and optionally exported to Git artifacts.

Why: events are not the audit system.

### 3.5 Add capability contracts for agents
Each agent should declare:
- role
- supported task classes
- required context inputs
- output contract
- escalation rules

Why: routing becomes deterministic instead of prompt magic.

### 3.6 Define artifact promotion policy now
Promotion should require explicit provenance metadata:
- produced by run id
- produced by step id
- reviewed by
- approved at
- source asset lineage

Why: prevents "mystery file becomes trusted input" drift.

### 3.7 Treat secrets as references only
Workflow variables of type `secret` should store references, never raw values. Actual resolution should happen only at execution boundary.

### 3.8 Concurrency and isolation standards

These are non-negotiable runtime rules for Ralleh Flow.

#### Execution isolation
- each workflow run MUST execute in its own Git worktree
- each workflow run MUST use a unique branch name
- worktrees MUST never be shared between runs
- temporary execution directories MUST be namespaced by run ID
- if the repo is not a valid git execution root with a committed `HEAD`, run creation MUST fail explicitly instead of silently degrading to plain directories

#### Distributed coordination
- Redis distributed locks MUST be used for shared repository operations
- workflow state transitions MUST be atomic and idempotent
- only one orchestrator worker MAY process a workflow run at a time
- Redis Streams MUST be used for ordered event coordination
- readiness MUST fail when configured Redis coordination/event infrastructure is unavailable

#### Immutable execution context
- asset versions MUST be immutable for the lifetime of a workflow run
- workflow definitions MUST be version-pinned at run start
- prompt versions MUST be recorded and immutable per run
- agent versions SHOULD be recorded for reproducibility

#### Parallelism rules
- parallel execution MUST be explicitly declared in workflow definitions
- dependent steps MUST declare `depends_on` relationships
- parallel tasks MUST NOT share mutable workspace state
- agent concurrency limits MUST be configurable and enforced by the scheduler

#### Reliability
- steps MUST be idempotent or provide compensating actions
- checkpoints MUST allow resuming from the last successful step
- snapshots MUST capture execution state before major transitions
- crashes or worker restarts MUST NOT corrupt workflow state

---

## 4. UX plan for `/flow`

The UI should borrow the structural discipline of **Ralleh Voice** while being implemented with the standard Ralleh Nuxt/Vue stack.

## 4.1 IA model

### Desktop layout
Use a 3-panel control-room layout:
- **Left rail** — workflow/run context, filters, run actions, approvals summary
- **Center stage** — active run timeline and current step state
- **Right rail** — diagnostics, costs, git state, event feed

### Mobile layout
Use stacked surfaces:
1. header/status strip
2. active run card
3. primary actions
4. timeline/logs
5. tabbed details for assets/approvals/diagnostics

## 4.2 Primary pages\n
### `/flow`
Dashboard:
- active runs
- queued runs
- blocked approvals
- recent failures
- agent health
- event throughput

### `/flow/workflows`
Workflow catalog:
- workflow cards
- tags / owner / version
- last run status
- create run action

### `/flow/workflows/[workflowId]`
Workflow detail:
- YAML definition summary
- variables contract
- asset requirements
- step graph
- test / dry-run result
- recent runs

### `/flow/runs/[runId]`
Run explorer:
- current status hero
- timeline
- current checkpoint
- assets/artifacts
- handoffs
- approvals
- logs and events
- git branch/worktree details

### `/flow/approvals`
Approval inbox:
- pending approvals
- diffs / artifacts / evidence
- approve / reject / request changes

### `/flow/assets`
Asset registry:
- uploaded assets
- source provenance
- version history
- workflow references

## 4.3 State language
Use operator-truthful states, not vague AI language:
- queued
- preparing
- waiting_for_assets
- waiting_for_approval
- dispatching
- running
- checkpointed
- failed
- cancelled
- completed

## 4.4 UI design rules
- no fake builder affordances in MVP
- every state badge must map to backend truth
- long logs and YAML must live in proper scroll containers
- timestamps, cost, and git refs must be copyable
- approvals must show explicit evidence, not just a button row
- subpath mounting under `/flow` must work cleanly in local and VPS deploys

---

## 5. Domain model for MVP

### WorkflowDefinition
- id
- name
- version
- description
- source repo path
- yaml path
- input schema
- asset schema
- steps[]
- policy ref

### WorkflowRun
- id
- workflow id/version
- trigger type
- requested by
- status
- branch name
- worktree path
- workspace path
- started at / ended at
- current step id
- cost summary

### WorkflowStepRun
- id
- run id
- step id
- kind
- status
- attempt
- started/ended timestamps
- assigned agent
- input snapshot id
- output snapshot id
- summary

### AssetRecord
- id
- type
- original name
- storage uri/path
- checksum
- metadata json
- source run/step
- version lineage

### ArtifactRecord
- id
- run id
- step id
- type
- path/uri
- promoted bool
- provenance json

### ApprovalRecord
- id
- run id
- step id
- kind
- status
- requested by
- decided by
- rationale
- evidence manifest

### EventRecord
- id
- run id
- step id nullable
- stream sequence
- type
- payload summary
- created at

---

## 6. Workflow package contract

Recommended package layout:

```text
workflows/
  feature-dev/
    workflow.yaml
    policy.yaml
    prompts/
    assets/
    agents/
    tests/
      happy-path.inputs.yaml
      approval-path.inputs.yaml
```

## 6.1 `workflow.yaml` sections
- metadata
- variables
- assets
- outputs
- steps
- checkpoints
- approvals

## 6.2 Validation rules
The CLI/API validator should fail if:
- unknown step kinds are used
- variable defaults mismatch type
- approval step has no approver policy
- referenced prompts/assets are missing
- fan-in dependencies create cycles
- output artifacts are undeclared where required

---

## 7. Backend technical plan

## Phase 0 — foundation

### Deliverables
- repo scaffolding
- Makefile / task runner
- local compose for Redis + app + optional SQLite volume
- config loading and env contract
- basic health/readiness endpoints
- OpenAPI skeleton
- baseline test harness and fixtures for TDD

### Tasks
- create `apps/api` Go module
- wire Chi and Zerolog
- add config package with env + file support
- add `/v1/healthz` and `/v1/readyz`
- add migrations mechanism for SQLite
- add Redis connection bootstrap with readiness reporting
- add CI for fmt, lint, test
- add test helpers for ephemeral SQLite, Redis, workflow fixtures, and run workspaces
- document test-first slice workflow: failing test -> minimal implementation -> refactor -> verify

## Phase 1 — definitions and validation

### Deliverables
- workflow YAML schema
- validator CLI/API
- workflow registry loader
- dry-run plan generation

### Tasks
- define JSON Schema / Go structs for workflow and policy
- implement loader for `workflows/**/workflow.yaml`
- add `ralleh-flow validate <workflow>` CLI
- add dry-run planner returning execution graph, variable issues, missing assets, estimated cost placeholders
- persist workflow definitions metadata into SQLite
- create sample workflows: `feature-development`, `seo-analysis`, `doc-update`

## Phase 2 — run engine

### Deliverables
- create run
- checkpointed sequential execution
- Redis event emission
- run explorer API

### Tasks
- add run creation endpoint + service
- implement branch naming: `workflow/{workflow-name}/{run-id}`
- create Git worktree manager with cleanup rules
- enforce per-run worktree isolation and namespaced runtime directories
- implement run state machine
- make state transitions atomic and idempotent
- emit lifecycle events to Redis Streams and persist summaries to SQLite
- implement checkpoint save/load
- add cancellation and resume semantics
- expose REST endpoints for run list/detail/timeline
- add integration tests for branch/worktree isolation, checkpoint resume, and duplicate command safety

## Phase 3 — agent dispatch and handoffs

### Deliverables
- OpenClaw integration
- agent capability registry
- HANDOFF.md generation

### Tasks
- define agent capability contract
- build OpenClaw dispatch adapter for agent task steps
- record session/run linkage
- generate structured HANDOFF.md per inter-agent transfer
- persist agent outputs and token/cost metrics
- implement retry + escalation policy for agent failures/timeouts
- enforce configurable agent concurrency limits in the scheduler/dispatcher

## Phase 4 — approvals and governance

### Deliverables
- approval step execution
- operator inbox
- reject/resume flow
- audit-grade evidence bundle

### Tasks
- implement approval request records
- expose approval APIs
- add evidence manifests for diff/artifact/log references
- add approve / reject / request-changes actions
- ensure paused run resumes at correct checkpoint only
- persist all governance actions into audit tables

## Phase 5 — assets and artifacts

### Deliverables
- asset ingest
- metadata extraction
- artifact lineage
- promotion flow

### Tasks
- define asset storage layout under repo/local storage
- add checksum + metadata extraction pipeline
- support core MVP types: CSV, JSON, TXT, PDF, images, ZIP, URL list, Git repo ref
- add artifact registry and download/view APIs
- implement artifact promotion with provenance requirements
- add search indexing hooks placeholder for later

## Phase 6 — web UI under `/flow`

### Deliverables
- Nuxt operator app
- dashboard, workflow catalog, run explorer, approval inbox
- SSE or WebSocket live updates
- reverse-proxy-safe subpath mounting for `/flow`

### Tasks
- create `apps/web` with Nuxt, TypeScript, Tailwind
- configure app base path / reverse-proxy mounting for `/flow`
- implement shared shell modeled after Ralleh Voice control-room structure
- build dashboard summary cards and active-runs table
- build workflow catalog and workflow detail pages
- build run explorer with timeline/logs/checkpoints pane
- build approval inbox and evidence review panel
- build diagnostics rail: Redis, agent, git, runtime health
- add dark mode and mobile responsive behavior

## Phase 7 — testing and operations

Testing is not deferred until this phase. Every earlier phase must ship with tests. This phase expands coverage, failure-mode validation, and operational confidence.

### Deliverables
- fixture workflows
- integration tests
- concurrency/recovery test matrix
- VPS deployment contract
- Caddy route guidance for `/flow`

### Tasks
- add unit tests for parser, state machine, git manager, approval logic
- add integration tests with ephemeral SQLite + Redis
- add golden tests for workflow validation and dry-run plans
- add concurrency tests for worker exclusivity, Redis locks, and idempotent retries
- add crash/restart recovery tests for checkpoint resume and event replay safety
- add smoke script for installed deployment
- define Caddy reverse proxy snippet for `/flow`
- define systemd/docker-compose examples
- document backup/restore for SQLite and run artifacts

---

## 8. API outline

MVP REST surface:
- `GET /v1/healthz`
- `GET /v1/readyz`
- `GET /v1/workflows`
- `GET /v1/workflows/{id}`
- `POST /v1/workflows/{id}/validate`
- `POST /v1/workflows/{id}/dry-run`
- `POST /v1/runs`
- `GET /v1/runs`
- `GET /v1/runs/{id}`
- `GET /v1/runs/{id}/timeline`
- `POST /v1/runs/{id}/cancel`
- `POST /v1/runs/{id}/resume`
- `GET /v1/approvals`
- `POST /v1/approvals/{id}/approve`
- `POST /v1/approvals/{id}/reject`
- `POST /v1/assets`
- `GET /v1/assets/{id}`
- `GET /v1/events/stream`

---

## 9. Git execution model

### 9.1 Per-run isolation
Each run should create:
- branch
- worktree
- runtime workspace folder
- run manifest

Required rules:
- one run, one branch, one worktree
- no shared mutable workspace state across runs
- runtime folders must be namespaced by run ID
- workflow definition, prompt set, and asset versions must be pinned at run start

Recommended filesystem layout:

```text
/var/lib/ralleh-flow/
  runs/
    <run-id>/
      workspace/
      snapshots/
      logs/
      handoffs/
      artifacts/
```

### 9.2 Commit discipline
The engine should not auto-commit constantly. Use explicit checkpoint commits only when configured or when a step materially changes versioned state.

### 9.3 Cleanup policy
Completed runs should support:
- keep workspace
- prune worktree, keep snapshots
- archive artifacts only

This must be policy-driven per workflow/run class.

---

## 10. Event model

Recommended event types:
- `workflow.registered`
- `run.created`
- `run.prepared`
- `run.started`
- `step.started`
- `step.checkpointed`
- `step.completed`
- `step.failed`
- `approval.requested`
- `approval.granted`
- `approval.rejected`
- `artifact.created`
- `artifact.promoted`
- `run.cancelled`
- `run.completed`

Event payloads should stay compact and reference records by id instead of embedding giant logs.

---

## 11. Security and governance tasks

MVP security requirements:
- RBAC for operator actions
- approval enforcement in engine, not UI only
- secrets as runtime refs only
- asset access checks on API
- audit trail for user and agent actions
- step sandbox policy for shell/git operations
- timeouts on all external calls
- no trust in agent-produced artifacts without explicit review or policy

Detailed tasks:
- add auth middleware and user identity model
- define operator/admin/reviewer roles
- sign or checksum key manifests and snapshots
- redact secrets from logs/events
- protect artifact download routes
- add immutable audit append path for approval decisions

---

## 12. Concrete MVP task backlog

## Epic A — repo and contracts
- [ ] scaffold monorepo directories
- [ ] add root README, .gitignore, editorconfig, license
- [ ] add package manager/workspace choice and root scripts
- [ ] add Go module for API
- [ ] add Nuxt app for `/flow`
- [ ] add shared contracts package

## Epic B — workflow definitions
- [ ] define workflow YAML schema
- [ ] define policy YAML schema
- [~] implement parser + validator
- [~] add dry-run planner returning execution graph, variable issues, missing assets, estimated cost placeholders
- [ ] add sample workflow packages
- [ ] add CLI validate command
- [ ] add dry-run planner

## Epic C — runtime engine
- [~] SQLite schema + migrations
- [ ] Redis stream bootstrap
- [ ] run state machine
- [ ] checkpoint persistence
- [ ] cancellation and resume logic
- [ ] event persistence summarizer
- [ ] atomic/idempotent transition guardrails
- [~] single-worker lease enforcement per run

## Epic D — Git isolation
- [x] branch naming helper
- [x] worktree creator
- [x] workspace directory contract
- [ ] cleanup/archive policy
- [ ] checkpoint commit helper
- [ ] git error mapping and retries
- [~] Redis lock wrapper for shared repo operations
- [ ] isolation tests for parallel runs

## Coordination status notes
- [x] in-memory coordination for tests/local runtime
- [x] Redis coordinator implementation scaffold
- [x] readiness reflects selected coordination mode and fails honestly when Redis is configured but unavailable
- [~] Redis Streams event bootstrap and consumer-group lifecycle
- [~] run creation transitions into an explicit pending/orchestration-ready state; manual and Redis consumer advancement to `running` now share the same guarded transition path, but step execution is still pending

## Epic E — OpenClaw agent integration
- [ ] agent capability registry
- [ ] dispatch adapter
- [ ] handoff template generation
- [ ] session/run linkage persistence
- [ ] token/cost capture
- [ ] escalation path for failures

## Epic F — approvals
- [ ] approval entity + migrations
- [ ] approval request API
- [ ] evidence bundle generation
- [ ] UI inbox
- [ ] resume/reject path
- [ ] audit append for decisions

## Epic G — assets/artifacts
- [ ] asset upload API
- [ ] checksum + MIME detection
- [ ] metadata extraction per file type
- [ ] artifact registry API
- [ ] promotion flow
- [ ] lineage UI

## Epic H — web UI
- [~] app shell modeled after Ralleh Voice layout principles
- [~] dashboard
- [~] workflow catalog
- [~] workflow detail
- [~] run explorer timeline
- [ ] approvals inbox
- [ ] diagnostics rail
- [ ] live event stream
- [ ] subpath mounting validation for `/flow`

## Epic I — operations
- [ ] docker-compose dev stack
- [ ] systemd service file
- [ ] Caddy `/flow` route example
- [ ] smoke check script
- [ ] backup/restore docs
- [ ] production env example

---

## 13. Recommended first build slice

### Slice 1
- [x] load one sample workflow from YAML
- [x] validate inputs
- [x] create run record in SQLite
- [x] create branch/worktree
- [ ] execute one sequential `agent_task`
- [x] persist run timeline events
- [x] show run in `/flow/runs/[id]`

Why this slice first:
- proves the core architecture
- exercises Git, SQLite, Redis, API, UI, and OpenClaw integration together
- exposes missing contracts early
- avoids premature builder complexity

---

## 14. What is already scaffolded now

Already created in this repo:
- Nuxt app shell in `apps/web`
- Go API skeleton in `apps/api`
- sample workflow fixture in `workflows/examples/feature-development/workflow.yaml`
- Caddy and systemd starter files in `deploy/`
- root workspace files for pnpm and Make targets

Current scaffold originally used mock data, but this is now partially superseded.

Implemented since the original draft:
- real workflow loading from YAML for catalog/detail
- SQLite-backed run creation/list/detail
- required workflow variable enforcement on create-run
- unique run-id namespacing for runtime directories and branch naming strings
- basic workflow validation endpoint
- live workflow detail/create-run UI and run explorer UI

Still not implemented and should be treated as active gaps:
- actual git worktree creation and repository isolation enforcement beyond naming/runtime-dir prep
- Redis Streams, distributed locks, and single-worker lease enforcement
- dry-run endpoint and execution graph planning
- real step execution/checkpoint/resume lifecycle
- approval persistence/actions and asset/artifact APIs
- automated UI tests beyond build verification

---

## 15. Recommendation

My recommendation is to implement **Ralleh Flow as a monorepo with a Go runtime and a Nuxt control-room UI under `/flow`**, borrowing the layout discipline and deployment posture of Ralleh Voice.

The critical success condition is this:

**make the run engine truthful before making the builder fancy.**
