# CWSO Makefile — Docker-first build & test
# All toolchains run inside containers; no local Go/Rust required.

SHELL := /bin/bash
COMPOSE := docker compose -f deploy/docker-compose.yml

.PHONY: help build build-orchestrator build-git-shadow build-merge-engine \
	test test-go test-rust run stop logs inspector demo smoke-local clean lint fmt release-assets

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}'

build: build-orchestrator build-git-shadow build-merge-engine ## Build all images

build-orchestrator: ## Build Go orchestrator image
	docker build -t cwso/orchestrator:dev -f deploy/Dockerfile.orchestrator .

build-git-shadow: ## Build Rust git-shadow sidecar image
	docker build -t cwso/git-shadow:dev -f deploy/Dockerfile.git-shadow .

build-merge-engine: ## Build Rust merge-engine sidecar image
	docker build -t cwso/merge-engine:dev -f deploy/Dockerfile.merge-engine .

test: test-go test-rust ## Run all tests

test-go: ## Run Go test suite in container
	docker run --rm -v $$PWD/orchestrator:/src -w /src golang:1.23-alpine \
	  sh -c "apk add --no-cache git build-base && go test ./... -race -count=1"

test-rust: ## Run Rust test suites in container (placeholder until Phase 2)
	@echo "Rust tests run in Phase 2+"

run: ## docker compose up
	$(COMPOSE) up --build

stop: ## docker compose down
	$(COMPOSE) down

logs: ## Tail compose logs
	$(COMPOSE) logs -f

inspector: ## Launch mcp-inspector against running server
	docker run --rm -it --network host \
	  -e MCP_SERVER_URL=http://localhost:8080/mcp \
	  node:20-alpine sh -c "npx -y @modelcontextprotocol/inspector"

demo: ## End-to-end Phase 1 demo
	@echo "Running Phase 1 demo..."
	docker run --rm --network host cwso/orchestrator:dev /usr/local/bin/cwso-demo

smoke-local: ## Deterministic local smoke (build + phase2 integration + teardown)
	python3 scripts/phase2-integration.py

lint: ## Run linters
	docker run --rm -v $$PWD/orchestrator:/src -w /src golangci/golangci-lint:v1.62-alpine \
	  golangci-lint run --timeout=5m ./...

fmt: ## Format code
	docker run --rm -v $$PWD/orchestrator:/src -w /src golang:1.23-alpine \
	  sh -c "go fmt ./..."

clean: ## Remove build artifacts
	rm -rf bin/ target/ dist/
	docker image prune -f --filter "label=cwso=dev" || true

release-assets: ## Build and upload binaries/container archives to a release tag (use TAG=vX.Y.Z)
	@test -n "$(TAG)" || (echo "Usage: make release-assets TAG=vX.Y.Z" && exit 1)
	./scripts/release-assets.sh "$(TAG)"
