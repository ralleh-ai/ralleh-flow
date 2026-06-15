# Playwright lane policy

This directory is split into two reliability lanes:

- **Deterministic regression lane (default):**
  - Command: `pnpm --dir apps/web test:ui`
  - Runs all specs **except** tests tagged with `@live`
  - Uses mocked API fixtures for stable regressions
  - Retry policy: `retries: 0`
  - Worker policy: `workers: 1` (serial for stability)

- **Live API smoke lane:**
  - Command: `pnpm --dir apps/web test:ui:live`
  - Runs only tests tagged with `@live`
  - Boots local API + web app and validates live contract behavior
  - Retry policy: `retries: 0`
  - Worker policy: `workers: 1`

Failure evidence defaults for both lanes:
- traces: `retain-on-failure`
- screenshots: `only-on-failure`
- videos: `retain-on-failure`
- HTML reports are generated for CI artifact upload
