package spec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeSpec(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("detects non-behavioral requirements", func(t *testing.T) {
		specContent := `# Test Spec
The system must be fast and efficient.
All responses should be scalable.
`
		specPath := filepath.Join(tmpDir, "spec-non-behavioral.md")
		if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
			t.Fatalf("failed to create spec file: %v", err)
		}

		result, err := AnalyzeSpec(specPath)
		if err != nil {
			t.Fatalf("AnalyzeSpec failed: %v", err)
		}

		if result.Review == nil {
			t.Fatal("expected non-nil review")
		}

		found := false
		for _, w := range result.Review.Weaknesses {
			if w.Category == CategoryNonBehavioral {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to detect non-behavioral requirement")
		}
	})

	t.Run("detects implicit assumptions", func(t *testing.T) {
		specContent := `# Test Spec
Obviously, the user will be authenticated.
Clearly, errors should be handled.
`
		specPath := filepath.Join(tmpDir, "spec-implicit.md")
		if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
			t.Fatalf("failed to create spec file: %v", err)
		}

		result, err := AnalyzeSpec(specPath)
		if err != nil {
			t.Fatalf("AnalyzeSpec failed: %v", err)
		}

		found := false
		for _, w := range result.Review.Weaknesses {
			if w.Category == CategoryImplicit {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to detect implicit assumption")
		}
	})

	t.Run("detects missing acceptance criteria", func(t *testing.T) {
		specContent := `# Test Spec
The system should handle errors gracefully.
Error handling must be implemented.
`
		specPath := filepath.Join(tmpDir, "spec-missing-ac.md")
		if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
			t.Fatalf("failed to create spec file: %v", err)
		}

		result, err := AnalyzeSpec(specPath)
		if err != nil {
			t.Fatalf("AnalyzeSpec failed: %v", err)
		}

		criticalCount := result.Review.Summary.BySeverity["critical"]
		if criticalCount == 0 {
			t.Error("expected to detect critical weakness for missing AC")
		}

		if !result.Blocking {
			t.Error("expected result to be blocking when critical issues found")
		}
	})

	t.Run("detects cross-cutting concerns", func(t *testing.T) {
		specContent := `# Test Spec
All API calls require authentication.
Logging should capture all errors.
`
		specPath := filepath.Join(tmpDir, "spec-cross-cutting.md")
		if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
			t.Fatalf("failed to create spec file: %v", err)
		}

		result, err := AnalyzeSpec(specPath)
		if err != nil {
			t.Fatalf("AnalyzeSpec failed: %v", err)
		}

		found := false
		for _, w := range result.Review.Weaknesses {
			if w.Category == CategoryCrossCutting {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to detect cross-cutting concern")
		}
	})

	t.Run("computes correct summary", func(t *testing.T) {
		specContent := `# Test Spec
The system must be fast.
Obviously, users expect this.
Handle errors properly.
`
		specPath := filepath.Join(tmpDir, "spec-summary.md")
		if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
			t.Fatalf("failed to create spec file: %v", err)
		}

		result, err := AnalyzeSpec(specPath)
		if err != nil {
			t.Fatalf("AnalyzeSpec failed: %v", err)
		}

		if result.Review.Summary.Total != len(result.Review.Weaknesses) {
			t.Errorf("summary total %d doesn't match weaknesses count %d",
				result.Review.Summary.Total, len(result.Review.Weaknesses))
		}

		severitySum := result.Review.Summary.BySeverity["critical"] +
			result.Review.Summary.BySeverity["warning"] +
			result.Review.Summary.BySeverity["info"]
		if severitySum != result.Review.Summary.Total {
			t.Errorf("severity sum %d doesn't match total %d", severitySum, result.Review.Summary.Total)
		}
	})

	t.Run("generates valid checksum", func(t *testing.T) {
		specContent := `# Simple Spec
This is a test.
`
		specPath := filepath.Join(tmpDir, "spec-checksum.md")
		if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
			t.Fatalf("failed to create spec file: %v", err)
		}

		result, err := AnalyzeSpec(specPath)
		if err != nil {
			t.Fatalf("AnalyzeSpec failed: %v", err)
		}

		if len(result.Review.SpecChecksum) != 16 {
			t.Errorf("expected 16-char checksum, got %d chars", len(result.Review.SpecChecksum))
		}
	})

	t.Run("handles file not found", func(t *testing.T) {
		_, err := AnalyzeSpec("/nonexistent/path/spec.md")
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("generates weakness IDs in correct format", func(t *testing.T) {
		specContent := `# Test Spec
The system must be fast.
The system must be efficient.
`
		specPath := filepath.Join(tmpDir, "spec-ids.md")
		if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
			t.Fatalf("failed to create spec file: %v", err)
		}

		result, err := AnalyzeSpec(specPath)
		if err != nil {
			t.Fatalf("AnalyzeSpec failed: %v", err)
		}

		for _, w := range result.Review.Weaknesses {
			if !strings.HasPrefix(w.ID, "W") && !strings.HasPrefix(w.ID, "CK-") {
				t.Errorf("weakness ID %s doesn't start with W or CK-", w.ID)
			}
			if !strings.Contains(w.ID, "-") {
				t.Errorf("weakness ID %s doesn't contain hyphen", w.ID)
			}
		}
	})
}

