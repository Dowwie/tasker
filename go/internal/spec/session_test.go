package spec

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManageSession(t *testing.T) {
	t.Run("initializes new session", func(t *testing.T) {
		tmpDir := t.TempDir()

		session, err := InitSession(tmpDir, "Test Feature", "")
		if err != nil {
			t.Fatalf("InitSession failed: %v", err)
		}

		if session.Topic != "Test Feature" {
			t.Errorf("expected topic 'Test Feature', got '%s'", session.Topic)
		}

		if session.Phase != "scope" {
			t.Errorf("expected phase 'scope', got '%s'", session.Phase)
		}

		if session.TargetDir != tmpDir {
			t.Errorf("expected target dir '%s', got '%s'", tmpDir, session.TargetDir)
		}

		if session.StartedAt == "" {
			t.Error("expected StartedAt to be set")
		}

		discoveryPath := filepath.Join(tmpDir, DiscoveryFile)
		if _, err := os.Stat(discoveryPath); os.IsNotExist(err) {
			t.Error("expected discovery file to be created")
		}

		sessionPath := filepath.Join(tmpDir, SessionFile)
		if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
			t.Error("expected session file to be created")
		}
	})

	t.Run("initializes session with custom target dir", func(t *testing.T) {
		tmpDir := t.TempDir()
		targetDir := t.TempDir()

		session, err := InitSession(tmpDir, "Test Feature", targetDir)
		if err != nil {
			t.Fatalf("InitSession failed: %v", err)
		}

		if session.TargetDir != targetDir {
			t.Errorf("expected target dir '%s', got '%s'", targetDir, session.TargetDir)
		}
	})

	t.Run("saves and loads session", func(t *testing.T) {
		tmpDir := t.TempDir()

		session := &Session{
			Topic:     "Persistence Test",
			TargetDir: tmpDir,
			StartedAt: "2025-01-15T10:00:00Z",
			Phase:     "clarify",
			Rounds:    3,
			Scope: Scope{
				Goal:      "Build something",
				NonGoals:  []string{"Not this"},
				DoneMeans: []string{"Tests pass"},
			},
			OpenQuestions: OpenQuestions{
				Blocking:    []string{"What about X?"},
				NonBlocking: []string{"Nice to know Y"},
			},
			Decisions: []Decision{
				{Decision: "Use Go", ADRID: "ADR-001", DecidedAt: "2025-01-15T10:30:00Z"},
			},
			ADRs: []string{"ADR-001"},
		}

		if err := SaveSessionToDir(tmpDir, session); err != nil {
			t.Fatalf("SaveSessionToDir failed: %v", err)
		}

		loaded, err := LoadSessionFromBaseDir(tmpDir)
		if err != nil {
			t.Fatalf("LoadSessionFromBaseDir failed: %v", err)
		}

		if loaded == nil {
			t.Fatal("expected non-nil session")
		}

		if loaded.Topic != session.Topic {
			t.Errorf("topic mismatch: expected '%s', got '%s'", session.Topic, loaded.Topic)
		}

		if loaded.Phase != session.Phase {
			t.Errorf("phase mismatch: expected '%s', got '%s'", session.Phase, loaded.Phase)
		}

		if loaded.Rounds != session.Rounds {
			t.Errorf("rounds mismatch: expected %d, got %d", session.Rounds, loaded.Rounds)
		}

		if loaded.Scope.Goal != session.Scope.Goal {
			t.Errorf("goal mismatch: expected '%s', got '%s'", session.Scope.Goal, loaded.Scope.Goal)
		}

		if len(loaded.OpenQuestions.Blocking) != 1 {
			t.Errorf("expected 1 blocking question, got %d", len(loaded.OpenQuestions.Blocking))
		}

		if len(loaded.Decisions) != 1 {
			t.Errorf("expected 1 decision, got %d", len(loaded.Decisions))
		}
	})

	t.Run("returns nil for nonexistent session", func(t *testing.T) {
		tmpDir := t.TempDir()

		session, err := LoadSessionFromBaseDir(tmpDir)
		if err != nil {
			t.Fatalf("LoadSessionFromBaseDir failed: %v", err)
		}

		if session != nil {
			t.Error("expected nil session for nonexistent file")
		}
	})
}

