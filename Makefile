.PHONY: web-install web-dev web-build web-typecheck api-run api-test

web-install:
	cd apps/web && pnpm install

web-dev:
	cd apps/web && pnpm dev

web-build:
	cd apps/web && pnpm build

web-typecheck:
	cd apps/web && pnpm exec nuxi typecheck

api-run:
	cd apps/api && go run ./cmd/ralleh-flow-api

api-test:
	cd apps/api && go test ./...
