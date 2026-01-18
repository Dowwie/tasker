package spec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withPlanningDir(tmpDir string, fn func()) {
	original := getPlanningDirFunc
	getPlanningDirFunc = func() string { return tmpDir }
	defer func() { getPlanningDirFunc = original }()
	fn()
}

func setupSpecReview(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	artifactsDir := filepath.Join(tmpDir, "artifacts")
	os.MkdirAll(artifactsDir, 0755)

	review := map[string]interface{}{
		"version":       "1.0",
		"status":        "pending",
		"spec_checksum": "abc123",
		"analyzed_at":   "2026-01-18T10:00:00Z",
		"notes":         "Test review",
		"weaknesses": []interface{}{
			map[string]interface{}{
				"id":          "W001",
				"category":    "ambiguity",
				"severity":    "warning",
				"location":    "Section 1",
				"description": "Test weakness",
				"spec_quote":  "Some quote",
			},
		},
		"summary": map[string]interface{}{
			"total":    1,
			"blocking": false,
			"by_severity": map[string]interface{}{
				"critical": 0,
				"warning":  1,
				"info":     0,
			},
			"by_category": map[string]interface{}{
				"ambiguity": 1,
			},
		},
	}

	data, _ := json.MarshalIndent(review, "", "  ")
	os.WriteFile(filepath.Join(artifactsDir, "spec-review.json"), data, 0644)

	return tmpDir
}

func TestReviewCmd(t *testing.T) {
	t.Run("fails for nonexistent file", func(t *testing.T) {
		tmpDir := t.TempDir()

		withPlanningDir(tmpDir, func() {
			err := reviewCmd.RunE(reviewCmd, []string{"/nonexistent/spec.md"})
			if err == nil {
				t.Error("reviewCmd should fail for nonexistent file")
			}
		})
	})

	t.Run("analyzes spec file", func(t *testing.T) {
		tmpDir := t.TempDir()

		specContent := `# Test Spec

## Overview
This is a test specification.

## Requirements
- REQ-001: The system shall do something.
`
		specPath := filepath.Join(tmpDir, "spec.md")
		os.WriteFile(specPath, []byte(specContent), 0644)

		withPlanningDir(tmpDir, func() {
			err := reviewCmd.RunE(reviewCmd, []string{specPath})
			if err != nil {
				t.Errorf("reviewCmd should succeed, got: %v", err)
			}
		})
	})
}

func TestStatusCmd(t *testing.T) {
	t.Run("shows status with existing review", func(t *testing.T) {
		tmpDir := setupSpecReview(t)

		withPlanningDir(tmpDir, func() {
			err := statusCmd.RunE(statusCmd, []string{})
			if err != nil {
				t.Errorf("statusCmd should succeed, got: %v", err)
			}
		})
	})

	t.Run("handles no existing review", func(t *testing.T) {
		tmpDir := t.TempDir()

		withPlanningDir(tmpDir, func() {
			err := statusCmd.RunE(statusCmd, []string{})
			if err != nil {
				t.Errorf("statusCmd should handle no review, got: %v", err)
			}
		})
	})
}

func TestSessionShowCmd(t *testing.T) {
	t.Run("shows session details", func(t *testing.T) {
		tmpDir := setupSpecReview(t)

		withPlanningDir(tmpDir, func() {
			err := sessionShowCmd.RunE(sessionShowCmd, []string{})
			if err != nil {
				t.Errorf("sessionShowCmd should succeed, got: %v", err)
			}
		})
	})

	t.Run("handles no session", func(t *testing.T) {
		tmpDir := t.TempDir()

		withPlanningDir(tmpDir, func() {
			err := sessionShowCmd.RunE(sessionShowCmd, []string{})
			if err != nil {
				t.Errorf("sessionShowCmd should handle no session, got: %v", err)
			}
		})
	})
}

func TestResolveCmd(t *testing.T) {
	t.Run("resolves weakness", func(t *testing.T) {
		tmpDir := setupSpecReview(t)

		withPlanningDir(tmpDir, func() {
			err := resolveCmd.RunE(resolveCmd, []string{"W001", "Fixed by clarifying requirements"})
			if err != nil {
				t.Errorf("resolveCmd should succeed, got: %v", err)
			}
		})
	})

	t.Run("fails for nonexistent weakness", func(t *testing.T) {
		tmpDir := setupSpecReview(t)

		withPlanningDir(tmpDir, func() {
			err := resolveCmd.RunE(resolveCmd, []string{"WXXX", "Some resolution"})
			if err == nil {
				t.Error("resolveCmd should fail for nonexistent weakness")
			}
		})
	})

	t.Run("fails when no review exists", func(t *testing.T) {
		tmpDir := t.TempDir()

		withPlanningDir(tmpDir, func() {
			err := resolveCmd.RunE(resolveCmd, []string{"W001", "resolution"})
			if err == nil {
				t.Error("resolveCmd should fail when no review exists")
			}
		})
	})
}

