# Contributing to Ralleh Flow

Thanks for contributing.

Ralleh Flow is an operations product, so quality and clarity are non-negotiable. Every contribution should improve reliability, observability, and trust.

## Contribution principles

- Keep it simple and explicit
- Prefer deterministic behavior over cleverness
- Preserve run isolation and auditability invariants
- Pair behavior changes with tests
- Update docs with code changes when behavior or expectations shift

## Development setup

```bash
pnpm install
```

Run web:

```bash
pnpm web:dev
```

Run API:

```bash
pnpm api:run
```

## Required quality gates

Before opening or merging a PR:

```bash
pnpm api:test
pnpm web:lint
pnpm web:build
pnpm verify:mvp
```

If a gate is intentionally skipped, document the reason in the PR and create a follow-up task.

## Pull request checklist

- [ ] Scope is clear and limited
- [ ] Tests added/updated for behavior changes
- [ ] Docs updated (`README`, `docs/*`, or runbooks) when needed
- [ ] Backward-compat and migration impact evaluated
- [ ] Operational impact explained (runtime, deploy, rollback)

## Commit guidance

Use clear, imperative commit messages.

Examples:

- `feat(runtime): add checkpoint manifest persistence`
- `fix(api): enforce immutable asset versions per run`
- `docs: align deploy runbook with preflight gate`

## Architecture-sensitive areas

Changes touching these areas need extra care and explicit verification notes:

- run orchestration state machine
- Redis lock/lease behavior
- Git worktree and branch isolation
- approval pause/resume transitions
- artifact provenance and promotion

## Documentation expectations

When you introduce or change behavior, update the closest relevant doc:

- product intent: `README.md` and `docs/project-charter.md`
- architecture/runtime behavior: `docs/architecture.md`
- UX behavior and IA: `docs/ui-doctrine.md`
- deploy/process changes: `docs/deploy-runbook.md`
- strategic direction: `ROADMAP.md`

## Security and secrets

- Never commit secrets
- Use `.env.example` for required variable documentation
- Treat secret values as references in workflow configuration; resolve only at execution boundary

## Need help?

If you are unsure about a runtime invariant or product decision, open a draft PR and ask early. It is better to align first than ship incorrect assumptions.