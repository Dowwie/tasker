package schema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dgordon/tasker/internal/config"
)

func createTestSchema(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create test schema: %v", err)
	}
}

func TestValidator(t *testing.T) {
	schemaDir := t.TempDir()

	stateSchema := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"required": ["version", "phase"],
		"properties": {
			"version": { "const": "2.0" },
			"phase": {
				"type": "object",
				"required": ["current"],
				"properties": {
					"current": { "type": "string" }
				}
			}
		}
	}`
	createTestSchema(t, schemaDir, "state.schema.json", stateSchema)

	v := New(schemaDir)

	t.Run("valid data passes", func(t *testing.T) {
		data := map[string]interface{}{
			"version": "2.0",
			"phase": map[string]interface{}{
				"current": "ready",
			},
		}

		result, err := v.Validate(SchemaState, data)
		if err != nil {
			t.Fatalf("Validate returned error: %v", err)
		}
		if !result.Valid {
			t.Errorf("expected valid, got invalid with errors: %v", result.Errors)
		}
	})

	t.Run("invalid version fails", func(t *testing.T) {
		data := map[string]interface{}{
			"version": "1.0",
			"phase": map[string]interface{}{
				"current": "ready",
			},
		}

		result, err := v.Validate(SchemaState, data)
		if err != nil {
			t.Fatalf("Validate returned error: %v", err)
		}
		if result.Valid {
			t.Error("expected invalid for wrong version")
		}
		if len(result.Errors) == 0 {
			t.Error("expected validation errors")
		}
	})

	t.Run("missing required field fails", func(t *testing.T) {
		data := map[string]interface{}{
			"version": "2.0",
		}

		result, err := v.Validate(SchemaState, data)
		if err != nil {
			t.Fatalf("Validate returned error: %v", err)
		}
		if result.Valid {
			t.Error("expected invalid for missing phase")
		}
	})

	t.Run("validates JSON bytes", func(t *testing.T) {
		jsonData := []byte(`{"version": "2.0", "phase": {"current": "ready"}}`)

		result, err := v.Validate(SchemaState, jsonData)
		if err != nil {
			t.Fatalf("Validate returned error: %v", err)
		}
		if !result.Valid {
			t.Errorf("expected valid for JSON bytes, got: %v", result.Errors)
		}
	})

	t.Run("validates JSON string", func(t *testing.T) {
		jsonStr := `{"version": "2.0", "phase": {"current": "ready"}}`

		result, err := v.Validate(SchemaState, jsonStr)
		if err != nil {
			t.Fatalf("Validate returned error: %v", err)
		}
		if !result.Valid {
			t.Errorf("expected valid for JSON string, got: %v", result.Errors)
		}
	})

	t.Run("schema caching", func(t *testing.T) {
		data := map[string]interface{}{
			"version": "2.0",
			"phase":   map[string]interface{}{"current": "ready"},
		}

		v.Validate(SchemaState, data)
		v.Validate(SchemaState, data)

		v.mu.RLock()
		cached, ok := v.cache[SchemaState]
		v.mu.RUnlock()

		if !ok || cached == nil {
			t.Error("expected schema to be cached")
		}
	})
}

func TestValidatorSchemaNotFound(t *testing.T) {
	schemaDir := t.TempDir()
	v := New(schemaDir)

	_, err := v.Validate("nonexistent.schema.json", map[string]interface{}{})
	if err == nil {
		t.Error("expected error for nonexistent schema")
	}
}

func TestValidateFile(t *testing.T) {
	schemaDir := t.TempDir()
	dataDir := t.TempDir()

	testSchema := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"required": ["name"],
		"properties": {
			"name": { "type": "string" }
		}
	}`
	createTestSchema(t, schemaDir, "test.schema.json", testSchema)

	v := New(schemaDir)

	t.Run("validates valid file", func(t *testing.T) {
		filePath := filepath.Join(dataDir, "valid.json")
		if err := os.WriteFile(filePath, []byte(`{"name": "test"}`), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		result, err := v.ValidateFile("test.schema.json", filePath)
		if err != nil {
			t.Fatalf("ValidateFile returned error: %v", err)
		}
		if !result.Valid {
			t.Errorf("expected valid, got: %v", result.Errors)
		}
	})

	t.Run("validates invalid file", func(t *testing.T) {
		filePath := filepath.Join(dataDir, "invalid.json")
		if err := os.WriteFile(filePath, []byte(`{"other": "field"}`), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		result, err := v.ValidateFile("test.schema.json", filePath)
		if err != nil {
			t.Fatalf("ValidateFile returned error: %v", err)
		}
		if result.Valid {
			t.Error("expected invalid")
		}
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		_, err := v.ValidateFile("test.schema.json", "/nonexistent/file.json")
		if err == nil {
			t.Error("expected error for missing file")
		}
	})
}

func TestMustValidate(t *testing.T) {
	schemaDir := t.TempDir()

	testSchema := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"required": ["id"],
		"properties": {
			"id": { "type": "string" }
		}
	}`
	createTestSchema(t, schemaDir, "test.schema.json", testSchema)

	v := New(schemaDir)

	t.Run("returns nil for valid data", func(t *testing.T) {
		data := map[string]interface{}{"id": "123"}
		err := v.MustValidate("test.schema.json", data)
		if err != nil {
			t.Errorf("expected nil error, got: %v", err)
		}
	})

	t.Run("returns error for invalid data", func(t *testing.T) {
		data := map[string]interface{}{"other": "field"}
		err := v.MustValidate("test.schema.json", data)
		if err == nil {
			t.Error("expected error for invalid data")
		}
	})
}

func TestSchemaDir(t *testing.T) {
	v := New("/some/schema/dir")
	if v.SchemaDir() != "/some/schema/dir" {
		t.Errorf("expected /some/schema/dir, got %s", v.SchemaDir())
	}
}

func TestValidationErrorFormat(t *testing.T) {
	schemaDir := t.TempDir()

	testSchema := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"required": ["name", "count"],
		"properties": {
			"name": { "type": "string" },
			"count": { "type": "integer", "minimum": 0 }
		}
	}`
	createTestSchema(t, schemaDir, "test.schema.json", testSchema)

	v := New(schemaDir)

	data := map[string]interface{}{
		"name":  123,
		"count": -1,
	}

	result, err := v.Validate("test.schema.json", data)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	if result.Valid {
		t.Error("expected invalid")
	}

	if len(result.Errors) == 0 {
		t.Error("expected validation errors")
	}

	for _, e := range result.Errors {
		if e.Message == "" {
			t.Error("validation error should have message")
		}
	}
}

