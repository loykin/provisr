.PHONY: build test test-unit test-integration clean ui

# Build the provisr binary
build:
	go build -o provisr ./cmd/provisr

# Build the web UI and embed it into internal/ui/dist for go:embed.
# Run this and commit internal/ui/dist after any frontend change.
ui:
	cd frontend && npm run build
	rm -rf internal/ui/dist
	cp -r frontend/dist internal/ui/dist
	@echo "UI built. Commit internal/ui/dist/ to include it in the binary."

# Run all tests
test: test-unit test-integration

# Run unit tests
test-unit:
	go test -v ./...

# Run integration tests
test-integration: build
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