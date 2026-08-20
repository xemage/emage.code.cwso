# CWSO Makefile — Docker-first build & test
# All toolchains run inside containers; no local Go/Rust required.

SHELL := /bin/bash
COMPOSE := docker compose -f deploy/docker-compose.yml

.PHONY: help build build-orchestrator build-git-shadow build-merge-engine \
	test test-go test-rust run up stop down logs inspector demo smoke-local smoke clean lint fmt release-assets doctor \
	mcp-contract-snapshot mcp-contract-snapshot-update

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

mcp-contract-snapshot: ## C034: verify the live MCP surface matches the committed contract snapshot (same check CI runs)
	docker run --rm -v $$PWD/orchestrator:/src -w /src golang:1.25.13 \
	  sh -c "go test ./internal/server/... -run TestMCPContractSnapshot -v"

mcp-contract-snapshot-update: ## C034: DELIBERATELY regenerate testdata/mcp_contract_snapshot_v1.json from the live surface. Never run this to silence a failing check without reviewing the diff first -- a passing regen after an unreviewed surface change hides real protocol drift, it doesn't fix it.
	docker run --rm -v $$PWD/orchestrator:/src -w /src golang:1.25.13 \
	  sh -c "go test ./internal/server/... -run TestMCPContractSnapshot -update-snapshot -v"
	@echo "Regenerated orchestrator/internal/server/testdata/mcp_contract_snapshot_v1.json -- review the diff (git diff) before committing."

test-rust: ## Run Rust test suites in container (placeholder until Phase 2)
	@echo "Rust tests run in Phase 2+"

run: ## docker compose up
	$(COMPOSE) up --build

up: ## One command: bootstrap secrets -> build -> start -> wait for health -> mint token -> print MCP config (C016)
	@set -euo pipefail; \
	echo "==> [1/5] Bootstrapping secrets (scripts/cwso-bootstrap-secrets.sh)"; \
	if ! bash scripts/cwso-bootstrap-secrets.sh; then \
		echo "make up: FAILED at step 1/5 (bootstrap secrets) -- see output above" >&2; \
		exit 1; \
	fi; \
	echo "==> [2/5] Building images (docker compose build)"; \
	if ! $(COMPOSE) build; then \
		echo "make up: FAILED at step 2/5 (docker compose build) -- see output above" >&2; \
		exit 1; \
	fi; \
	echo "==> [3/5] Starting stack (docker compose up -d)"; \
	if ! $(COMPOSE) up -d; then \
		echo "make up: FAILED at step 3/5 (docker compose up -d) -- see output above" >&2; \
		echo "        run 'make down' to clean up any partially-started containers, then retry" >&2; \
		exit 1; \
	fi; \
	echo "==> [4/5] Waiting for http://127.0.0.1:8080/healthz (up to 120s)"; \
	healthy=""; \
	last_code=""; \
	deadline=$$((SECONDS + 120)); \
	while [ "$$SECONDS" -lt "$$deadline" ]; do \
		last_code="$$(curl -sS -o /dev/null -w '%{http_code}' --max-time 3 http://127.0.0.1:8080/healthz 2>/dev/null || true)"; \
		if [ "$$last_code" = "200" ]; then healthy=1; break; fi; \
		sleep 2; \
	done; \
	if [ -z "$$healthy" ]; then \
		echo "make up: FAILED at step 4/5 -- http://127.0.0.1:8080/healthz did not return 200 within 120s (last status: $${last_code:-no response})" >&2; \
		echo "---- last 50 lines of 'docker compose logs' ----" >&2; \
		$(COMPOSE) logs --tail=50 >&2 || true; \
		echo "        run 'make logs' for the full stream, or 'make down' to stop the stack" >&2; \
		exit 1; \
	fi; \
	echo "==> [5/5] Minting MCP token (scripts/cwso-token.sh)"; \
	token="$$(bash scripts/cwso-token.sh)"; \
	if [ -z "$$token" ]; then \
		echo "make up: FAILED at step 5/5 (token minting produced no output) -- see stderr above" >&2; \
		exit 1; \
	fi; \
	echo ""; \
	echo "CWSO stack is healthy and ready."; \
	echo ""; \
	echo "===== PASTE INTO YOUR MCP CLIENT ====="; \
	printf '%s\n' '{'; \
	printf '%s\n' '  "servers": {'; \
	printf '%s\n' '    "cwso": {'; \
	printf '%s\n' '      "type": "http",'; \
	printf '%s\n' '      "url": "http://127.0.0.1:8080/mcp",'; \
	printf '%s\n' '      "headers": {'; \
	printf '        "Authorization": "Bearer %s",\n' "$$token"; \
	printf '%s\n' '        "Origin": "http://localhost"'; \
	printf '%s\n' '      }'; \
	printf '%s\n' '    }'; \
	printf '%s\n' '  }'; \
	printf '%s\n' '}'; \
	echo "===== END ====="; \
	echo ""; \
	echo "Paste the block above into .vscode/mcp.json (VS Code) or .cursor/mcp.json (Cursor)."; \
	echo "The token expires per scripts/cwso-token.sh's default TTL; re-run 'make up' or 'scripts/cwso-token.sh' to mint a new one."

stop: ## docker compose down
	$(COMPOSE) down

down: stop ## Alias for 'stop' (docker compose down) -- symmetry with 'up' (C016)

logs: ## Tail compose logs
	$(COMPOSE) logs -f

doctor: ## Run pre-flight/post-flight diagnostics for the one-command stack
	@bash scripts/cwso-doctor.sh

inspector: ## Launch mcp-inspector against running server
	docker run --rm -it --network host \
	  -e MCP_SERVER_URL=http://localhost:8080/mcp \
	  node:20-alpine sh -c "npx -y @modelcontextprotocol/inspector"

demo: ## End-to-end Phase 1 demo
	@echo "Running Phase 1 demo..."
	docker run --rm --network host cwso/orchestrator:dev /usr/local/bin/cwso-demo

smoke-local: ## Deterministic local smoke (build + phase2 integration + teardown)
	python3 scripts/phase2-integration.py

smoke: up ## v1.0 definition-of-done: real MCP flow (shadow workspace -> write -> AST query -> commit -> merge) + teardown (C018)
	@bash scripts/cwso-smoke-test.sh

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
