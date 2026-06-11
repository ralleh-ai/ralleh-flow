# Ralleh Flow Charter

This repo implements **Ralleh Flow**: a self-hosted, Git-native workflow orchestration platform for OpenClaw agents and human-governed execution.

## Core stance

- Git is the permanent audit log.
- Redis is disposable runtime coordination.
- SQLite stores metadata, not authoritative workflow truth.
- Agents execute work, humans approve critical transitions.
- The product must run comfortably on a single VPS.
- The primary operator UI is served under `/flow`.
- Test-first development is required for backend/runtime behavior and expected UI state behavior.
- Concurrency correctness, isolation, and recovery are first-class product requirements, not later hardening work.

## UI product stance

The operator experience should mirror the clarity and operational tone of **Ralleh Voice**, while using the standard Ralleh Nuxt/Vue stack:

- strong control-room layout
- clear state visibility
- one dominant primary workflow surface
- diagnostics available without cluttering the main experience
- honest status and no fake controls
- clean reverse-proxy mounting under `/flow`

## Initial deliverables

1. workflow package contract
2. workflow run engine
3. Git/worktree execution isolation
4. run timeline + explorer UI
5. approvals and pause/resume behavior
6. asset registry + ingestion pipeline
7. OpenClaw agent dispatch + handoff generation
8. cost/event/audit visibility
9. VPS deployment contract for `/flow` and `/api/flow`
10. automated test coverage for runtime correctness, recovery, and concurrency invariants

## Engineering guardrails

- every meaningful runtime rule should be expressed in tests before or alongside implementation
- workflow state transitions must be atomic, idempotent, and safe under retries
- each run must have isolated branch, worktree, runtime directories, and immutable execution inputs
- shared repository operations must be coordinated with Redis locks
- only one worker may actively orchestrate a given run at a time
- parallelism must be explicit in workflow definitions and safe by construction
- resume/restart behavior must preserve correctness after crashes or process restarts