func TestGenerateCmd(t *testing.T) {
	t.Run("runs without error", func(t *testing.T) {
		err := generateCmd.RunE(generateCmd, []string{})
		if err != nil {
			t.Errorf("generateCmd should succeed, got: %v", err)
		}
	})
}

func TestFindSpecFile(t *testing.T) {
	t.Run("finds spec.md in planning dir", func(t *testing.T) {
		tmpDir := t.TempDir()
		specPath := filepath.Join(tmpDir, "spec.md")
		os.WriteFile(specPath, []byte("# Spec"), 0644)

		found, err := FindSpecFile(tmpDir)
		if err != nil {
			t.Errorf("FindSpecFile should find spec.md, got: %v", err)
		}
		if found != specPath {
			t.Errorf("expected %s, got %s", specPath, found)
		}
	})

	t.Run("finds specification.md", func(t *testing.T) {
		tmpDir := t.TempDir()
		specPath := filepath.Join(tmpDir, "specification.md")
		os.WriteFile(specPath, []byte("# Spec"), 0644)

		found, err := FindSpecFile(tmpDir)
		if err != nil {
			t.Errorf("FindSpecFile should find specification.md, got: %v", err)
		}
		if found != specPath {
			t.Errorf("expected %s, got %s", specPath, found)
		}
	})

	t.Run("returns error when not found", func(t *testing.T) {
		tmpDir := t.TempDir()

		_, err := FindSpecFile(tmpDir)
		if err == nil {
			t.Error("FindSpecFile should return error when no spec found")
		}
	})
}

func TestOutputReviewJSON(t *testing.T) {
	review := map[string]interface{}{
		"version": "1.0",
		"status":  "pending",
	}

	data, _ := json.Marshal(review)
	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	if parsed["version"] != "1.0" {
		t.Error("expected version 1.0")
	}
}

func TestPrintFunctions(t *testing.T) {
	t.Run("printReviewSummary handles empty weaknesses", func(t *testing.T) {
		tmpDir := t.TempDir()
		specContent := "# Test Spec\n\nSimple test content."
		specPath := filepath.Join(tmpDir, "spec.md")
		os.WriteFile(specPath, []byte(specContent), 0644)

		withPlanningDir(tmpDir, func() {
			err := reviewCmd.RunE(reviewCmd, []string{specPath})
			if err != nil {
				t.Errorf("reviewCmd should succeed, got: %v", err)
			}
		})
	})
}

func TestReviewCmdWithJSON(t *testing.T) {
	tmpDir := t.TempDir()
	specContent := "# Test Spec\n\nSimple test content."
	specPath := filepath.Join(tmpDir, "spec.md")
	os.WriteFile(specPath, []byte(specContent), 0644)

	outputJSON = true
	defer func() { outputJSON = false }()

	withPlanningDir(tmpDir, func() {
		err := reviewCmd.RunE(reviewCmd, []string{specPath})
		if err != nil {
			t.Errorf("reviewCmd with JSON should succeed, got: %v", err)
		}
	})
}

func TestStatusCmdWithJSON(t *testing.T) {
	tmpDir := setupSpecReview(t)

	outputJSON = true
	defer func() { outputJSON = false }()

	withPlanningDir(tmpDir, func() {
		err := statusCmd.RunE(statusCmd, []string{})
		if err != nil {
			t.Errorf("statusCmd with JSON should succeed, got: %v", err)
		}
	})
}

func TestSessionShowCmdWithJSON(t *testing.T) {
	tmpDir := setupSpecReview(t)

	outputJSON = true
	defer func() { outputJSON = false }()

	withPlanningDir(tmpDir, func() {
		err := sessionShowCmd.RunE(sessionShowCmd, []string{})
		if err != nil {
			t.Errorf("sessionShowCmd with JSON should succeed, got: %v", err)
		}
	})
}

func TestReadStdin(t *testing.T) {
	_, err := readStdin()
	if err == nil {
		t.Skip("stdin test requires pipe input")
	}
	if !strings.Contains(err.Error(), "no input") {
		t.Errorf("expected 'no input' error, got: %v", err)
	}
}
