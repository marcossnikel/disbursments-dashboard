.PHONY: backend frontend build generate check-generated lint test test-race verify

backend:
	cd backend && go run ./cmd/api

frontend:
	cd frontend && pnpm dev

generate:
	cd backend && go tool oapi-codegen --config oapi-codegen.yaml ../api/openapi.yaml
	cd frontend && pnpm generate:types

check-generated:
	$(MAKE) generate
	git diff --exit-code -- backend/internal/openapi/generated.go frontend/src/api/generated/schema.ts

build:
	cd backend && go build ./...
	cd frontend && pnpm build

lint:
	cd backend && go vet ./...
	cd frontend && pnpm lint
	cd frontend && pnpm format:check
	cd frontend && pnpm typecheck

test:
	cd backend && go test ./...
	cd frontend && pnpm test

test-race:
	cd backend && go test -race ./...

verify: lint test test-race build check-generated