func TestSessionTracking(t *testing.T) {
	t.Run("tracks discovery rounds", func(t *testing.T) {
		tmpDir := t.TempDir()

		session, err := InitSession(tmpDir, "Round Tracking", "")
		if err != nil {
			t.Fatalf("InitSession failed: %v", err)
		}

		if session.Rounds != 0 {
			t.Errorf("expected 0 rounds initially, got %d", session.Rounds)
		}

		IncrementRound(session)
		if session.Rounds != 1 {
			t.Errorf("expected 1 round after increment, got %d", session.Rounds)
		}

		IncrementRound(session)
		IncrementRound(session)
		if session.Rounds != 3 {
			t.Errorf("expected 3 rounds, got %d", session.Rounds)
		}
	})

	t.Run("tracks clarifications via open questions", func(t *testing.T) {
		session := &Session{
			Topic: "Question Tracking",
			OpenQuestions: OpenQuestions{
				Blocking:    []string{},
				NonBlocking: []string{},
			},
		}

		AddOpenQuestion(session, "What about security?", true)
		if len(session.OpenQuestions.Blocking) != 1 {
			t.Errorf("expected 1 blocking question, got %d", len(session.OpenQuestions.Blocking))
		}

		AddOpenQuestion(session, "What color?", false)
		if len(session.OpenQuestions.NonBlocking) != 1 {
			t.Errorf("expected 1 non-blocking question, got %d", len(session.OpenQuestions.NonBlocking))
		}

		resolved := ResolveOpenQuestion(session, "What about security?", "Use OAuth2")
		if !resolved {
			t.Error("expected question to be resolved")
		}
		if len(session.OpenQuestions.Blocking) != 0 {
			t.Errorf("expected 0 blocking questions after resolve, got %d", len(session.OpenQuestions.Blocking))
		}
	})

	t.Run("tracks decisions", func(t *testing.T) {
		session := &Session{
			Topic:     "Decision Tracking",
			Decisions: []Decision{},
			ADRs:      []string{},
		}

		AddDecision(session, "Use PostgreSQL", "ADR-001")
		if len(session.Decisions) != 1 {
			t.Errorf("expected 1 decision, got %d", len(session.Decisions))
		}
		if len(session.ADRs) != 1 {
			t.Errorf("expected 1 ADR, got %d", len(session.ADRs))
		}

		AddDecision(session, "Use REST API", "")
		if len(session.Decisions) != 2 {
			t.Errorf("expected 2 decisions, got %d", len(session.Decisions))
		}
		if len(session.ADRs) != 1 {
			t.Errorf("expected 1 ADR (no new ADR added), got %d", len(session.ADRs))
		}
	})

	t.Run("advances phase correctly", func(t *testing.T) {
		session := &Session{
			Topic: "Phase Tracking",
			Phase: "scope",
		}

		next, err := AdvancePhase(session)
		if err != nil {
			t.Fatalf("AdvancePhase failed: %v", err)
		}
		if next != "clarify" {
			t.Errorf("expected 'clarify' phase, got '%s'", next)
		}
		if session.Phase != "clarify" {
			t.Errorf("session phase should be updated to 'clarify', got '%s'", session.Phase)
		}

		session.Phase = "complete"
		_, err = AdvancePhase(session)
		if err == nil {
			t.Error("expected error when advancing from final phase")
		}
	})

	t.Run("sets scope correctly", func(t *testing.T) {
		session := &Session{
			Topic: "Scope Test",
			Scope: Scope{},
		}

		SetScope(session, "Build a CLI tool", []string{"GUI support"}, []string{"All tests pass", "Documentation complete"})

		if session.Scope.Goal != "Build a CLI tool" {
			t.Errorf("expected goal 'Build a CLI tool', got '%s'", session.Scope.Goal)
		}
		if len(session.Scope.NonGoals) != 1 {
			t.Errorf("expected 1 non-goal, got %d", len(session.Scope.NonGoals))
		}
		if len(session.Scope.DoneMeans) != 2 {
			t.Errorf("expected 2 done means, got %d", len(session.Scope.DoneMeans))
		}
	})

	t.Run("checks gate requirements", func(t *testing.T) {
		tmpDir := t.TempDir()

		claudeDir := filepath.Join(tmpDir, ".claude")
		if err := os.MkdirAll(claudeDir, 0755); err != nil {
			t.Fatalf("failed to create .claude dir: %v", err)
		}
		discoveryPath := filepath.Join(tmpDir, DiscoveryFile)
		if err := os.WriteFile(discoveryPath, []byte("# Discovery"), 0644); err != nil {
			t.Fatalf("failed to create discovery file: %v", err)
		}

		session := &Session{
			Topic: "Gate Test",
			Phase: "decisions",
			Scope: Scope{
				Goal:      "",
				DoneMeans: []string{},
			},
			OpenQuestions: OpenQuestions{
				Blocking: []string{"Unresolved question"},
			},
		}

		result := CheckGate(session, tmpDir)
		if result.Passed {
			t.Error("gate should fail with missing goal, done means, and blocking questions")
		}
		if len(result.Issues) == 0 {
			t.Error("expected issues to be reported")
		}

		session.Scope.Goal = "Build something"
		session.Scope.DoneMeans = []string{"Tests pass"}
		session.OpenQuestions.Blocking = []string{}
		session.Phase = "gate"

		result = CheckGate(session, tmpDir)
		if !result.Passed {
			t.Errorf("gate should pass: %v", result.Issues)
		}
	})

	t.Run("gets session status", func(t *testing.T) {
		tmpDir := t.TempDir()

		status, err := GetSessionStatus(tmpDir)
		if err != nil {
			t.Fatalf("GetSessionStatus failed: %v", err)
		}
		if status.Status != "no_session" {
			t.Errorf("expected 'no_session' status, got '%s'", status.Status)
		}

		session, err := InitSession(tmpDir, "Status Test", "")
		if err != nil {
			t.Fatalf("InitSession failed: %v", err)
		}

		AddOpenQuestion(session, "Q1?", true)
		AddOpenQuestion(session, "Q2?", false)
		AddDecision(session, "D1", "")
		if err := SaveSessionToDir(tmpDir, session); err != nil {
			t.Fatalf("SaveSessionToDir failed: %v", err)
		}

		status, err = GetSessionStatus(tmpDir)
		if err != nil {
			t.Fatalf("GetSessionStatus failed: %v", err)
		}

		if status.Status != "active" {
			t.Errorf("expected 'active' status, got '%s'", status.Status)
		}
		if status.Topic != "Status Test" {
			t.Errorf("expected topic 'Status Test', got '%s'", status.Topic)
		}
		if status.Phase != "scope" {
			t.Errorf("expected phase 'scope', got '%s'", status.Phase)
		}
		if status.OpenQuestions.Blocking != 1 {
			t.Errorf("expected 1 blocking question, got %d", status.OpenQuestions.Blocking)
		}
		if status.OpenQuestions.NonBlocking != 1 {
			t.Errorf("expected 1 non-blocking question, got %d", status.OpenQuestions.NonBlocking)
		}
		if status.Decisions != 1 {
			t.Errorf("expected 1 decision, got %d", status.Decisions)
		}
		if status.TotalPhases != len(PhaseOrder) {
			t.Errorf("expected %d total phases, got %d", len(PhaseOrder), status.TotalPhases)
		}
	})
}

