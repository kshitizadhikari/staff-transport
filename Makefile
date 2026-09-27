.PHONY: help setup infra-up infra-down stack-up stack-down migrate-up migrate-down seed-manager backend-run backend-worker backend-logs backend-build backend-test backend-fmt backend-vet backend-check web-dev mobile-dev check

help:
	@echo "Targets:"
	@echo "  setup          Install JS dependencies (pnpm)"
	@echo "  infra-up       Start PostgreSQL/PostGIS and Redis"
	@echo "  infra-down     Stop infrastructure"
	@echo "  stack-up       Build and start the full backend stack (db, redis, api, worker)"
	@echo "  stack-down     Stop the full backend stack"
	@echo "  migrate-up     Apply database migrations"
	@echo "  migrate-down   Roll back the latest migration"
	@echo "  seed-manager   Create/update the local manager account"
	@echo "  backend-run    Run the API container in the foreground"
	@echo "  backend-worker Run the worker container in the foreground"
	@echo "  backend-logs   Tail API and worker logs"
	@echo "  backend-check  Format, vet, and test the backend"
	@echo "  web-dev        Run the web app"
	@echo "  mobile-dev     Run the driver mobile app"
	@echo "  check          Run all backend and JS checks"

setup:
	pnpm install

infra-up:
	docker compose -f docker-compose.dev.yml up -d postgres redis

infra-down:
	docker compose -f docker-compose.dev.yml down

stack-up:
	docker compose -f docker-compose.dev.yml up -d --build

stack-down:
	docker compose -f docker-compose.dev.yml down

migrate-up:
	docker compose -f docker-compose.dev.yml run --rm migrate

migrate-down:
	docker compose -f docker-compose.dev.yml run --rm migrate \
		-path=/migrations \
		-database=postgres://postgres:postgres@postgres:5432/staff_transport?sslmode=disable \
		down 1

seed-manager:
	docker compose -f docker-compose.dev.yml run --rm seed

backend-run:
	docker compose -f docker-compose.dev.yml up --build api

backend-worker:
	docker compose -f docker-compose.dev.yml up --build worker

backend-logs:
	docker compose -f docker-compose.dev.yml logs -f api worker

backend-build:
	cd backend && go build ./...

backend-test:
	cd backend && go test ./...

backend-fmt:
	cd backend && go fmt ./...

backend-vet:
	cd backend && go vet ./...

backend-check: backend-fmt backend-vet backend-test

web-dev:
	pnpm --filter web dev

mobile-dev:
	pnpm --filter driver-mobile start

check: backend-check
	pnpm lint
	pnpm typecheck
	pnpm build