func TestDefaultAndResetDefault(t *testing.T) {
	schemaDir := t.TempDir()
	testSchema := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": { "name": { "type": "string" } }
	}`
	createTestSchema(t, schemaDir, "test.schema.json", testSchema)

	config.Reset()
	ResetDefault()

	t.Setenv("TASKER_SCHEMA_DIR", schemaDir)

	v := Default()
	if v == nil {
		t.Fatal("expected Default() to return validator")
	}

	if v.SchemaDir() != schemaDir {
		t.Errorf("expected schema dir %s, got %s", schemaDir, v.SchemaDir())
	}

	v2 := Default()
	if v != v2 {
		t.Error("expected Default() to return same cached instance")
	}

	config.Reset()
	ResetDefault()
	v3 := Default()
	if v3 == nil {
		t.Fatal("expected Default() to return new validator after reset")
	}
}

func TestPackageLevelValidate(t *testing.T) {
	schemaDir := t.TempDir()
	testSchema := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"required": ["id"],
		"properties": { "id": { "type": "string" } }
	}`
	createTestSchema(t, schemaDir, "test.schema.json", testSchema)

	config.Reset()
	ResetDefault()
	t.Setenv("TASKER_SCHEMA_DIR", schemaDir)

	t.Run("Validate with default validator", func(t *testing.T) {
		data := map[string]interface{}{"id": "123"}
		result, err := Validate("test.schema.json", data)
		if err != nil {
			t.Fatalf("Validate error: %v", err)
		}
		if !result.Valid {
			t.Error("expected valid")
		}
	})

	t.Run("Validate returns nil validator error", func(t *testing.T) {
		config.Reset()
		ResetDefault()
		t.Setenv("TASKER_SCHEMA_DIR", "")

		_, err := Validate("test.schema.json", map[string]interface{}{})
		if err == nil {
			t.Error("expected error when default validator is nil")
		}
	})
}

