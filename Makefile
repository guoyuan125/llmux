.PHONY: build dev clean web

# Build the complete binary (backend + embedded frontend)
build: web
	CGO_ENABLED=1 go build -o bin/llmux .

# Build backend only (for development)
dev:
	CGO_ENABLED=1 go build -o bin/llmux .
	./bin/llmux start

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
