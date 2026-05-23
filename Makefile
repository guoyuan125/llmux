.PHONY: build dev clean web reload

BINARY=bin/llmux
PID_FILE=data/llmux.pid

# Build the complete binary (backend + embedded frontend)
build: web
	CGO_ENABLED=1 go build -o $(BINARY) .

# Build backend only (for development)
dev:
	CGO_ENABLED=1 go build -o $(BINARY) .
	./$(BINARY) start

# Graceful reload: build new binary, then signal running process to upgrade.
# Existing connections (SSE streams) stay alive on the old process.
reload:
	CGO_ENABLED=1 go build -o $(BINARY) .
	@if [ -f $(PID_FILE) ]; then \
		kill -USR2 $$(cat $(PID_FILE)) 2>/dev/null && echo "reload signal sent" || echo "process not running, use 'make dev' to start"; \
	else \
		echo "no pid file, use 'make dev' to start"; \
	fi

# Build frontend
web:
	cd web && pnpm install && pnpm build
	rm -rf static/web
	cp -r web/out static/web

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf bin/ static/web

# Run with hot reload (requires air: go install github.com/air-verse/air@latest)
air:
	air

# Docker build
docker:
	docker build -t llmux:latest .

# Format code
fmt:
	go fmt ./...
	gofmt -s -w .