func TestResolveOpenQuestion(t *testing.T) {
	t.Run("resolves blocking question", func(t *testing.T) {
		session := &Session{
			OpenQuestions: OpenQuestions{
				Blocking:    []string{"Q1", "Q2"},
				NonBlocking: []string{},
			},
		}

		resolved := ResolveOpenQuestion(session, "Q1", "Answer 1")
		if !resolved {
			t.Error("expected Q1 to be resolved")
		}
		if len(session.OpenQuestions.Blocking) != 1 {
			t.Errorf("expected 1 remaining blocking question, got %d", len(session.OpenQuestions.Blocking))
		}
		if session.OpenQuestions.Blocking[0] != "Q2" {
			t.Errorf("expected Q2 to remain, got %s", session.OpenQuestions.Blocking[0])
		}
	})

	t.Run("resolves non-blocking question", func(t *testing.T) {
		session := &Session{
			OpenQuestions: OpenQuestions{
				Blocking:    []string{},
				NonBlocking: []string{"Q1", "Q2"},
			},
		}

		resolved := ResolveOpenQuestion(session, "Q2", "Answer 2")
		if !resolved {
			t.Error("expected Q2 to be resolved")
		}
		if len(session.OpenQuestions.NonBlocking) != 1 {
			t.Errorf("expected 1 remaining non-blocking question, got %d", len(session.OpenQuestions.NonBlocking))
		}
	})

	t.Run("returns false for unknown question", func(t *testing.T) {
		session := &Session{
			OpenQuestions: OpenQuestions{
				Blocking:    []string{"Q1"},
				NonBlocking: []string{"Q2"},
			},
		}

		resolved := ResolveOpenQuestion(session, "Q999", "Answer")
		if resolved {
			t.Error("expected false for unknown question")
		}
	})
}

