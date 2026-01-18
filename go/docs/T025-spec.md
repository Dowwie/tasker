# T025: CI/CD Pipeline

## Summary

Implements a GitHub Actions CI pipeline for automated testing and linting on push to main and pull requests.

## Components

- `.github/workflows/ci.yml` - CI pipeline configuration

## Workflow Jobs

| Job | Purpose |
|-----|---------|
| `test` | Run tests with race detection on Go 1.21 and 1.22 |
| `lint` | Run golangci-lint on all Go code |
| `build` | Build binary to verify compilation succeeds |

## Triggers

- Push to `main` branch
- Pull requests targeting `main` branch

## Go Version Matrix

Tests run on multiple Go versions for compatibility:
- Go 1.21 (minimum supported)
- Go 1.22 (latest stable)

## Pipeline Steps

### Test Job
1. Checkout code
2. Set up Go (matrix version)
3. Download dependencies
4. Run tests with race detection and coverage
5. Upload coverage to Codecov (Go 1.22 only)

### Lint Job
1. Checkout code
2. Set up Go 1.22
3. Run golangci-lint via official action

### Build Job
1. Depends on test and lint passing
2. Checkout code
3. Set up Go 1.22
4. Build binary using Makefile
5. Verify binary runs with `--version`

## Dependencies

- T024: Makefile for `make build` target
- golangci-lint (installed by GitHub Action)

## Testing

```bash
# Verify workflow syntax
cat .github/workflows/ci.yml | head -20

# Verify triggers
grep -A5 "^on:" .github/workflows/ci.yml

# Verify Go versions
grep "1.21" .github/workflows/ci.yml
```
