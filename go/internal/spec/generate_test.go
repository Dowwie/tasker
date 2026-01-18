package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"My Feature!", "my-feature"},
		{"Test_Feature_Name", "test-feature-name"},
		{"Multiple   Spaces", "multiple-spaces"},
		{"CamelCase", "camelcase"},
		{"with-dashes", "with-dashes"},
		{"  Leading and Trailing  ", "leading-and-trailing"},
		{"Special@#$Characters", "specialcharacters"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := Slugify(tt.input)
			if result != tt.expected {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateSpec(t *testing.T) {
	t.Run("generates spec from session", func(t *testing.T) {
		tmpDir := t.TempDir()

		session := &Session{
			Topic:     "User Authentication",
			TargetDir: tmpDir,
			Phase:     "export",
			Scope: Scope{
				Goal:      "Enable secure user authentication",
				NonGoals:  []string{"OAuth integration", "SAML support"},
				DoneMeans: []string{"Users can log in with email/password", "Sessions are managed securely"},
			},
			Workflows:  "1. User enters credentials\n2. System validates\n3. Session created",
			Invariants: []string{"Passwords are never stored in plaintext", "Sessions expire after 24 hours"},
			Decisions: []Decision{
				{Decision: "Use bcrypt for password hashing", ADRID: "0001"},
				{Decision: "JWT for session tokens"},
			},
			ADRs: []string{"0001"},
			OpenQuestions: OpenQuestions{
				Blocking:    []string{},
				NonBlocking: []string{"Should we support remember me?"},
			},
		}

		result, err := GenerateSpec(session, GenerateOptions{})
		if err != nil {
			t.Fatalf("GenerateSpec failed: %v", err)
		}

		if result.Title != "User Authentication" {
			t.Errorf("expected title 'User Authentication', got %q", result.Title)
		}

		if result.Slug != "user-authentication" {
			t.Errorf("expected slug 'user-authentication', got %q", result.Slug)
		}

		if !strings.HasSuffix(result.OutputPath, "user-authentication.md") {
			t.Errorf("expected output path to end with 'user-authentication.md', got %q", result.OutputPath)
		}

		if _, err := os.Stat(result.OutputPath); os.IsNotExist(err) {
			t.Error("expected spec file to be created")
		}

		data, err := os.ReadFile(result.OutputPath)
		if err != nil {
			t.Fatalf("failed to read output file: %v", err)
		}

		content := string(data)
		if !strings.Contains(content, "# Spec: User Authentication") {
			t.Error("expected content to contain spec title")
		}
		if !strings.Contains(content, "Enable secure user authentication") {
			t.Error("expected content to contain goal")
		}
		if !strings.Contains(content, "OAuth integration") {
			t.Error("expected content to contain non-goals")
		}
		if !strings.Contains(content, "Use bcrypt for password hashing") {
			t.Error("expected content to contain decisions")
		}
	})

	t.Run("fails without session", func(t *testing.T) {
		_, err := GenerateSpec(nil, GenerateOptions{})
		if err == nil {
			t.Error("expected error for nil session")
		}
	})

	t.Run("fails without topic", func(t *testing.T) {
		session := &Session{}
		_, err := GenerateSpec(session, GenerateOptions{TargetDir: t.TempDir()})
		if err == nil {
			t.Error("expected error for empty topic")
		}
	})

	t.Run("refuses to overwrite without force", func(t *testing.T) {
		tmpDir := t.TempDir()
		specsDir := filepath.Join(tmpDir, "docs", "specs")
		if err := os.MkdirAll(specsDir, 0755); err != nil {
			t.Fatalf("failed to create specs dir: %v", err)
		}

		existingPath := filepath.Join(specsDir, "test-feature.md")
		if err := os.WriteFile(existingPath, []byte("existing"), 0644); err != nil {
			t.Fatalf("failed to create existing file: %v", err)
		}

		session := &Session{
			Topic:     "Test Feature",
			TargetDir: tmpDir,
		}

		_, err := GenerateSpec(session, GenerateOptions{})
		if err == nil {
			t.Error("expected error when file exists without force")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("expected 'already exists' error, got: %v", err)
		}
	})

	t.Run("overwrites with force option", func(t *testing.T) {
		tmpDir := t.TempDir()
		specsDir := filepath.Join(tmpDir, "docs", "specs")
		if err := os.MkdirAll(specsDir, 0755); err != nil {
			t.Fatalf("failed to create specs dir: %v", err)
		}

		existingPath := filepath.Join(specsDir, "test-feature.md")
		if err := os.WriteFile(existingPath, []byte("existing"), 0644); err != nil {
			t.Fatalf("failed to create existing file: %v", err)
		}

		session := &Session{
			Topic:     "Test Feature",
			TargetDir: tmpDir,
			Scope: Scope{
				Goal: "New content",
			},
		}

		result, err := GenerateSpec(session, GenerateOptions{Force: true})
		if err != nil {
			t.Fatalf("GenerateSpec with force failed: %v", err)
		}

		data, _ := os.ReadFile(result.OutputPath)
		if !strings.Contains(string(data), "New content") {
			t.Error("expected file to be overwritten with new content")
		}
	})
}

func TestGenerateSpecFormat(t *testing.T) {
	t.Run("includes all required sections", func(t *testing.T) {
		session := &Session{
			Topic: "Format Test",
			Scope: Scope{
				Goal:      "Test formatting",
				NonGoals:  []string{"Non goal 1"},
				DoneMeans: []string{"Done means 1"},
			},
			Workflows:    "Test workflow",
			Invariants:   []string{"Invariant 1"},
			Interfaces:   "Test interfaces",
			Architecture: "Test architecture",
			Decisions:    []Decision{{Decision: "Test decision"}},
			OpenQuestions: OpenQuestions{
				Blocking:    []string{"Blocking Q"},
				NonBlocking: []string{"Non-blocking Q"},
			},
			Handoff: map[string]string{
				"what_to_build": "Build something",
			},
			ADRs: []string{"0001"},
		}

		content, err := GenerateSpecContent(session)
		if err != nil {
			t.Fatalf("GenerateSpecContent failed: %v", err)
		}

		requiredSections := []string{
			"# Spec: Format Test",
			"## Related ADRs",
			"## Goal",
			"## Non-goals",
			"## Done means",
			"## Workflows",
			"## Invariants",
			"## Interfaces",
			"## Architecture sketch",
			"## Decisions",
			"## Open Questions",
			"### Blocking",
			"### Non-blocking",
			"## Agent Handoff",
			"## Artifacts",
		}

		for _, section := range requiredSections {
			if !strings.Contains(content, section) {
				t.Errorf("expected content to contain section %q", section)
			}
		}
	})

	t.Run("formats decisions as table", func(t *testing.T) {
		session := &Session{
			Topic: "Table Test",
			Decisions: []Decision{
				{Decision: "Decision 1", ADRID: "0001"},
				{Decision: "Decision 2"},
			},
		}

		content, err := GenerateSpecContent(session)
		if err != nil {
			t.Fatalf("GenerateSpecContent failed: %v", err)
		}

		if !strings.Contains(content, "| Decision | ADR |") {
			t.Error("expected decisions table header")
		}
		if !strings.Contains(content, "| Decision 1 | [ADR-0001]") {
			t.Error("expected decision with ADR link")
		}
		if !strings.Contains(content, "| Decision 2 | (inline) |") {
			t.Error("expected decision without ADR link")
		}
	})

	t.Run("handles empty session gracefully", func(t *testing.T) {
		session := &Session{
			Topic: "Empty Session Test",
		}

		content, err := GenerateSpecContent(session)
		if err != nil {
			t.Fatalf("GenerateSpecContent failed: %v", err)
		}

		if !strings.Contains(content, "(goal not defined)") {
			t.Error("expected placeholder for missing goal")
		}
		if !strings.Contains(content, "- (none specified)") {
			t.Error("expected placeholder for empty lists")
		}
		if !strings.Contains(content, "(to be defined)") {
			t.Error("expected placeholder for missing workflows")
		}
	})

	t.Run("links to related ADRs", func(t *testing.T) {
		session := &Session{
			Topic: "ADR Links Test",
			ADRs:  []string{"0001", "0002"},
		}

		content, err := GenerateSpecContent(session)
		if err != nil {
			t.Fatalf("GenerateSpecContent failed: %v", err)
		}

		if !strings.Contains(content, "[ADR-0001](../adrs/ADR-0001.md)") {
			t.Error("expected ADR 0001 link")
		}
		if !strings.Contains(content, "[ADR-0002](../adrs/ADR-0002.md)") {
			t.Error("expected ADR 0002 link")
		}
	})
}

func TestGenerateADR(t *testing.T) {
	t.Run("generates ADR from input", func(t *testing.T) {
		tmpDir := t.TempDir()

		input := ADRInput{
			Number:   1,
			Title:    "Use bcrypt for passwords",
			Context:  "We need to store user passwords securely.",
			Decision: "Use bcrypt with cost factor 12 for password hashing.",
			Alternatives: []Alternative{
				{Name: "Argon2", Reason: "More complex to configure"},
				{Name: "SHA-256", Reason: "Not designed for passwords"},
			},
			Consequences: []string{
				"Passwords are secure against rainbow tables",
				"Login may be slightly slower due to hashing",
			},
			AppliesTo: []SpecReference{
				{Slug: "user-authentication", Title: "User Authentication"},
			},
		}

		result, err := GenerateADR(input, GenerateOptions{TargetDir: tmpDir})
		if err != nil {
			t.Fatalf("GenerateADR failed: %v", err)
		}

		if result.Number != 1 {
			t.Errorf("expected number 1, got %d", result.Number)
		}

		if !strings.Contains(result.OutputPath, "ADR-0001-use-bcrypt-for-passwords.md") {
			t.Errorf("unexpected output path: %q", result.OutputPath)
		}

		data, err := os.ReadFile(result.OutputPath)
		if err != nil {
			t.Fatalf("failed to read output file: %v", err)
		}

		content := string(data)
		if !strings.Contains(content, "# ADR-0001: Use bcrypt for passwords") {
			t.Error("expected ADR title")
		}
		if !strings.Contains(content, "**Status:** Accepted") {
			t.Error("expected status")
		}
		if !strings.Contains(content, "## Context") {
			t.Error("expected context section")
		}
		if !strings.Contains(content, "Use bcrypt with cost factor 12") {
			t.Error("expected decision content")
		}
		if !strings.Contains(content, "| Argon2 | More complex to configure |") {
			t.Error("expected alternatives table")
		}
	})

	t.Run("fails without required fields", func(t *testing.T) {
		tmpDir := t.TempDir()

		_, err := GenerateADR(ADRInput{Number: 1}, GenerateOptions{TargetDir: tmpDir})
		if err == nil {
			t.Error("expected error for missing title")
		}

		_, err = GenerateADR(ADRInput{Number: 1, Title: "Test"}, GenerateOptions{TargetDir: tmpDir})
		if err == nil {
			t.Error("expected error for missing context")
		}

		_, err = GenerateADR(ADRInput{Number: 1, Title: "Test", Context: "Test"}, GenerateOptions{TargetDir: tmpDir})
		if err == nil {
			t.Error("expected error for missing decision")
		}
	})

	t.Run("formats ADR number with padding", func(t *testing.T) {
		input := ADRInput{
			Number:   42,
			Title:    "Test",
			Context:  "Test context",
			Decision: "Test decision",
		}

		content, err := GenerateADRContent(input)
		if err != nil {
			t.Fatalf("GenerateADRContent failed: %v", err)
		}

		if !strings.Contains(content, "# ADR-0042: Test") {
			t.Error("expected zero-padded ADR number")
		}
	})

	t.Run("includes related ADRs", func(t *testing.T) {
		input := ADRInput{
			Number:      2,
			Title:       "Test",
			Context:     "Context",
			Decision:    "Decision",
			Supersedes:  "0001",
			RelatedADRs: []string{"0003", "0004"},
		}

		content, err := GenerateADRContent(input)
		if err != nil {
			t.Fatalf("GenerateADRContent failed: %v", err)
		}

		if !strings.Contains(content, "Supersedes: ADR-0001") {
			t.Error("expected supersedes reference")
		}
		if !strings.Contains(content, "Related ADRs: ADR-0003, ADR-0004") {
			t.Error("expected related ADRs")
		}
	})
}

func TestGenerateSpecContentNilSession(t *testing.T) {
	_, err := GenerateSpecContent(nil)
	if err == nil {
		t.Error("expected error for nil session")
	}
}

func TestGenerateADRContentMissingFields(t *testing.T) {
	_, err := GenerateADRContent(ADRInput{})
	if err == nil {
		t.Error("expected error for missing fields")
	}
}

func TestGenerateADRRefusesOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	adrsDir := filepath.Join(tmpDir, "docs", "adrs")
	if err := os.MkdirAll(adrsDir, 0755); err != nil {
		t.Fatalf("failed to create adrs dir: %v", err)
	}

	existingPath := filepath.Join(adrsDir, "ADR-0001-test.md")
	if err := os.WriteFile(existingPath, []byte("existing"), 0644); err != nil {
		t.Fatalf("failed to create existing file: %v", err)
	}

	input := ADRInput{
		Number:   1,
		Title:    "Test",
		Context:  "Context",
		Decision: "Decision",
	}

	_, err := GenerateADR(input, GenerateOptions{TargetDir: tmpDir})
	if err == nil {
		t.Error("expected error when file exists without force")
	}
}

func TestGenerateADROverwriteWithForce(t *testing.T) {
	tmpDir := t.TempDir()
	adrsDir := filepath.Join(tmpDir, "docs", "adrs")
	if err := os.MkdirAll(adrsDir, 0755); err != nil {
		t.Fatalf("failed to create adrs dir: %v", err)
	}

	existingPath := filepath.Join(adrsDir, "ADR-0001-test.md")
	if err := os.WriteFile(existingPath, []byte("existing"), 0644); err != nil {
		t.Fatalf("failed to create existing file: %v", err)
	}

	input := ADRInput{
		Number:   1,
		Title:    "Test",
		Context:  "New context",
		Decision: "New decision",
	}

	result, err := GenerateADR(input, GenerateOptions{TargetDir: tmpDir, Force: true})
	if err != nil {
		t.Fatalf("GenerateADR with force failed: %v", err)
	}

	data, _ := os.ReadFile(result.OutputPath)
	if !strings.Contains(string(data), "New context") {
		t.Error("expected file to be overwritten with new content")
	}
}