func TestAnalyzeSpecContent(t *testing.T) {
	t.Run("analyzes content directly", func(t *testing.T) {
		content := []byte(`# Test Spec
The system must be scalable.
`)
		result, err := AnalyzeSpecContent(content, "test-source")
		if err != nil {
			t.Fatalf("AnalyzeSpecContent failed: %v", err)
		}

		if result.Review == nil {
			t.Fatal("expected non-nil review")
		}

		if !strings.Contains(result.Review.Notes, "test-source") {
			t.Error("expected notes to contain source name")
		}
	})
}

func TestGetReviewStatus(t *testing.T) {
	t.Run("returns pending when no review exists", func(t *testing.T) {
		tmpDir := t.TempDir()

		status, err := GetReviewStatus(tmpDir)
		if err != nil {
			t.Fatalf("GetReviewStatus failed: %v", err)
		}

		if status.Status != StatusPending {
			t.Errorf("expected pending status, got %s", status.Status)
		}

		if status.TotalIssues != 0 {
			t.Errorf("expected 0 issues, got %d", status.TotalIssues)
		}
	})

	t.Run("loads existing review status", func(t *testing.T) {
		tmpDir := t.TempDir()
		artifactsDir := filepath.Join(tmpDir, "artifacts")
		if err := os.MkdirAll(artifactsDir, 0755); err != nil {
			t.Fatalf("failed to create artifacts dir: %v", err)
		}

		review := SpecReview{
			Version:      "1.0",
			SpecChecksum: "abc123def456ghij",
			AnalyzedAt:   "2024-01-15T10:00:00Z",
			Status:       StatusInReview,
			Weaknesses: []Weakness{
				{ID: "W1-001", Category: CategoryNonBehavioral, Severity: SeverityWarning},
				{ID: "W4-001", Category: CategoryMissingAC, Severity: SeverityCritical},
			},
			Summary: Summary{
				Total:      2,
				BySeverity: map[string]int{"critical": 1, "warning": 1, "info": 0},
				ByCategory: map[string]int{"non_behavioral": 1, "missing_ac": 1},
				Blocking:   true,
			},
		}

		data, _ := json.Marshal(review)
		reviewPath := filepath.Join(artifactsDir, "spec-review.json")
		if err := os.WriteFile(reviewPath, data, 0644); err != nil {
			t.Fatalf("failed to write review file: %v", err)
		}

		status, err := GetReviewStatus(tmpDir)
		if err != nil {
			t.Fatalf("GetReviewStatus failed: %v", err)
		}

		if status.Status != StatusInReview {
			t.Errorf("expected in_review status, got %s", status.Status)
		}

		if status.TotalIssues != 2 {
			t.Errorf("expected 2 issues, got %d", status.TotalIssues)
		}

		if status.Critical != 1 {
			t.Errorf("expected 1 critical, got %d", status.Critical)
		}

		if status.Warnings != 1 {
			t.Errorf("expected 1 warning, got %d", status.Warnings)
		}

		if !status.Blocking {
			t.Error("expected blocking to be true")
		}

		if status.SpecChecksum != "abc123def456ghij" {
			t.Errorf("unexpected checksum: %s", status.SpecChecksum)
		}
	})

	t.Run("handles malformed review file", func(t *testing.T) {
		tmpDir := t.TempDir()
		artifactsDir := filepath.Join(tmpDir, "artifacts")
		if err := os.MkdirAll(artifactsDir, 0755); err != nil {
			t.Fatalf("failed to create artifacts dir: %v", err)
		}

		reviewPath := filepath.Join(artifactsDir, "spec-review.json")
		if err := os.WriteFile(reviewPath, []byte("not valid json"), 0644); err != nil {
			t.Fatalf("failed to write review file: %v", err)
		}

		_, err := GetReviewStatus(tmpDir)
		if err == nil {
			t.Error("expected error for malformed JSON")
		}
	})
}

