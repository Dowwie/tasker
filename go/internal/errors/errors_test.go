package errors

import (
	"errors"
	"strings"
	"testing"
)

func TestErrors(t *testing.T) {
	t.Run("basic error creation", func(t *testing.T) {
		err := New(CategoryState, "TEST", "test message")
		if err.Category != CategoryState {
			t.Errorf("expected category state, got %s", err.Category)
		}
		if err.Code != "TEST" {
			t.Errorf("expected code TEST, got %s", err.Code)
		}
		if err.Message != "test message" {
			t.Errorf("expected message 'test message', got %s", err.Message)
		}
	})

	t.Run("error string format", func(t *testing.T) {
		err := New(CategoryValidation, "INVALID", "field is invalid")
		expected := "[validation:INVALID] field is invalid"
		if err.Error() != expected {
			t.Errorf("expected '%s', got '%s'", expected, err.Error())
		}
	})

	t.Run("wrapped error", func(t *testing.T) {
		cause := errors.New("underlying error")
		err := Wrap(CategoryIO, "READ", "failed to read file", cause)

		if !strings.Contains(err.Error(), "underlying error") {
			t.Error("wrapped error should include cause")
		}

		unwrapped := err.Unwrap()
		if unwrapped != cause {
			t.Error("Unwrap should return the cause")
		}
	})

	t.Run("context support", func(t *testing.T) {
		err := New(CategoryState, "LOCKED", "file locked").
			WithContext("path", "/some/path").
			WithContext("holder", "process-123")

		if err.Context["path"] != "/some/path" {
			t.Error("expected path context")
		}
		if err.Context["holder"] != "process-123" {
			t.Error("expected holder context")
		}
	})

	t.Run("parseable stderr output", func(t *testing.T) {
		err := New(CategoryState, "NOT_FOUND", "state file missing").
			WithContext("path", "/project/state.json")

		output := FormatForStderr(err)

		if !strings.Contains(output, "ERROR [state:NOT_FOUND]") {
			t.Errorf("expected error prefix, got: %s", output)
		}
		if !strings.Contains(output, "Message: state file missing") {
			t.Errorf("expected message line, got: %s", output)
		}
		if !strings.Contains(output, "path: /project/state.json") {
			t.Errorf("expected context, got: %s", output)
		}
	})

	t.Run("non-TaskerError formatting", func(t *testing.T) {
		err := errors.New("regular error")
		output := FormatForStderr(err)

		if !strings.HasPrefix(output, "ERROR: ") {
			t.Errorf("expected ERROR: prefix, got: %s", output)
		}
	})
}

func TestStateErrors(t *testing.T) {
	t.Run("StateNotFound", func(t *testing.T) {
		err := StateNotFound("/path/state.json")
		if err.Category != CategoryState {
			t.Error("expected state category")
		}
		if err.Code != "NOT_FOUND" {
			t.Error("expected NOT_FOUND code")
		}
		if !strings.Contains(err.Message, "/path/state.json") {
			t.Error("expected path in message")
		}
	})

	t.Run("StateCorrupt", func(t *testing.T) {
		cause := errors.New("invalid json")
		err := StateCorrupt("/path/state.json", cause)
		if err.Code != "CORRUPT" {
			t.Error("expected CORRUPT code")
		}
		if err.Unwrap() != cause {
			t.Error("expected wrapped cause")
		}
	})

	t.Run("StateLocked", func(t *testing.T) {
		err := StateLocked("/path/state.json")
		if err.Code != "LOCKED" {
			t.Error("expected LOCKED code")
		}
	})

	t.Run("StateWriteFailed", func(t *testing.T) {
		cause := errors.New("disk full")
		err := StateWriteFailed("/path/state.json", cause)
		if err.Code != "WRITE_FAILED" {
			t.Error("expected WRITE_FAILED code")
		}
	})
}

func TestValidationErrors(t *testing.T) {
	t.Run("ValidationFailed", func(t *testing.T) {
		err := ValidationFailed("missing required field")
		if err.Category != CategoryValidation {
			t.Error("expected validation category")
		}
	})

	t.Run("ValidationInvalidField", func(t *testing.T) {
		err := ValidationInvalidField("version", "must be 2.0")
		if !strings.Contains(err.Message, "version") {
			t.Error("expected field name in message")
		}
		if !strings.Contains(err.Message, "must be 2.0") {
			t.Error("expected reason in message")
		}
	})
}

func TestSchemaErrors(t *testing.T) {
	t.Run("SchemaNotFound", func(t *testing.T) {
		err := SchemaNotFound("state.schema.json")
		if err.Category != CategorySchema {
			t.Error("expected schema category")
		}
		if err.Code != "NOT_FOUND" {
			t.Error("expected NOT_FOUND code")
		}
	})

	t.Run("SchemaValidationFailed", func(t *testing.T) {
		err := SchemaValidationFailed("state", []string{"missing version", "invalid phase"})
		if !strings.Contains(err.Message, "missing version") {
			t.Error("expected first violation")
		}
		if !strings.Contains(err.Message, "invalid phase") {
			t.Error("expected second violation")
		}
	})
}

func TestConfigErrors(t *testing.T) {
	t.Run("ConfigMissing", func(t *testing.T) {
		err := ConfigMissing("TASKER_DIR")
		if err.Category != CategoryConfig {
			t.Error("expected config category")
		}
		if !strings.Contains(err.Message, "TASKER_DIR") {
			t.Error("expected key in message")
		}
	})

	t.Run("ConfigInvalid", func(t *testing.T) {
		err := ConfigInvalid("LOG_LEVEL", "must be DEBUG, INFO, WARN, or ERROR")
		if err.Code != "INVALID" {
			t.Error("expected INVALID code")
		}
	})
}

func TestIOErrors(t *testing.T) {
	t.Run("IOReadFailed", func(t *testing.T) {
		cause := errors.New("permission denied")
		err := IOReadFailed("/path/file", cause)
		if err.Category != CategoryIO {
			t.Error("expected io category")
		}
		if err.Unwrap() != cause {
			t.Error("expected wrapped cause")
		}
	})

	t.Run("IONotExists", func(t *testing.T) {
		err := IONotExists("/missing/path")
		if err.Code != "NOT_EXISTS" {
			t.Error("expected NOT_EXISTS code")
		}
	})
}

func TestIsCategory(t *testing.T) {
	stateErr := StateNotFound("/path")
	if !IsCategory(stateErr, CategoryState) {
		t.Error("expected IsCategory to return true for state error")
	}
	if IsCategory(stateErr, CategoryValidation) {
		t.Error("expected IsCategory to return false for different category")
	}

	regularErr := errors.New("regular")
	if IsCategory(regularErr, CategoryState) {
		t.Error("expected IsCategory to return false for non-TaskerError")
	}
}

func TestGetCode(t *testing.T) {
	err := StateNotFound("/path")
	if GetCode(err) != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND, got %s", GetCode(err))
	}

	regularErr := errors.New("regular")
	if GetCode(regularErr) != "" {
		t.Error("expected empty code for non-TaskerError")
	}
}

func TestInternal(t *testing.T) {
	t.Run("with cause", func(t *testing.T) {
		cause := errors.New("panic")
		err := Internal("unexpected state", cause)
		if err.Category != CategoryInternal {
			t.Error("expected internal category")
		}
		if err.Unwrap() != cause {
			t.Error("expected wrapped cause")
		}
	})

	t.Run("without cause", func(t *testing.T) {
		err := Internal("invariant violated", nil)
		if err.Wrapped != nil {
			t.Error("expected nil wrapped error")
		}
	})
}
