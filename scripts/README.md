# Test Scripts

This directory contains test and utility scripts for the provisr project.

## Integration Tests

### `simple-test.sh`
- **Main integration test script** - Recommended for CI/CD
- Quick and reliable tests covering core functionality
- Tests: build, error handling, API, process management, groups, CLI commands
- Auto-cleanup and colored output

### `test-integration.sh`
- **Detailed integration test script** - For thorough testing
- More comprehensive test cases with detailed logging
- Better debugging capabilities with server logs
- More robust error handling

## Usage

```bash
# Run quick integration tests (recommended)
./scripts/simple-test.sh

# Run detailed integration tests
./scripts/test-integration.sh

# Through Makefile
make test-integration  # Uses simple-test.sh
```

## What's Tested

✅ **Server startup and shutdown**
✅ **Error handling** (invalid configurations)
✅ **API endpoints** (`/api/status`, `/api/start`, etc.)
✅ **Process management** (start, stop, status)
✅ **Group operations** (group-start, group-stop, group-status)
✅ **CLI commands** (all group management commands)
✅ **Configuration validation**
✅ **Authentication** (login functionality)

## Requirements

- Go 1.21+
- `curl` command available
- Port 9999 available (simple-test.sh)
- Port 8081 available (test-integration.sh)

## Notes

- Both scripts include automatic cleanup
- Scripts can be run multiple times safely
- Test failures will exit with non-zero status for CI
- Temporary files are cleaned up on exit