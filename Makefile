.PHONY: backend frontend

backend:
	cd backend && go run ./cmd/api

frontend:
	cd frontend && pnpm dev