func TestLoadReview(t *testing.T) {
	t.Run("loads complete review", func(t *testing.T) {
		tmpDir := t.TempDir()
		artifactsDir := filepath.Join(tmpDir, "artifacts")
		if err := os.MkdirAll(artifactsDir, 0755); err != nil {
			t.Fatalf("failed to create artifacts dir: %v", err)
		}

		review := SpecReview{
			Version:      "1.0",
			SpecChecksum: "test1234test5678",
			AnalyzedAt:   "2024-01-15T10:00:00Z",
			Status:       StatusResolved,
			Weaknesses:   []Weakness{},
			Summary: Summary{
				Total:      0,
				BySeverity: map[string]int{"critical": 0, "warning": 0, "info": 0},
				ByCategory: map[string]int{},
				Blocking:   false,
			},
		}

		data, _ := json.Marshal(review)
		reviewPath := filepath.Join(artifactsDir, "spec-review.json")
		if err := os.WriteFile(reviewPath, data, 0644); err != nil {
			t.Fatalf("failed to write review file: %v", err)
		}

		loaded, err := LoadReview(tmpDir)
		if err != nil {
			t.Fatalf("LoadReview failed: %v", err)
		}

		if loaded.Version != "1.0" {
			t.Errorf("expected version 1.0, got %s", loaded.Version)
		}

		if loaded.Status != StatusResolved {
			t.Errorf("expected resolved status, got %s", loaded.Status)
		}
	})

	t.Run("returns error for nonexistent review", func(t *testing.T) {
		tmpDir := t.TempDir()

		_, err := LoadReview(tmpDir)
		if err == nil {
			t.Error("expected error for nonexistent review")
		}
	})
}

