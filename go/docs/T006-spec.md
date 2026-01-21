# T006: Cross-cutting concerns: logging, errors, config, schema validation

## Summary

Implements foundational cross-cutting concerns for the Go tasker CLI: structured logging with configurable levels, centralized error types with agent-parseable stderr output, environment-based configuration management, and JSON Schema validation using the same schema files as the Python implementation.

## Components

- `go/internal/logging/logger.go` - Structured logger with configurable levels (DEBUG, INFO, WARN, ERROR)
- `go/internal/logging/logger_test.go` - Unit tests for logging functionality
- `go/internal/errors/errors.go` - Centralized error types with category/code structure
- `go/internal/errors/errors_test.go` - Unit tests for error handling
- `go/internal/config/config.go` - Environment-based configuration with defaults
- `go/internal/config/config_test.go` - Unit tests for configuration
- `go/internal/schema/validator.go` - JSON Schema validation using santhosh-tekuri/jsonschema
- `go/internal/schema/validator_test.go` - Unit tests for schema validation

## API / Interface

### Logging

```go
// Package-level functions use default logger
logging.Debug("message %s", arg)
logging.Info("message %s", arg)
logging.Warn("message %s", arg)
logging.Error("message %s", arg)
logging.SetLevel(logging.LevelDebug)
logging.InitFromEnv()

// Custom logger
logger := logging.New(os.Stderr, logging.LevelInfo)
logger.SetPrefix("component")
logger.Info("message")
```

### Errors

```go
// Create structured errors
err := errors.StateNotFound("/path/state.json")
err := errors.ValidationFailed("missing required field")
err := errors.SchemaValidationFailed("state", []string{"error1", "error2"})

// Format for agent parsing
errors.PrintToStderr(err)
// Output:
// ERROR [state:NOT_FOUND]
//   Message: state file missing
//   Context:
//     path: /project/state.json

// Check error type
if errors.IsCategory(err, errors.CategoryState) { ... }
```

### Config

```go
// Get singleton config (loads from environment)
cfg := config.Get()

// Resolve paths with fallbacks
planningDir := cfg.ResolvePlanningDir()
schemaDir := cfg.ResolveSchemaDir()

// Validate required configuration
issues := cfg.ValidateRequired("planning_dir", "schema_dir")
```

Environment variables:
- `TASKER_TASKER_DIR` - Override planning directory path
- `TASKER_TARGET_DIR` - Override target directory path
- `TASKER_SCHEMA_DIR` - Override schema directory path
- `TASKER_LOG_LEVEL` - Set log level (DEBUG, INFO, WARN, ERROR)
- `TASKER_DEBUG` - Enable debug mode (sets log level to DEBUG)

### Schema Validation

```go
// Validate data against schema
result, err := schema.Validate(schema.SchemaState, stateData)
if !result.Valid {
    for _, e := range result.Errors {
        fmt.Printf("%s: %s\n", e.Path, e.Message)
    }
}

// Validate file directly
result, err := schema.ValidateFile(schema.SchemaState, "/path/to/state.json")

// Fail fast validation
err := schema.MustValidate(schema.SchemaState, data)
```

## Dependencies

- `github.com/santhosh-tekuri/jsonschema/v5` - JSON Schema validation (draft-07)

## Testing

```bash
go test ./go/internal/logging/... -run TestLogger -v
go test ./go/internal/errors/... -run TestErrors -v
go test ./go/internal/config/... -run TestConfig -v
go test ./go/internal/schema/... -run TestValidator -v
```

## Schema Compatibility

The schema validator uses JSON Schema draft-07 and reads schema files from the shared `schemas/` directory, ensuring the Go implementation validates against the same schemas as the Python implementation (INV-009).
