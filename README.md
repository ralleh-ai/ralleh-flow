# Ralleh Flow

> **Git-native workflow orchestration for AI + human teams.**

Ralleh Flow is a self-hosted control room for running real work through structured workflows, specialized agents, and human approval gates — with Git-backed traceability from start to finish.

## Why Ralleh Flow exists

Most AI workflows break down at scale because they lack:

- reproducibility
- operational visibility
- safe handoffs between automation and human judgment
- trustworthy audit trails

Ralleh Flow solves that by making **workflow execution explicit, observable, and governable**.

## What it does

Ralleh Flow combines:

- **Workflow orchestration** from Git-tracked definitions
- **Isolated run execution** using branch/worktree boundaries
- **Agent dispatch + context handoffs** via OpenClaw
- **Approval checkpoints** for critical decisions
- **Run timeline and evidence surfaces** for operators
- **Artifact and outcome tracking** tied to provenance

Core product stance: **Git is the trust layer, Redis is runtime coordination, SQLite is operational metadata.**

## Workflow patterns supported today

Ralleh Flow already supports multiple execution patterns that teams can mix depending on risk, speed, and governance needs:

- **Human-gated approval workflow**
  - Create a run from a Git-tracked workflow
  - Progress steps until an approval gate is reached
  - Approve, reject, or request changes with rationale
  - Resume safely with full timeline traceability

- **Agent handoff workflow (OpenClaw dispatch)**
  - Dispatch active steps to specialized agents
  - Track dispatch metadata and session linkage in run history
  - Complete or fail the step with explicit outcomes

- **Operator-driven mission control workflow**
  - Advance pending runs intentionally from the UI/API
  - Intervene during failures and recover from known error states
  - Execute run mutations through explicit controls rather than hidden automation

- **Git-isolated execution workflow**
  - Run-scoped branch/worktree isolation to avoid cross-run contamination
  - Runtime directories and checkpoints tied to run identity
  - Repeatable execution context for debugging and audit

- **Resilient callback/recovery workflow**
  - Recover from approval transitions and step-level failures
  - Maintain deterministic run state transitions (`pending -> running -> awaiting_approval -> resumed/completed/failed`)
  - Preserve operator trust with visible timeline evidence

These patterns let Ralleh Flow function as a true **AI operations control room**: fast enough for day-to-day execution, but governed enough for high-stakes production work.

## Real-world workflow examples

Here are believable, high-signal examples of what teams can run through Ralleh Flow today:

- **Production incident triage + patch rollout**
  - Trigger run from an incident workflow
  - Agent gathers logs, proposes root-cause hypotheses, drafts a patch
  - Human gate approves rollback vs fix-forward decision
  - Run records exact decision path, patch artifacts, and final outcome

- **Security dependency update with controlled approval**
  - Workflow scans dependencies and prepares update PRs
  - Agent executes compatibility checks and summarizes breaking risk
  - Approval gate requires security/lead sign-off before merge
  - Timeline preserves who approved, why, and which evidence was reviewed

- **Weekly content pipeline (draft → review → publish-ready)**
  - Workflow assembles source material and drafts structured content
  - Editorial agent iterates through revisions against quality rules
  - Human reviewer approves, rejects, or requests changes with rationale
  - Final package ships with traceable revision history

- **Customer escalation response workflow**
  - Intake step captures escalation facts and desired SLA path
  - Agent prepares technical summary + response options
  - Operator approves outbound response and remediation actions
  - Execution history links every action back to run context

- **Data/report generation with compliance checkpoints**
  - Workflow runs data pull + report generation on schedule or demand
  - Validation steps flag anomalies and require explicit approval on threshold breaches
  - Approved reports move forward; failed validations branch into recovery actions
  - Full audit trail supports postmortem and compliance review

These examples are intentionally generic, but they map directly to the same core capabilities in this repo: deterministic state transitions, approval checkpoints, agent handoffs, and Git-backed provenance.

## Why it matters (the potential)

Ralleh Flow is designed to evolve from a workflow runner into a durable operations platform for AI-native teams:

- **From prompts to systems**: repeatable workflows instead of ad hoc chat execution
- **From black box to control room**: live state, agent activity, and intervention points
- **From output to accountability**: decision and artifact lineage you can inspect
- **From solo automation to team operations**: specialized agents working under policy and human governance

In short: it is built to turn AI assistance into production-grade operational capability.

## Repository map

```text
ralleh-flow/
  apps/
    api/          # Go API + runtime engine
    web/          # Nuxt operator UI (mounted under /flow)
  deploy/         # Systemd/Caddy/deployment scripts and templates
  docs/           # Product, architecture, doctrine, and runbooks
  runtime/        # Runtime-related implementation support
  scripts/        # Verification and local automation
```

## Quick start

### Prerequisites

- Node.js 22+
- pnpm 10+
- Go (for API runtime/test)
- Redis + SQLite available for local runtime behavior

### Install

```bash
pnpm install
```

### Run web

```bash
pnpm web:dev
```

### Run API

```bash
pnpm api:run
```

### Verify MVP gate

```bash
pnpm verify:mvp
```

## Documentation

Start here:

- [`docs/README.md`](docs/README.md) — full docs index
- [`docs/project-charter.md`](docs/project-charter.md) — product charter
- [`docs/development-plan.md`](docs/development-plan.md) — engineering plan
- [`docs/ui-doctrine.md`](docs/ui-doctrine.md) — UI philosophy
- [`docs/deploy-runbook.md`](docs/deploy-runbook.md) — production deployment baseline

Additional project docs:

- [`CONTRIBUTING.md`](CONTRIBUTING.md) — contribution workflow and quality gates
- [`ROADMAP.md`](ROADMAP.md) — phased direction and strategic potential

## Golden standard for this repo

This repository should remain:

- **Descriptive**: every newcomer can understand what Ralleh Flow is and why it matters
- **Operational**: docs support shipping, debugging, and safe deployment
- **Organized**: clear paths for product, architecture, runbooks, and contributor guidance
- **Trustworthy**: claims are testable and tied to code, runbooks, or evidence

If documentation drifts from implementation, implementation truth wins and docs must be updated in the same change set.