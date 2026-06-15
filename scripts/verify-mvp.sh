#!/usr/bin/env bash

set -euo pipefail

echo "[verify:mvp] api:test"
pnpm api:test

echo "[verify:mvp] web:lint"
pnpm web:lint

echo "[verify:mvp] web:build"
pnpm web:build

echo "[verify:mvp] web test:ui"
pnpm --dir apps/web test:ui

echo "[verify:mvp] web test:ui:live"
pnpm --dir apps/web test:ui:live

echo "[verify:mvp] deployable MVP verified"
