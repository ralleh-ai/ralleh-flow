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