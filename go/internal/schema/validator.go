package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/dgordon/tasker/internal/config"
	"github.com/dgordon/tasker/internal/errors"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

type SchemaName string

const (
	SchemaState              SchemaName = "state.schema.json"
	SchemaCapabilityMap      SchemaName = "capability-map.schema.json"
	SchemaPhysicalMap        SchemaName = "physical-map.schema.json"
	SchemaTask               SchemaName = "task.schema.json"
	SchemaTaskResult         SchemaName = "task-result.schema.json"
	SchemaExecutionBundle    SchemaName = "execution-bundle.schema.json"
	SchemaFSMStates          SchemaName = "fsm-states.schema.json"
	SchemaFSMTransitions     SchemaName = "fsm-transitions.schema.json"
	SchemaFSMIndex           SchemaName = "fsm-index.schema.json"
	SchemaSpecReview         SchemaName = "spec-review.schema.json"
	SchemaSpecResolutions    SchemaName = "spec-resolutions.schema.json"
	SchemaOrchestratorCkpt   SchemaName = "orchestrator-checkpoint.schema.json"
)

type Validator struct {
	schemaDir string
	compiler  *jsonschema.Compiler
	cache     map[SchemaName]*jsonschema.Schema
	mu        sync.RWMutex
}

var (
	defaultValidator *Validator
	validatorOnce    sync.Once
)

func Default() *Validator {
	validatorOnce.Do(func() {
		cfg := config.Get()
		schemaDir := cfg.ResolveSchemaDir()
		if schemaDir != "" {
			defaultValidator = New(schemaDir)
		}
	})
	return defaultValidator
}

func ResetDefault() {
	defaultValidator = nil
	validatorOnce = sync.Once{}
}

func New(schemaDir string) *Validator {
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft7

	return &Validator{
		schemaDir: schemaDir,
		compiler:  compiler,
		cache:     make(map[SchemaName]*jsonschema.Schema),
	}
}

func (v *Validator) SchemaDir() string {
	return v.schemaDir
}

func (v *Validator) getSchema(name SchemaName) (*jsonschema.Schema, error) {
	v.mu.RLock()
	if schema, ok := v.cache[name]; ok {
		v.mu.RUnlock()
		return schema, nil
	}
	v.mu.RUnlock()

	v.mu.Lock()
	defer v.mu.Unlock()

	if schema, ok := v.cache[name]; ok {
		return schema, nil
	}

	schemaPath := filepath.Join(v.schemaDir, string(name))
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		return nil, errors.SchemaNotFound(string(name))
	}

	schemaURL := "file://" + schemaPath
	schema, err := v.compiler.Compile(schemaURL)
	if err != nil {
		return nil, errors.SchemaCompileFailed(string(name), err)
	}

	v.cache[name] = schema
	return schema, nil
}

type ValidationError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

func (v *Validator) Validate(schemaName SchemaName, data interface{}) (*ValidationResult, error) {
	schema, err := v.getSchema(schemaName)
	if err != nil {
		return nil, err
	}

	var toValidate interface{}
	switch d := data.(type) {
	case []byte:
		if err := json.Unmarshal(d, &toValidate); err != nil {
			return nil, errors.Internal("failed to parse JSON for validation", err)
		}
	case string:
		if err := json.Unmarshal([]byte(d), &toValidate); err != nil {
			return nil, errors.Internal("failed to parse JSON for validation", err)
		}
	default:
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return nil, errors.Internal("failed to serialize data for validation", err)
		}
		if err := json.Unmarshal(jsonBytes, &toValidate); err != nil {
			return nil, errors.Internal("failed to parse serialized JSON", err)
		}
	}

	err = schema.Validate(toValidate)
	if err == nil {
		return &ValidationResult{Valid: true}, nil
	}

	validationErr, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return nil, errors.Internal("unexpected validation error type", err)
	}

	result := &ValidationResult{
		Valid:  false,
		Errors: extractValidationErrors(validationErr),
	}

	return result, nil
}

func extractValidationErrors(err *jsonschema.ValidationError) []ValidationError {
	var errs []ValidationError

	var extract func(ve *jsonschema.ValidationError, path string)
	extract = func(ve *jsonschema.ValidationError, path string) {
		currentPath := path
		if ve.InstanceLocation != "" {
			currentPath = ve.InstanceLocation
		}

		if ve.Message != "" {
			errs = append(errs, ValidationError{
				Path:    currentPath,
				Message: ve.Message,
			})
		}

		for _, cause := range ve.Causes {
			extract(cause, currentPath)
		}
	}

	extract(err, "")
	return errs
}

func (v *Validator) ValidateFile(schemaName SchemaName, filePath string) (*ValidationResult, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, errors.IOReadFailed(filePath, err)
	}

	return v.Validate(schemaName, data)
}

func (v *Validator) MustValidate(schemaName SchemaName, data interface{}) error {
	result, err := v.Validate(schemaName, data)
	if err != nil {
		return err
	}

	if !result.Valid {
		violations := make([]string, len(result.Errors))
		for i, e := range result.Errors {
			violations[i] = fmt.Sprintf("%s: %s", e.Path, e.Message)
		}
		return errors.SchemaValidationFailed(string(schemaName), violations)
	}

	return nil
}

func Validate(schemaName SchemaName, data interface{}) (*ValidationResult, error) {
	v := Default()
	if v == nil {
		return nil, errors.ConfigMissing("schema directory not configured")
	}
	return v.Validate(schemaName, data)
}

func ValidateFile(schemaName SchemaName, filePath string) (*ValidationResult, error) {
	v := Default()
	if v == nil {
		return nil, errors.ConfigMissing("schema directory not configured")
	}
	return v.ValidateFile(schemaName, filePath)
}

func MustValidate(schemaName SchemaName, data interface{}) error {
	v := Default()
	if v == nil {
		return errors.ConfigMissing("schema directory not configured")
	}
	return v.MustValidate(schemaName, data)
}
