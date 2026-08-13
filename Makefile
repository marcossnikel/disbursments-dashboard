.PHONY: backend frontend generate check-generated

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
