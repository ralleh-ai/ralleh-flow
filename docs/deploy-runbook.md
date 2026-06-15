# Ralleh Flow Deploy Runbook (Production Hardening Baseline)

This runbook defines a safe, repeatable baseline for deployment and rollback during MVP hardening.

## 1) Preflight gate (required)

From repository root:

```bash
./deploy/scripts/preflight.sh
```

What this covers:
- full deployability verification: `pnpm verify:mvp`
- systemd unit syntax check (when `systemd-analyze` is available)
- Caddy snippet validation (when `caddy` is available)

Do not deploy if preflight fails.

## 2) Service unit hardening baseline

Hardened unit source:
- `deploy/systemd/ralleh-flow-api.service`

Install/update on host (example):

```bash
sudo install -m 644 deploy/systemd/ralleh-flow-api.service /etc/systemd/system/ralleh-flow-api.service
sudo systemctl daemon-reload
sudo systemctl enable --now ralleh-flow-api.service
sudo systemctl restart ralleh-flow-api.service
sudo systemctl status --no-pager ralleh-flow-api.service
```

## 3) Caddy route validation + reload

Snippet source:
- `deploy/caddy/ralleh-flow.caddy`

Example host flow:

```bash
sudo caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
sudo systemctl reload caddy
```

## 4) API mutation protection baseline

For production, set a write token so all `POST /v1/*` endpoints require bearer auth.

Environment variable:
- `RALLEH_FLOW_API_WRITE_TOKEN=<strong-random-token>`

Example verification:

```bash
curl -i -X POST http://127.0.0.1:4310/v1/runs -d '{}'
curl -i -X POST http://127.0.0.1:4310/v1/runs \
  -H "Authorization: Bearer ${RALLEH_FLOW_API_WRITE_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"workflowId":"example"}'
```

## 5) Post-deploy verification

At minimum:

```bash
curl -fsS http://127.0.0.1:4310/v1/healthz
```

Then run the web smoke lane from CI or local host:

```bash
pnpm --dir apps/web test:ui:live
```

## 6) Rollback procedure

If a deploy is unhealthy:

1. Identify last known good commit.
2. Roll back working tree and restart services.

Example:

```bash
git checkout <last-known-good-commit>
./deploy/scripts/preflight.sh
sudo systemctl restart ralleh-flow-api.service
sudo systemctl reload caddy
```

3. Verify health:

```bash
curl -fsS http://127.0.0.1:4310/v1/healthz
```

4. Capture incident notes in CORTEX before next attempt.