func TestSaveReview(t *testing.T) {
	t.Run("saves review to artifacts directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		review := &SpecReview{
			Version:      "1.0",
			SpecChecksum: "save1234test5678",
			AnalyzedAt:   "2024-01-15T10:00:00Z",
			Status:       StatusPending,
			Weaknesses:   []Weakness{},
			Summary: Summary{
				Total:      0,
				BySeverity: map[string]int{"critical": 0, "warning": 0, "info": 0},
				ByCategory: map[string]int{},
				Blocking:   false,
			},
		}

		if err := SaveReview(tmpDir, review); err != nil {
			t.Fatalf("SaveReview failed: %v", err)
		}

		reviewPath := filepath.Join(tmpDir, "artifacts", "spec-review.json")
		if _, err := os.Stat(reviewPath); os.IsNotExist(err) {
			t.Error("expected review file to be created")
		}

		loaded, err := LoadReview(tmpDir)
		if err != nil {
			t.Fatalf("failed to load saved review: %v", err)
		}

		if loaded.SpecChecksum != review.SpecChecksum {
			t.Errorf("checksum mismatch: expected %s, got %s", review.SpecChecksum, loaded.SpecChecksum)
		}
	})

	t.Run("creates artifacts directory if needed", func(t *testing.T) {
		tmpDir := t.TempDir()

		review := &SpecReview{
			Version:      "1.0",
			SpecChecksum: "test",
			AnalyzedAt:   "2024-01-15T10:00:00Z",
			Status:       StatusPending,
			Weaknesses:   []Weakness{},
			Summary: Summary{
				Total:      0,
				BySeverity: map[string]int{"critical": 0, "warning": 0, "info": 0},
				ByCategory: map[string]int{},
				Blocking:   false,
			},
		}

		if err := SaveReview(tmpDir, review); err != nil {
			t.Fatalf("SaveReview failed: %v", err)
		}

		artifactsDir := filepath.Join(tmpDir, "artifacts")
		if _, err := os.Stat(artifactsDir); os.IsNotExist(err) {
			t.Error("expected artifacts directory to be created")
		}
	})
}

func TestResolveWeakness(t *testing.T) {
	t.Run("resolves existing weakness", func(t *testing.T) {
		review := &SpecReview{
			Version: "1.0",
			Status:  StatusInReview,
			Weaknesses: []Weakness{
				{ID: "W1-001", Category: CategoryNonBehavioral, Severity: SeverityWarning},
			},
			Summary: Summary{
				Total:      1,
				BySeverity: map[string]int{"critical": 0, "warning": 1, "info": 0},
			},
		}

		err := ResolveWeakness(review, "W1-001", "Added measurable SLA: 95th percentile < 200ms")
		if err != nil {
			t.Fatalf("ResolveWeakness failed: %v", err)
		}

		if review.Weaknesses[0].SuggestedRes == "" {
			t.Error("expected resolution to be set")
		}
	})

	t.Run("returns error for unknown weakness", func(t *testing.T) {
		review := &SpecReview{
			Version:    "1.0",
			Status:     StatusInReview,
			Weaknesses: []Weakness{},
		}

		err := ResolveWeakness(review, "W999-001", "some resolution")
		if err == nil {
			t.Error("expected error for unknown weakness")
		}
	})

	t.Run("updates status to resolved when all critical resolved", func(t *testing.T) {
		review := &SpecReview{
			Version: "1.0",
			Status:  StatusInReview,
			Weaknesses: []Weakness{
				{ID: "W4-001", Category: CategoryMissingAC, Severity: SeverityCritical},
			},
			Summary: Summary{
				Total:      1,
				BySeverity: map[string]int{"critical": 1, "warning": 0, "info": 0},
			},
		}

		err := ResolveWeakness(review, "W4-001", "Added acceptance criteria")
		if err != nil {
			t.Fatalf("ResolveWeakness failed: %v", err)
		}

		if review.Status != StatusResolved {
			t.Errorf("expected status to be resolved, got %s", review.Status)
		}
	})
}

func TestWeaknessCategorization(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected Category
	}{
		{
			name:     "non-behavioral",
			content:  "System must be fast",
			expected: CategoryNonBehavioral,
		},
		{
			name:     "implicit",
			content:  "Obviously users want this",
			expected: CategoryImplicit,
		},
		{
			name:     "cross-cutting",
			content:  "All requests require authentication",
			expected: CategoryCrossCutting,
		},
		{
			name:     "fragmented",
			content:  "See also the section below",
			expected: CategoryFragmented,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := AnalyzeSpecContent([]byte(tt.content), "test")
			if err != nil {
				t.Fatalf("AnalyzeSpecContent failed: %v", err)
			}

			found := false
			for _, w := range result.Review.Weaknesses {
				if w.Category == tt.expected {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("expected to find weakness of category %s", tt.expected)
			}
		})
	}
}

