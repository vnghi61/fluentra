SHELL := /bin/bash
.DEFAULT_GOAL := help

COMPOSE_BASE := deploy/compose/compose.yaml
# compose.dev.yaml publishes the data-service ports and stays valid on its own.
# The observability port overrides live in their own file because a service that
# exists only as a `ports:` entry is what makes compose reject the whole project
# with "has neither an image nor a build context specified".
COMPOSE_DEV  := -f $(COMPOSE_BASE) \
                -f deploy/compose/compose.dev.yaml \
                -f deploy/compose/compose.observability.yaml \
                -f deploy/compose/compose.observability.dev.yaml
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
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	go install github.com/fe3dback/go-arch-lint@latest
	go install golang.org/x/vuln/cmd/govulncheck@latest
	go install github.com/air-verse/air@latest
	cd web && pnpm install
	pre-commit install || true

## ----------------------------------------------------------------- dev

dev: ## Start the full local stack (app + data + observability)
	docker compose $(COMPOSE_DEV) up -d --build
	@echo "web    http://localhost:5173   api http://localhost:8080   worker http://localhost:8081"
	@echo "graf   http://localhost:3000   mail http://localhost:8025   minio  http://localhost:9001"

dev-down: ## Stop the local stack (volumes preserved)
	docker compose $(COMPOSE_DEV) down

logs: ## Tail application logs
	docker compose $(COMPOSE_DEV) logs -f api worker

prod-up: ## Start the production stack
	docker compose $(COMPOSE_PROD) up -d

api: ## Run the API server on the host
	go run ./cmd/api

worker: ## Run the background worker on the host
	go run ./cmd/worker

web: ## Run the frontend dev server on the host
	cd web && pnpm dev

## ----------------------------------------------------------------- codegen

gen: gen-backend gen-web ## Regenerate everything

gen-backend: gen-sql gen-api gen-mocks ## Regenerate the Go side only (what backend CI checks)

gen-sql: ## sqlc: SQL -> typed Go
	sqlc generate

gen-api: bundle-api ## oapi-codegen: OpenAPI -> server interfaces + client
	oapi-codegen -config api/openapi/codegen-server.yaml api/openapi/openapi.bundle.yaml
	oapi-codegen -config api/openapi/codegen-client.yaml api/openapi/openapi.bundle.yaml

# The bundler version is pinned. Unpinned, `npx @redocly/cli` resolves to
# whatever is current, two bundlers emit subtly different YAML, and that lands in
# the base64 spec embedded in server.gen.go — so the staleness gate would fail on
# pull requests that changed nothing. That is exactly what it was doing.
REDOCLY_VERSION ?= 2.46.0

bundle-api: ## Bundle split OpenAPI components for code generators
	npx --yes @redocly/cli@$(REDOCLY_VERSION) bundle api/openapi/openapi.yaml -o api/openapi/openapi.bundle.yaml

gen-mocks: ## moq: interfaces -> mocks
	go generate ./...

gen-web: ## openapi-typescript: OpenAPI -> TS types + MSW handlers
	cd web && pnpm run gen:api

# gen-check-web is the frontend half of the staleness gate. It is separate from
# gen-check because the backend "generated code is current" CI job has Node but
# not pnpm and does not install web dependencies, so folding this into gen-check
# would break that job.
#
# It exists because gen-check alone was not enough: P1.2 added four operations
# to openapi.yaml, `make gen-check` was green, and the frontend job failed on a
# stale src/types/api.ts. A spec change writes generated code on both sides of
# the repository, and a gate that only checks one side reports success for a
# tree that does not build.
gen-check-web: gen-web ## Fail if the generated web types are not what the generator produces
	@dirty=$$(git status --porcelain -- web/src/types/api.ts); 	 if [ -n "$$dirty" ]; then 	   echo "generated web types are stale: run make gen-web and commit the result"; 	   git diff --stat -- web/src/types/api.ts; 	   exit 1; 	 fi

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

fmt-check: ## Fail if anything is unformatted (CI checks; `make fmt` fixes)
	@unformatted=$$(gofmt -l .); \
	 if [ -n "$$unformatted" ]; then echo "run make fmt:"; echo "$$unformatted"; exit 1; fi

lint: ## golangci-lint + eslint + spectral
	golangci-lint run ./...
	cd web && pnpm run lint
	npx @stoplight/spectral-cli lint api/openapi/openapi.yaml

# Files behind //go:build integration are invisible to the default lint run, and
# that is where every container-backed test lives. Linting them separately is
# what stops that becoming a permanent blind spot.
lint-int: ## golangci-lint over the integration-tagged files
	golangci-lint run --build-tags=integration ./...

lint-go: ## Just the Go linters, no Node required
	golangci-lint run ./...
	golangci-lint run --build-tags=integration ./...

arch: ## Enforce module boundaries (rules L1/L2)
	# bash, not sh. The script declares `#!/usr/bin/env bash` and uses
	# `set -o pipefail`, but invoking it through `sh` overrides the shebang —
	# and `sh` is dash on Ubuntu, which has no pipefail. It worked locally only
	# because Git for Windows ships bash as sh.
	bash scripts/verify-arch-lint.sh

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