func TestPhaseOrder(t *testing.T) {
	t.Run("has correct number of phases", func(t *testing.T) {
		if len(PhaseOrder) != 9 {
			t.Errorf("expected 9 phases, got %d", len(PhaseOrder))
		}
	})

	t.Run("starts with scope and ends with complete", func(t *testing.T) {
		if PhaseOrder[0] != "scope" {
			t.Errorf("expected first phase to be 'scope', got '%s'", PhaseOrder[0])
		}
		if PhaseOrder[len(PhaseOrder)-1] != "complete" {
			t.Errorf("expected last phase to be 'complete', got '%s'", PhaseOrder[len(PhaseOrder)-1])
		}
	})
}

func TestGetNextADRNumber(t *testing.T) {
	t.Run("returns 1 for nonexistent directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		num, err := GetNextADRNumber(tmpDir)
		if err != nil {
			t.Fatalf("GetNextADRNumber failed: %v", err)
		}
		if num != 1 {
			t.Errorf("expected 1, got %d", num)
		}
	})

	t.Run("returns next number based on existing ADRs", func(t *testing.T) {
		tmpDir := t.TempDir()
		adrsDir := filepath.Join(tmpDir, "docs", "adrs")
		if err := os.MkdirAll(adrsDir, 0755); err != nil {
			t.Fatalf("failed to create adrs dir: %v", err)
		}

		os.WriteFile(filepath.Join(adrsDir, "ADR-001-first.md"), []byte("# ADR 1"), 0644)
		os.WriteFile(filepath.Join(adrsDir, "ADR-005-later.md"), []byte("# ADR 5"), 0644)
		os.WriteFile(filepath.Join(adrsDir, "ADR-003-middle.md"), []byte("# ADR 3"), 0644)
		os.WriteFile(filepath.Join(adrsDir, "README.md"), []byte("# Info"), 0644)

		num, err := GetNextADRNumber(tmpDir)
		if err != nil {
			t.Fatalf("GetNextADRNumber failed: %v", err)
		}
		if num != 6 {
			t.Errorf("expected 6, got %d", num)
		}
	})

	t.Run("returns 1 for empty directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		adrsDir := filepath.Join(tmpDir, "docs", "adrs")
		if err := os.MkdirAll(adrsDir, 0755); err != nil {
			t.Fatalf("failed to create adrs dir: %v", err)
		}

		num, err := GetNextADRNumber(tmpDir)
		if err != nil {
			t.Fatalf("GetNextADRNumber failed: %v", err)
		}
		if num != 1 {
			t.Errorf("expected 1, got %d", num)
		}
	})

	t.Run("ignores subdirectories", func(t *testing.T) {
		tmpDir := t.TempDir()
		adrsDir := filepath.Join(tmpDir, "docs", "adrs")
		if err := os.MkdirAll(adrsDir, 0755); err != nil {
			t.Fatalf("failed to create adrs dir: %v", err)
		}
		os.MkdirAll(filepath.Join(adrsDir, "ADR-999-subdir"), 0755)
		os.WriteFile(filepath.Join(adrsDir, "ADR-002-real.md"), []byte("# ADR"), 0644)

		num, err := GetNextADRNumber(tmpDir)
		if err != nil {
			t.Fatalf("GetNextADRNumber failed: %v", err)
		}
		if num != 3 {
			t.Errorf("expected 3, got %d", num)
		}
	})
}

