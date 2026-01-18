package errors

import (
	"fmt"
	"os"
	"strings"
)

type Category string

const (
	CategoryState      Category = "state"
	CategoryValidation Category = "validation"
	CategorySchema     Category = "schema"
	CategoryConfig     Category = "config"
	CategoryIO         Category = "io"
	CategoryInternal   Category = "internal"
)

type TaskerError struct {
	Category Category
	Code     string
	Message  string
	Wrapped  error
	Context  map[string]string
}

func (e *TaskerError) Error() string {
	if e.Wrapped != nil {
		return fmt.Sprintf("[%s:%s] %s: %v", e.Category, e.Code, e.Message, e.Wrapped)
	}
	return fmt.Sprintf("[%s:%s] %s", e.Category, e.Code, e.Message)
}

func (e *TaskerError) Unwrap() error {
	return e.Wrapped
}

func (e *TaskerError) WithContext(key, value string) *TaskerError {
	if e.Context == nil {
		e.Context = make(map[string]string)
	}
	e.Context[key] = value
	return e
}

func New(category Category, code, message string) *TaskerError {
	return &TaskerError{
		Category: category,
		Code:     code,
		Message:  message,
	}
}

func Wrap(category Category, code, message string, err error) *TaskerError {
	return &TaskerError{
		Category: category,
		Code:     code,
		Message:  message,
		Wrapped:  err,
	}
}

func StateNotFound(path string) *TaskerError {
	return New(CategoryState, "NOT_FOUND", fmt.Sprintf("state file not found: %s", path))
}

func StateCorrupt(path string, err error) *TaskerError {
	return Wrap(CategoryState, "CORRUPT", fmt.Sprintf("state file corrupt: %s", path), err)
}

func StateLocked(path string) *TaskerError {
	return New(CategoryState, "LOCKED", fmt.Sprintf("state file is locked: %s", path))
}

func StateWriteFailed(path string, err error) *TaskerError {
	return Wrap(CategoryState, "WRITE_FAILED", fmt.Sprintf("failed to write state: %s", path), err)
}

func ValidationFailed(message string) *TaskerError {
	return New(CategoryValidation, "FAILED", message)
}

func ValidationInvalidField(field, reason string) *TaskerError {
	return New(CategoryValidation, "INVALID_FIELD", fmt.Sprintf("invalid field '%s': %s", field, reason))
}

func SchemaNotFound(schemaName string) *TaskerError {
	return New(CategorySchema, "NOT_FOUND", fmt.Sprintf("schema not found: %s", schemaName))
}

func SchemaCompileFailed(schemaName string, err error) *TaskerError {
	return Wrap(CategorySchema, "COMPILE_FAILED", fmt.Sprintf("failed to compile schema: %s", schemaName), err)
}

func SchemaValidationFailed(schemaName string, violations []string) *TaskerError {
	return New(CategorySchema, "VALIDATION_FAILED",
		fmt.Sprintf("schema validation failed for %s: %s", schemaName, strings.Join(violations, "; ")))
}

func ConfigMissing(key string) *TaskerError {
	return New(CategoryConfig, "MISSING", fmt.Sprintf("required config missing: %s", key))
}

func ConfigInvalid(key, reason string) *TaskerError {
	return New(CategoryConfig, "INVALID", fmt.Sprintf("invalid config '%s': %s", key, reason))
}

func IOReadFailed(path string, err error) *TaskerError {
	return Wrap(CategoryIO, "READ_FAILED", fmt.Sprintf("failed to read: %s", path), err)
}

func IOWriteFailed(path string, err error) *TaskerError {
	return Wrap(CategoryIO, "WRITE_FAILED", fmt.Sprintf("failed to write: %s", path), err)
}

func IONotExists(path string) *TaskerError {
	return New(CategoryIO, "NOT_EXISTS", fmt.Sprintf("path does not exist: %s", path))
}

func Internal(message string, err error) *TaskerError {
	if err != nil {
		return Wrap(CategoryInternal, "ERROR", message, err)
	}
	return New(CategoryInternal, "ERROR", message)
}

func FormatForStderr(err error) string {
	if te, ok := err.(*TaskerError); ok {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("ERROR [%s:%s]\n", te.Category, te.Code))
		sb.WriteString(fmt.Sprintf("  Message: %s\n", te.Message))
		if te.Wrapped != nil {
			sb.WriteString(fmt.Sprintf("  Cause: %v\n", te.Wrapped))
		}
		if len(te.Context) > 0 {
			sb.WriteString("  Context:\n")
			for k, v := range te.Context {
				sb.WriteString(fmt.Sprintf("    %s: %s\n", k, v))
			}
		}
		return sb.String()
	}
	return fmt.Sprintf("ERROR: %v\n", err)
}

func PrintToStderr(err error) {
	fmt.Fprint(os.Stderr, FormatForStderr(err))
}

func IsCategory(err error, category Category) bool {
	if te, ok := err.(*TaskerError); ok {
		return te.Category == category
	}
	return false
}

func GetCode(err error) string {
	if te, ok := err.(*TaskerError); ok {
		return te.Code
	}
	return ""
}