# COVERAGE_MIN is a ratchet, not a target. It sits just under the measured
# figure so an ordinary refactor does not trip it, while deleting a test suite
# does. Raise it when coverage rises; never lower it to make a build pass.
#
# Raised from 45.0 when the gate started measuring hand-written code only (see
# cover-gate). Over that set, `main` reads 61.6% and this branch 67.2%. The
# ratchet sits under both rather than just under the higher one: pinning it to
# a single branch's figure makes the next task fail on its first commit for
# reasons that have nothing to do with its tests.
COVERAGE_MIN ?= 60.0

cover-check: ## Run the integration suite for coverage, then gate on it
	go test -tags=integration -coverprofile=coverage.out -covermode=atomic ./... > /dev/null
	$(MAKE) cover-gate

# cover-gate evaluates an existing coverage.out without re-running anything, so
# CI can produce the profile once during its integration step instead of paying
# for a third full pass through the suite.
#
# Generated code is filtered out first: everything under internal/generated, and
# every *.gen.go. That is the same set GENERATED_PATHS names, because it is the
# same idea — what the generators write, nobody writes tests for.
#
# `go test -coverprofile` instruments only the package under test, so generated
# code is pure denominator: nothing can ever cover it, and every module that
# adds queries or endpoints drags the ratchet down for a reason that says
# nothing about test quality. Both halves have been measured. The user module's
# first four sqlc query files moved the total from 46.9% to 43.8%; its four
# OpenAPI operations then tripled client.gen.go and took it to 44.9%. Neither
# had anything to do with how well the hand-written code was tested.
cover-gate: ## Fail if coverage.out is below COVERAGE_MIN
	@test -f coverage.out || (echo "no coverage.out; run make cover-check" && exit 1)
	@grep -v -e '/internal/generated/' -e '\.gen\.go:' coverage.out > coverage.handwritten.out; \
	 total=$$(go tool cover -func=coverage.handwritten.out | awk '/^total:/ {print $$3}' | tr -d '%'); \
	 rm -f coverage.handwritten.out; \
	 echo "total coverage: $$total% of hand-written code (minimum $(COVERAGE_MIN)%)"; \
	 awk -v t="$$total" -v m="$(COVERAGE_MIN)" 'BEGIN { exit !(t+0 >= m+0) }' \
	   || (echo "coverage $$total% is below the $(COVERAGE_MIN)% gate" && exit 1)

# GENERATED_PATHS is everything `make gen-backend` writes. The check is scoped to
# these rather than the whole tree so it reports codegen drift and not whatever
# else the working copy happens to have in flight.
GENERATED_PATHS := api/openapi/server.gen.go api/openapi/client.gen.go internal/generated

# gen-check is the staleness gate. It uses `git status --porcelain` rather than
# `git diff --exit-code` because codegen can create *new* files, and a new
# untracked file is invisible to git diff — which is how sqlc's output was never
# committed at all without anything noticing.
gen-check: gen-backend ## Fail if generated code is not what the generators produce
	@dirty=$$(git status --porcelain -- $(GENERATED_PATHS)); \
	 if [ -n "$$dirty" ]; then \
	   echo "generated code is stale: run make gen-backend and commit the result"; \
	   echo "$$dirty"; \
	   git diff --stat -- $(GENERATED_PATHS); \
	   exit 1; \
	 fi

## ----------------------------------------------------------------- docs

docs: ## Regenerate module documentation from the manifest
	node tools/docgen/generate.mjs

docs-check: ## Fail if documentation is stale or has drifted
	node tools/docgen/check-drift.mjs
	node tools/docgen/generate.mjs --check
	npx --yes markdownlint-cli2@0.20.0
# Link checking is not run here: lychee is a Rust binary, not an npm package, so
# `npx lychee` never worked. CI uses lycheeverse/lychee-action. To check links
# locally, install it (`cargo install lychee`) and run:
#   lychee --offline --no-progress '**/*.md'

## ----------------------------------------------------------------- ci

# `ci` mirrors both workflows, backend and frontend. It used to run only the Go
# half while claiming to run everything, which is how a stale web artefact got
# merged: the local gate said green and the frontend job said red.
ci: ci-backend ci-frontend ## Run exactly what CI runs, in the same order

ci-backend: ## The ci-backend.yml gates
	$(MAKE) fmt-check vet lint lint-int arch
	$(MAKE) gen-check
	$(MAKE) test
	$(MAKE) test-int
	$(MAKE) cover-check
	$(MAKE) docs-check

ci-frontend: ## The ci-frontend.yml gates
	$(MAKE) gen-check-web
	cd web && pnpm run typecheck
	cd web && pnpm run lint
	cd web && pnpm run test
	cd web && pnpm run build

ci-fast: fmt vet lint arch test ## Inner-loop subset

security: ## Security scans
	govulncheck ./...
	gitleaks detect --no-banner
	cd web && pnpm audit --audit-level=high

.PHONY: help setup dev dev-down logs prod-up api worker web gen gen-backend gen-sql gen-api gen-mocks gen-web \
        gen-check gen-check-web migrate-up migrate-down migrate-status migrate-new seed \
        db-reset-DANGEROUS check fmt fmt-check vet lint lint-int lint-go arch test test-int \
        test-contract test-web test-e2e test-load test-eval cover cover-check cover-gate docs \
        docs-check ci ci-backend ci-frontend ci-fast security