func TestPhaseIndex(t *testing.T) {
	t.Run("returns 0 for unknown phase", func(t *testing.T) {
		idx := phaseIndex("unknown_phase")
		if idx != 0 {
			t.Errorf("expected 0 for unknown phase, got %d", idx)
		}
	})

	t.Run("returns correct index for known phases", func(t *testing.T) {
		tests := []struct {
			phase string
			want  int
		}{
			{"scope", 0},
			{"clarify", 1},
			{"complete", 8},
		}

		for _, tt := range tests {
			idx := phaseIndex(tt.phase)
			if idx != tt.want {
				t.Errorf("phaseIndex(%q) = %d, want %d", tt.phase, idx, tt.want)
			}
		}
	})
}

func TestGetSessionStatusAtCompletePhase(t *testing.T) {
	tmpDir := t.TempDir()

	session, err := InitSession(tmpDir, "Complete Test", "")
	if err != nil {
		t.Fatalf("InitSession failed: %v", err)
	}

	session.Phase = "complete"
	if err := SaveSessionToDir(tmpDir, session); err != nil {
		t.Fatalf("SaveSessionToDir failed: %v", err)
	}

	status, err := GetSessionStatus(tmpDir)
	if err != nil {
		t.Fatalf("GetSessionStatus failed: %v", err)
	}

	if status.Phase != "complete" {
		t.Errorf("expected phase 'complete', got '%s'", status.Phase)
	}
	if status.PhaseIndex != 8 {
		t.Errorf("expected phase index 8, got %d", status.PhaseIndex)
	}
}

func TestLoadSessionInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, ".claude")
	os.MkdirAll(claudeDir, 0755)

	sessionPath := filepath.Join(tmpDir, SessionFile)
	os.WriteFile(sessionPath, []byte("{invalid json}"), 0644)

	_, err := LoadSessionFromBaseDir(tmpDir)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestGetSessionStatusWithSpecsAndADRs(t *testing.T) {
	tmpDir := t.TempDir()

	session, err := InitSession(tmpDir, "Full Status Test", "")
	if err != nil {
		t.Fatalf("InitSession failed: %v", err)
	}

	specsDir := filepath.Join(tmpDir, "docs", "specs")
	adrsDir := filepath.Join(tmpDir, "docs", "adrs")
	os.MkdirAll(specsDir, 0755)
	os.MkdirAll(adrsDir, 0755)

	os.WriteFile(filepath.Join(specsDir, "spec1.md"), []byte("# Spec 1"), 0644)
	os.WriteFile(filepath.Join(specsDir, "spec2.md"), []byte("# Spec 2"), 0644)
	os.WriteFile(filepath.Join(adrsDir, "ADR-001-test.md"), []byte("# ADR"), 0644)
	os.WriteFile(filepath.Join(adrsDir, "ADR-002-test.md"), []byte("# ADR"), 0644)
	os.WriteFile(filepath.Join(adrsDir, "README.md"), []byte("# Info"), 0644)

	if err := SaveSessionToDir(tmpDir, session); err != nil {
		t.Fatalf("SaveSessionToDir failed: %v", err)
	}

	status, err := GetSessionStatus(tmpDir)
	if err != nil {
		t.Fatalf("GetSessionStatus failed: %v", err)
	}

	if status.Specs != 2 {
		t.Errorf("expected 2 specs, got %d", status.Specs)
	}
	if status.ADRs != 2 {
		t.Errorf("expected 2 ADRs, got %d", status.ADRs)
	}
}

func TestCheckGateWithDiscoveryRounds(t *testing.T) {
	tmpDir := t.TempDir()

	claudeDir := filepath.Join(tmpDir, ".claude")
	os.MkdirAll(claudeDir, 0755)

	discoveryContent := `# Discovery Log

### Round 1
Some content

### Round 2
More content
`
	discoveryPath := filepath.Join(tmpDir, DiscoveryFile)
	os.WriteFile(discoveryPath, []byte(discoveryContent), 0644)

	session := &Session{
		Topic: "Gate Round Test",
		Phase: "gate",
		Scope: Scope{
			Goal:      "Test",
			DoneMeans: []string{"Done"},
		},
	}

	result := CheckGate(session, tmpDir)
	if !result.Passed {
		t.Errorf("gate should pass: %v", result.Issues)
	}
}
