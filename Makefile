SHELL := /bin/bash
.DEFAULT_GOAL := help

COMPOSE_BASE := deploy/compose/compose.yaml
COMPOSE_DEV  := -f $(COMPOSE_BASE) -f deploy/compose/compose.dev.yaml -f deploy/compose/compose.observability.yaml
COMPOSE_PROD := -f $(COMPOSE_BASE) -f deploy/compose/compose.prod.yaml -f deploy/compose/compose.observability.yaml

## ----------------------------------------------------------------- help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

## ----------------------------------------------------------------- setup

setup: ## Install tool binaries, git hooks and frontend dependencies
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/pressly/goose/v3/cmd/goose@latest
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
	go install github.com/matryer/moq@latest
	go install github.com/incu6us/goimports-reviser/v3@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/fe3dback/go-arch-lint@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/air-verse/air@latest
	cd web && pnpm install
	pre-commit install || true

## ----------------------------------------------------------------- dev

dev: ## Start the full local stack (app + data + observability)
	docker compose $(COMPOSE_DEV) up -d --build
	@echo "web  http://localhost:5173   api http://localhost:8080"
	@echo "graf http://localhost:3000   mail http://localhost:8025   minio http://localhost:9001"

dev-down: ## Stop the local stack (volumes preserved)
	docker compose $(COMPOSE_DEV) down

logs: ## Tail application logs
	docker compose $(COMPOSE_DEV) logs -f api worker

prod-up: ## Start the production stack
	docker compose $(COMPOSE_PROD) up -d

## ----------------------------------------------------------------- codegen

gen: gen-sql gen-api gen-mocks gen-web ## Regenerate everything

gen-sql: ## sqlc: SQL -> typed Go
	sqlc generate

gen-api: bundle-api ## oapi-codegen: OpenAPI -> server interfaces + client
	oapi-codegen -config api/openapi/codegen-server.yaml api/openapi/openapi.bundle.yaml
	oapi-codegen -config api/openapi/codegen-client.yaml api/openapi/openapi.bundle.yaml

bundle-api: ## Bundle split OpenAPI components for code generators
	npx @redocly/cli bundle api/openapi/openapi.yaml -o api/openapi/openapi.bundle.yaml

gen-mocks: ## moq: interfaces -> mocks
	go generate ./...

gen-web: ## openapi-typescript: OpenAPI -> TS types + MSW handlers
	cd web && pnpm run gen:api

## ----------------------------------------------------------------- database

migrate-up: ## Apply all migrations
	go run ./cmd/migrate up

migrate-down: ## Roll back the last migration
	go run ./cmd/migrate down

migrate-status: ## Show migration status
	go run ./cmd/migrate status

migrate-new: ## Create a migration: make migrate-new MODULE=auth NAME=add_mfa
	@test -n "$(MODULE)" && test -n "$(NAME)" || (echo "MODULE and NAME are required" && exit 1)
	@mkdir -p db/migrations/$(MODULE)
	goose -dir db/migrations/$(MODULE) create $(NAME) sql

seed: ## Load the development dataset
	go run ./cmd/seed

db-reset-DANGEROUS: ## DESTROYS all local data. Requires confirmation.
	@read -p "This deletes every local volume including learner data. Type 'yes' to continue: " c; \
	 [ "$$c" = "yes" ] && docker compose $(COMPOSE_DEV) down -v || echo "aborted"

## ----------------------------------------------------------------- quality

check: fmt vet lint arch test ## Everything that must pass before you finish

fmt: ## Format
	gofmt -w .
	goimports-reviser -project-name github.com/fluentra/fluentra ./...
	cd web && pnpm run format

vet: ## go vet
	go vet ./...

lint: ## golangci-lint + eslint + spectral
	golangci-lint run ./...
	cd web && pnpm run lint
	npx @stoplight/spectral-cli lint api/openapi/openapi.yaml

arch: ## Enforce module boundaries (rules L1/L2)
	go-arch-lint check

## ----------------------------------------------------------------- tests

test: ## Unit tests with the race detector
	go test -race -short ./...

test-int: ## Integration tests (testcontainers; Docker required)
	go test -race -tags=integration ./... -timeout 10m

test-contract: ## Handler <-> OpenAPI conformance
	go test -tags=contract ./internal/... -run TestContract

test-web: ## Frontend unit tests
	cd web && pnpm run test

test-e2e: ## Playwright end-to-end
	cd web && pnpm run test:e2e

test-load: ## k6 load scenarios
	k6 run test/load/scenarios.js

test-eval: ## AI evaluation suites (mock provider)
	go run ./cmd/eval --suite all --provider mock

cover: ## Coverage report
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out

## ----------------------------------------------------------------- docs

docs: ## Regenerate module documentation from the manifest
	node tools/docgen/generate.mjs

docs-check: ## Fail if documentation is stale or has drifted
	node tools/docgen/generate.mjs --check
	npx markdownlint-cli2 "**/*.md" "#node_modules" "#web/node_modules"
	npx lychee --no-progress --offline .

## ----------------------------------------------------------------- ci

ci: ## Run exactly what CI runs, in the same order
	$(MAKE) fmt vet lint arch
	$(MAKE) gen && git diff --exit-code || (echo "generated code is stale: run make gen" && exit 1)
	$(MAKE) test
	$(MAKE) test-int
	$(MAKE) docs-check

ci-fast: fmt vet lint arch test ## Inner-loop subset

security: ## Security scans
	govulncheck ./...
	gitleaks detect --no-banner
	cd web && pnpm audit --audit-level=high

.PHONY: help setup dev dev-down logs prod-up gen gen-sql gen-api gen-mocks gen-web \
        migrate-up migrate-down migrate-status migrate-new seed db-reset-DANGEROUS \
        check fmt vet lint arch test test-int test-contract test-web test-e2e test-load \
        test-eval cover docs docs-check ci ci-fast security
