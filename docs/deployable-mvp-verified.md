# Deployable MVP Verified

`deployable MVP verified` means the repository passes the full production hardening gate from a clean checkout:

1. `pnpm api:test`
2. `pnpm web:lint`
3. `pnpm web:build`
4. `pnpm --dir apps/web test:ui`
5. `pnpm --dir apps/web test:ui:live`

Run the unified local gate from repository root:

```bash
pnpm verify:mvp
```

Implementation detail:

- Root command: `pnpm verify:mvp`
- Script entrypoint: `./scripts/verify-mvp.sh`
- CI workflow includes backend verification + web build before e2e lanes: `.github/workflows/web-e2e-reliability.yml`
- Deployment preflight wrapper: `./deploy/scripts/preflight.sh` (runs MVP gate + deployment config syntax checks)