func TestPackageLevelValidateFile(t *testing.T) {
	schemaDir := t.TempDir()
	dataDir := t.TempDir()

	testSchema := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"required": ["name"],
		"properties": { "name": { "type": "string" } }
	}`
	createTestSchema(t, schemaDir, "test.schema.json", testSchema)

	filePath := filepath.Join(dataDir, "valid.json")
	if err := os.WriteFile(filePath, []byte(`{"name": "test"}`), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	config.Reset()
	ResetDefault()
	t.Setenv("TASKER_SCHEMA_DIR", schemaDir)

	t.Run("ValidateFile with default validator", func(t *testing.T) {
		result, err := ValidateFile("test.schema.json", filePath)
		if err != nil {
			t.Fatalf("ValidateFile error: %v", err)
		}
		if !result.Valid {
			t.Error("expected valid")
		}
	})

	t.Run("ValidateFile returns nil validator error", func(t *testing.T) {
		config.Reset()
		ResetDefault()
		t.Setenv("TASKER_SCHEMA_DIR", "")

		_, err := ValidateFile("test.schema.json", filePath)
		if err == nil {
			t.Error("expected error when default validator is nil")
		}
	})
}

func TestPackageLevelMustValidate(t *testing.T) {
	schemaDir := t.TempDir()
	testSchema := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"required": ["id"],
		"properties": { "id": { "type": "string" } }
	}`
	createTestSchema(t, schemaDir, "test.schema.json", testSchema)

	config.Reset()
	ResetDefault()
	t.Setenv("TASKER_SCHEMA_DIR", schemaDir)

	t.Run("MustValidate with default validator", func(t *testing.T) {
		err := MustValidate("test.schema.json", map[string]interface{}{"id": "123"})
		if err != nil {
			t.Errorf("expected nil error, got: %v", err)
		}
	})

	t.Run("MustValidate returns nil validator error", func(t *testing.T) {
		config.Reset()
		ResetDefault()
		t.Setenv("TASKER_SCHEMA_DIR", "")

		err := MustValidate("test.schema.json", map[string]interface{}{})
		if err == nil {
			t.Error("expected error when default validator is nil")
		}
	})
}

func TestValidateInvalidJSON(t *testing.T) {
	schemaDir := t.TempDir()
	testSchema := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object"
	}`
	createTestSchema(t, schemaDir, "test.schema.json", testSchema)

	v := New(schemaDir)

	t.Run("invalid JSON bytes", func(t *testing.T) {
		_, err := v.Validate("test.schema.json", []byte(`{invalid json`))
		if err == nil {
			t.Error("expected error for invalid JSON bytes")
		}
	})

	t.Run("invalid JSON string", func(t *testing.T) {
		_, err := v.Validate("test.schema.json", `{not valid json`)
		if err == nil {
			t.Error("expected error for invalid JSON string")
		}
	})
}

func TestSchemaCompileError(t *testing.T) {
	schemaDir := t.TempDir()
	invalidSchema := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "invalid-type-that-does-not-exist"
	}`
	createTestSchema(t, schemaDir, "invalid.schema.json", invalidSchema)

	v := New(schemaDir)

	_, err := v.Validate("invalid.schema.json", map[string]interface{}{})
	if err == nil {
		t.Error("expected error for invalid schema")
	}
}
