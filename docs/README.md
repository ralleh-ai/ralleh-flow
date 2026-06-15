# Ralleh Flow Docs Index

This directory contains the canonical product and engineering documentation for Ralleh Flow.

## Start here

- [`../README.md`](../README.md) — project overview and quick start
- [`project-charter.md`](project-charter.md) — product purpose and non-negotiable stance
- [`architecture.md`](architecture.md) — runtime and system architecture overview
- [`development-plan.md`](development-plan.md) — implementation plan and MVP shape
- [`ui-doctrine.md`](ui-doctrine.md) — UI philosophy and operator experience doctrine
- [`deploy-runbook.md`](deploy-runbook.md) — deployment, rollback, and hardening baseline
- [`deployable-mvp-verified.md`](deployable-mvp-verified.md) — verification definition and gate
- [`code-review-2026-06.md`](code-review-2026-06.md) — structured architecture/code/security review and ecosystem integration contracts

## How docs are maintained

Documentation is part of the product.

- update docs in the same PR as behavior changes
- prefer concrete claims linked to tests, scripts, or code paths
- keep high-level docs concise and link to deep references
- remove stale sections instead of leaving contradictory guidance

## Golden documentation standard

A repo doc set is at standard when it helps a newcomer quickly answer:

1. What is this product?
2. Why does it matter?
3. How do I run and verify it?
4. How is it architected?
5. How do I safely contribute and deploy changes?
6. Where is it going next?