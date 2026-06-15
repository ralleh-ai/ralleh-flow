#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SERVICE_FILE="$ROOT_DIR/deploy/systemd/ralleh-flow-api.service"
CADDY_SNIPPET="$ROOT_DIR/deploy/caddy/ralleh-flow.caddy"

cd "$ROOT_DIR"

echo "[preflight] repo root: $ROOT_DIR"

echo "[preflight] checking required commands"
for cmd in git pnpm go; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "[preflight] missing command: $cmd" >&2
    exit 1
  fi
done

echo "[preflight] git branch + cleanliness"
git status --short --branch

echo "[preflight] verify:mvp"
pnpm verify:mvp

echo "[preflight] validating systemd unit syntax"
if command -v systemd-analyze >/dev/null 2>&1; then
  systemd-analyze verify "$SERVICE_FILE"
else
  echo "[preflight] systemd-analyze not found; skipped unit syntax verification"
fi

echo "[preflight] validating caddy snippet syntax"
if command -v caddy >/dev/null 2>&1; then
  tmp_caddyfile="$(mktemp)"
  trap 'rm -f "$tmp_caddyfile"' EXIT
  cat > "$tmp_caddyfile" <<EOF
example.invalid {
  import $CADDY_SNIPPET
}
EOF
  caddy validate --config "$tmp_caddyfile" --adapter caddyfile
else
  echo "[preflight] caddy not found; skipped caddy syntax verification"
fi

echo "[preflight] all checks passed"
