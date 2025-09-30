.PHONY: build test test-unit test-integration clean

# Build the provisr binary
build:
	go build -o provisr ./cmd/provisr

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