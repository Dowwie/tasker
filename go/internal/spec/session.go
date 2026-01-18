package spec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/dgordon/tasker/internal/errors"
)

const (
	SessionFile   = ".claude/spec-session.json"
	DiscoveryFile = ".claude/clarify-session.md"
)

const (
	PhaseScope        = "scope"
	PhaseClarify      = "clarify"
	PhaseSynthesis    = "synthesis"
	PhaseArchitecture = "architecture"
	PhaseDecisions    = "decisions"
	PhaseGate         = "gate"
	PhaseSpecReview   = "spec_review"
	PhaseExport       = "export"
	PhaseComplete     = "complete"
)

var PhaseOrder = []string{
	PhaseScope,
	PhaseClarify,
	PhaseSynthesis,
	PhaseArchitecture,
	PhaseDecisions,
	PhaseGate,
	PhaseSpecReview,
	PhaseExport,
	PhaseComplete,
}

type Scope struct {
	Goal      string   `json:"goal"`
	NonGoals  []string `json:"non_goals"`
	DoneMeans []string `json:"done_means"`
}

type OpenQuestions struct {
	Blocking    []string `json:"blocking"`
	NonBlocking []string `json:"non_blocking"`
}

type Decision struct {
	Decision  string `json:"decision"`
	ADRID     string `json:"adr_id,omitempty"`
	DecidedAt string `json:"decided_at,omitempty"`
}

type ResolvedQuestion struct {
	Question   string `json:"question"`
	Resolution string `json:"resolution"`
	ResolvedAt string `json:"resolved_at"`
}

type Session struct {
	Topic             string             `json:"topic"`
	TargetDir         string             `json:"target_dir,omitempty"`
	StartedAt         string             `json:"started_at"`
	Phase             string             `json:"phase"`
	Rounds            int                `json:"rounds"`
	Scope             Scope              `json:"scope"`
	OpenQuestions     OpenQuestions      `json:"open_questions"`
	Decisions         []Decision         `json:"decisions"`
	ADRs              []string           `json:"adrs"`
	ResolvedQuestions []ResolvedQuestion `json:"resolved_questions,omitempty"`
	Workflows         string             `json:"workflows,omitempty"`
	Invariants        []string           `json:"invariants,omitempty"`
	Interfaces        string             `json:"interfaces,omitempty"`
	Architecture      string             `json:"architecture,omitempty"`
	Handoff           map[string]string  `json:"handoff,omitempty"`
}

type SessionStatus struct {
	Status          string `json:"status"`
	Topic           string `json:"topic,omitempty"`
	TargetDir       string `json:"target_dir,omitempty"`
	Phase           string `json:"phase,omitempty"`
	PhaseIndex      int    `json:"phase_index"`
	TotalPhases     int    `json:"total_phases"`
	DiscoveryRounds int    `json:"discovery_rounds"`
	OpenQuestions   struct {
		Blocking    int `json:"blocking"`
		NonBlocking int `json:"non_blocking"`
	} `json:"open_questions"`
	Decisions int    `json:"decisions"`
	ADRs      int    `json:"adrs"`
	Specs     int    `json:"specs"`
	StartedAt string `json:"started_at,omitempty"`
	Message   string `json:"message,omitempty"`
}

type GateResult struct {
	Passed  bool     `json:"passed"`
	Issues  []string `json:"issues"`
	Message string   `json:"message"`
}

func InitSession(baseDir, topic, targetDir string) (*Session, error) {
	claudeDir := filepath.Join(baseDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return nil, errors.IOWriteFailed(claudeDir, err)
	}

	if targetDir == "" {
		targetDir = baseDir
	}

	session := &Session{
		Topic:     topic,
		TargetDir: targetDir,
		StartedAt: time.Now().UTC().Format(time.RFC3339),
		Phase:     "scope",
		Rounds:    0,
		Scope: Scope{
			Goal:      "",
			NonGoals:  []string{},
			DoneMeans: []string{},
		},
		OpenQuestions: OpenQuestions{
			Blocking:    []string{},
			NonBlocking: []string{},
		},
		Decisions: []Decision{},
		ADRs:      []string{},
	}

	discoveryContent := fmt.Sprintf(`# Discovery: %s
Started: %s

## Questions Asked

## Answers Received

## Emerging Requirements
`, topic, session.StartedAt)

	discoveryPath := filepath.Join(baseDir, DiscoveryFile)
	if err := os.WriteFile(discoveryPath, []byte(discoveryContent), 0644); err != nil {
		return nil, errors.IOWriteFailed(discoveryPath, err)
	}

	if err := SaveSessionToDir(baseDir, session); err != nil {
		return nil, err
	}

	return session, nil
}

func LoadSessionFromBaseDir(baseDir string) (*Session, error) {
	sessionPath := filepath.Join(baseDir, SessionFile)

	data, err := os.ReadFile(sessionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, errors.IOReadFailed(sessionPath, err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, errors.Internal("failed to parse session state", err)
	}

	return &session, nil
}

func SaveSessionToDir(baseDir string, session *Session) error {
	claudeDir := filepath.Join(baseDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return errors.IOWriteFailed(claudeDir, err)
	}

	sessionPath := filepath.Join(baseDir, SessionFile)

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return errors.Internal("failed to marshal session state", err)
	}

	if err := os.WriteFile(sessionPath, data, 0644); err != nil {
		return errors.IOWriteFailed(sessionPath, err)
	}

	return nil
}

