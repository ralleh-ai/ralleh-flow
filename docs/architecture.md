# Ralleh Flow Architecture Overview

This document summarizes the current intended architecture for Ralleh Flow MVP and near-term hardening.

## System purpose

Ralleh Flow orchestrates workflow runs that combine:

- machine execution (runtime + tools)
- specialized AI agent work
- human approvals for trust-critical transitions

The architecture prioritizes reproducibility, isolation, and operational visibility.

## High-level components

## 1) Web UI (`apps/web`)

- Nuxt-based control room under `/flow`
- primary surfaces: run state, timeline, approvals, artifacts, outcomes
- optimized for situational awareness and intervention, not CRUD administration

## 2) API + Runtime (`apps/api`)

- Go service exposing orchestration and run APIs
- workflow execution engine with checkpoint persistence
- approval pause/resume lifecycle handling
- dispatch integration with OpenClaw agent workflows

## 3) Persistence and coordination

### Git (source of truth for versioned workflow intent)

- workflow definitions
- prompts/templates
- versioned assets
- promotion-ready artifacts

### SQLite (operational metadata)

- run records
- step records
- approvals/audit indexes
- summary counters and cost metadata

### Redis (runtime coordination)

- queue and event stream transport
- distributed locks for shared operations
- leases/signals for active orchestrator work

## Runtime invariants (must hold)

- each run executes in an isolated branch/worktree/workspace context
- run state transitions are atomic and idempotent
- one orchestrator worker controls one run at a time
- parallelism is explicit and dependency-driven
- artifacts must preserve provenance metadata for trust and traceability

## Execution lifecycle (conceptual)

1. load workflow definition from versioned source
2. resolve typed variables and references
3. initialize isolated run context (branch/worktree/runtime dirs)
4. execute steps with checkpoint and event emission
5. pause at approval gates when required
6. resume and continue after decision
7. finalize outputs with metadata and audit linkage

## Operational visibility

Ralleh Flow should make these relationships visible to operators:

- objective -> run
- run -> steps
- run -> approvals
- run -> agent activity
- run -> repository/branch/worktree
- run -> artifacts/outcomes

## Architecture evolution

Near-term evolution focuses on:

- stronger policy separation from workflow logic
- richer snapshot manifests for replay/debug
- hardened recovery across process restarts
- clearer contract boundaries for step kinds and agent capabilities

See also:

- [`project-charter.md`](project-charter.md)
- [`development-plan.md`](development-plan.md)
- [`ui-doctrine.md`](ui-doctrine.md)
- [`../ROADMAP.md`](../ROADMAP.md)