# T024: Makefile and Build System

## Summary

Implements a standard Makefile for the Go tasker project with targets for building, testing, cross-compilation, and linting.

## Components

- `Makefile` - Build system with targets for development workflow

## Targets

| Target | Purpose |
|--------|---------|
| `build` | Compile binary for current platform to `./bin/tasker` |
| `test` | Run all tests with race detection and coverage reporting |
| `lint` | Run golangci-lint (auto-installs if not present) |
| `cross-compile` | Build for darwin/amd64, darwin/arm64, linux/amd64 |
| `clean` | Remove build artifacts |
| `help` | Display available targets |

## Build Configuration

The Makefile injects version information via ldflags:
- `version` - Git tag or commit hash
- `commit` - Short commit SHA
- `date` - Build timestamp (UTC)

## Usage

```bash
# Build for current platform
make build

# Run tests with coverage
make test

# Run linter
make lint

# Build for all platforms
make cross-compile

# Clean build artifacts
make clean
```

## Output Locations

- Single-platform binary: `./bin/tasker`
- Cross-compiled binaries: `./bin/tasker-{os}-{arch}`
- Coverage report: `./coverage.out`

## Dependencies

- Go 1.21+
- golangci-lint (auto-installed via `go install` if missing)

## Testing

```bash
# Verify build works
make build && ./bin/tasker --version

# Verify cross-compilation
make cross-compile && ls -la ./bin/

# Verify tests run
make test
```