func GetSessionStatus(baseDir string) (*SessionStatus, error) {
	session, err := LoadSessionFromBaseDir(baseDir)
	if err != nil {
		return nil, err
	}

	if session == nil {
		return &SessionStatus{
			Status:  "no_session",
			Message: "No active session. Run `/specify` to start.",
		}, nil
	}

	discoveryPath := filepath.Join(baseDir, DiscoveryFile)
	discoveryRounds := 0
	if content, err := os.ReadFile(discoveryPath); err == nil {
		roundPattern := regexp.MustCompile(`### Round \d+`)
		matches := roundPattern.FindAllString(string(content), -1)
		discoveryRounds = len(matches)
	}

	targetDir := session.TargetDir
	specsDir := filepath.Join(targetDir, "docs", "specs")
	specsCount := 0
	if entries, err := os.ReadDir(specsDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".md" {
				specsCount++
			}
		}
	}

	adrsDir := filepath.Join(targetDir, "docs", "adrs")
	adrsCount := 0
	if entries, err := os.ReadDir(adrsDir); err == nil {
		adrPattern := regexp.MustCompile(`^ADR-\d+`)
		for _, entry := range entries {
			if !entry.IsDir() && adrPattern.MatchString(entry.Name()) {
				adrsCount++
			}
		}
	}

	status := &SessionStatus{
		Status:          "active",
		Topic:           session.Topic,
		TargetDir:       targetDir,
		Phase:           session.Phase,
		PhaseIndex:      phaseIndex(session.Phase),
		TotalPhases:     len(PhaseOrder),
		DiscoveryRounds: discoveryRounds,
		Decisions:       len(session.Decisions),
		ADRs:            adrsCount,
		Specs:           specsCount,
		StartedAt:       session.StartedAt,
	}

	status.OpenQuestions.Blocking = len(session.OpenQuestions.Blocking)
	status.OpenQuestions.NonBlocking = len(session.OpenQuestions.NonBlocking)

	return status, nil
}

func AdvancePhase(session *Session) (string, error) {
	currentIdx := phaseIndex(session.Phase)
	if currentIdx >= len(PhaseOrder)-1 {
		return session.Phase, errors.ValidationFailed("already at final phase")
	}

	nextPhase := PhaseOrder[currentIdx+1]
	session.Phase = nextPhase
	return nextPhase, nil
}

func CheckGate(session *Session, baseDir string) *GateResult {
	var issues []string

	if session.Phase != "gate" && session.Phase != "decisions" && session.Phase != "architecture" {
		issues = append(issues, fmt.Sprintf("Not ready for gate check. Current phase: %s", session.Phase))
	}

	if len(session.OpenQuestions.Blocking) > 0 {
		issues = append(issues, fmt.Sprintf("Blocking Open Questions: %d", len(session.OpenQuestions.Blocking)))
		for i, q := range session.OpenQuestions.Blocking {
			if i >= 3 {
				break
			}
			issues = append(issues, fmt.Sprintf("  - %s", q))
		}
	}

	if session.Scope.Goal == "" {
		issues = append(issues, "Missing: Goal not defined")
	}
	if len(session.Scope.DoneMeans) == 0 {
		issues = append(issues, "Missing: Done means not defined")
	}

	discoveryPath := filepath.Join(baseDir, DiscoveryFile)
	if _, err := os.Stat(discoveryPath); os.IsNotExist(err) {
		issues = append(issues, "Missing: Discovery file not found")
	}

	passed := len(issues) == 0
	message := "Gate FAILED"
	if passed {
		message = "Gate PASSED - ready for export"
	}

	return &GateResult{
		Passed:  passed,
		Issues:  issues,
		Message: message,
	}
}

func AddOpenQuestion(session *Session, question string, blocking bool) {
	if blocking {
		session.OpenQuestions.Blocking = append(session.OpenQuestions.Blocking, question)
	} else {
		session.OpenQuestions.NonBlocking = append(session.OpenQuestions.NonBlocking, question)
	}
}

func ResolveOpenQuestion(session *Session, question, resolution string) bool {
	for i, q := range session.OpenQuestions.Blocking {
		if q == question {
			session.OpenQuestions.Blocking = append(
				session.OpenQuestions.Blocking[:i],
				session.OpenQuestions.Blocking[i+1:]...,
			)
			return true
		}
	}

	for i, q := range session.OpenQuestions.NonBlocking {
		if q == question {
			session.OpenQuestions.NonBlocking = append(
				session.OpenQuestions.NonBlocking[:i],
				session.OpenQuestions.NonBlocking[i+1:]...,
			)
			return true
		}
	}

	return false
}

func AddDecision(session *Session, decision string, adrID string) {
	d := Decision{
		Decision:  decision,
		ADRID:     adrID,
		DecidedAt: time.Now().UTC().Format(time.RFC3339),
	}
	session.Decisions = append(session.Decisions, d)

	if adrID != "" {
		session.ADRs = append(session.ADRs, adrID)
	}
}

func GetNextADRNumber(targetDir string) (int, error) {
	adrsDir := filepath.Join(targetDir, "docs", "adrs")

	entries, err := os.ReadDir(adrsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, errors.IOReadFailed(adrsDir, err)
	}

	adrPattern := regexp.MustCompile(`^ADR-(\d+)`)
	maxNum := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := adrPattern.FindStringSubmatch(entry.Name())
		if matches != nil {
			var num int
			fmt.Sscanf(matches[1], "%d", &num)
			if num > maxNum {
				maxNum = num
			}
		}
	}

	return maxNum + 1, nil
}

func SetScope(session *Session, goal string, nonGoals, doneMeans []string) {
	session.Scope.Goal = goal
	session.Scope.NonGoals = nonGoals
	session.Scope.DoneMeans = doneMeans
}

func IncrementRound(session *Session) {
	session.Rounds++
}

func phaseIndex(phase string) int {
	for i, p := range PhaseOrder {
		if p == phase {
			return i
		}
	}
	return 0
}