func TestCategoryIndex(t *testing.T) {
	t.Run("returns 0 for unknown category", func(t *testing.T) {
		idx := categoryIndex("unknown_category")
		if idx != 0 {
			t.Errorf("expected 0 for unknown category, got %d", idx)
		}
	})

	t.Run("returns correct index for known categories", func(t *testing.T) {
		if categoryIndex(CategoryNonBehavioral) != 1 {
			t.Error("expected non_behavioral to have index 1")
		}
	})
}

func TestDetectContradictions(t *testing.T) {
	t.Run("detects conflicting default values", func(t *testing.T) {
		// Pattern captures first word of line as the "entity" being described
		content := `# Config Spec
TIMEOUT defaults to 30 seconds.
TIMEOUT defaults to 60 seconds.
`
		weaknesses := detectContradictions(content)

		if len(weaknesses) == 0 {
			t.Error("expected to detect conflicting default values")
			return
		}

		found := false
		for _, w := range weaknesses {
			if w.Category == CategoryContradiction && w.Severity == SeverityCritical {
				found = true
				if !strings.Contains(strings.ToLower(w.Description), "timeout") {
					t.Errorf("expected description to mention 'timeout', got: %s", w.Description)
				}
				if !strings.HasPrefix(w.ID, "W6-") {
					t.Errorf("expected ID to start with W6-, got: %s", w.ID)
				}
			}
		}

		if !found {
			t.Error("expected to find a critical contradiction weakness")
		}
	})

	t.Run("ignores consistent default values", func(t *testing.T) {
		// Both lines use "defaults to 30" - same value, no contradiction
		content := `# Config Spec
TIMEOUT defaults to 30 seconds.
TIMEOUT defaults to 30 (configurable).
`
		weaknesses := detectContradictions(content)

		// Should not find any contradictions since both values are "30"
		for _, w := range weaknesses {
			if w.Category == CategoryContradiction && strings.Contains(strings.ToLower(w.Description), "timeout") {
				t.Error("should not flag consistent default values as contradictions")
			}
		}
	})

	t.Run("detects multiple conflicting variables", func(t *testing.T) {
		content := `# Config Spec
TIMEOUT defaults to 30 seconds.
TIMEOUT defaults to 60 seconds.
RETRIES default to 3.
RETRIES defaults to 5.
`
		weaknesses := detectContradictions(content)

		if len(weaknesses) < 2 {
			t.Errorf("expected at least 2 contradictions, got %d", len(weaknesses))
		}
	})

	t.Run("integration with AnalyzeSpec", func(t *testing.T) {
		tmpDir := t.TempDir()
		specContent := `# Config Spec
TIMEOUT defaults to 30 seconds.
TIMEOUT defaults to 60 seconds.
`
		specPath := filepath.Join(tmpDir, "spec-contradictions.md")
		if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
			t.Fatalf("failed to create spec file: %v", err)
		}

		result, err := AnalyzeSpec(specPath)
		if err != nil {
			t.Fatalf("AnalyzeSpec failed: %v", err)
		}

		found := false
		for _, w := range result.Review.Weaknesses {
			if w.Category == CategoryContradiction {
				found = true
				break
			}
		}

		if !found {
			t.Error("expected AnalyzeSpec to include contradiction detection")
		}
	})
}

func TestLoadReviewInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	artifactsDir := filepath.Join(tmpDir, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		t.Fatalf("failed to create artifacts dir: %v", err)
	}

	reviewPath := filepath.Join(artifactsDir, "spec-review.json")
	os.WriteFile(reviewPath, []byte("{invalid json}"), 0644)

	_, err := LoadReview(tmpDir)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
