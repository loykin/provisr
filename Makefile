.PHONY: build build-frontend build-backend test test-unit test-integration clean ui

# Build the web UI and copy it into internal/ui/dist for go:embed. Always
# runs as part of `build` so the binary never silently embeds a stale UI.
build-frontend:
	cd frontend && npm run build
	rm -rf internal/ui/dist
	cp -r frontend/dist internal/ui/dist

# Build the provisr binary only. Assumes internal/ui/dist is already
# current (either just built by build-frontend, or committed as-is) —
# use this directly for a fast Go-only edit/test loop.
build-backend:
	go build -o provisr ./cmd/provisr

# Build frontend then backend, so the binary always embeds the current UI.
build: build-frontend build-backend

# Refresh the embedded UI assets after a frontend change.
ui: build-frontend
	@echo "UI built. Commit internal/ui/dist/ to include it in the binary."

# Run all tests
test: test-unit test-integration

# Run unit tests
test-unit:
	go test -v ./...

# Run integration tests
test-integration: build-backend
	./scripts/simple-test.sh

# Clean up build artifacts and test files
clean:
	rm -f provisr
	rm -rf run/ provisr-logs/ programs/ test-*.toml auth.db provisr.pid
	pkill -f "provisr serve" 2>/dev/null || true

# Install dependencies
deps:
	go mod tidy
	go mod download

# Development setup
dev-setup: deps build
	@echo "Development environment ready!"

# CI target - runs all checks
ci: deps test
	@echo "All CI checks passed!"
