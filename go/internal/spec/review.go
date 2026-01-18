package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/dgordon/tasker/internal/errors"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

type Category string

const (
	CategoryNonBehavioral Category = "non_behavioral"
	CategoryImplicit      Category = "implicit"
	CategoryCrossCutting  Category = "cross_cutting"
	CategoryMissingAC     Category = "missing_ac"
	CategoryFragmented    Category = "fragmented"
	CategoryContradiction Category = "contradiction"
)

type Weakness struct {
	ID                 string   `json:"id"`
	Category           Category `json:"category"`
	Severity           Severity `json:"severity"`
	Location           string   `json:"location"`
	Description        string   `json:"description"`
	SpecQuote          string   `json:"spec_quote,omitempty"`
	SuggestedRes       string   `json:"suggested_resolution,omitempty"`
	BehavioralReframe  string   `json:"behavioral_reframe,omitempty"`
}

type Summary struct {
	Total      int            `json:"total"`
	BySeverity map[string]int `json:"by_severity"`
	ByCategory map[string]int `json:"by_category"`
	Blocking   bool           `json:"blocking"`
}

type ReviewStatus string

const (
	StatusPending   ReviewStatus = "pending"
	StatusInReview  ReviewStatus = "in_review"
	StatusResolved  ReviewStatus = "resolved"
)

type SpecReview struct {
	Version      string       `json:"version"`
	SpecChecksum string       `json:"spec_checksum"`
	AnalyzedAt   string       `json:"analyzed_at"`
	Weaknesses   []Weakness   `json:"weaknesses"`
	Status       ReviewStatus `json:"status"`
	Summary      Summary      `json:"summary"`
	Notes        string       `json:"notes,omitempty"`
}

type AnalysisResult struct {
	Review     *SpecReview
	NewFindings int
	Blocking   bool
}

var weaknessPatterns = []struct {
	Category    Category
	Severity    Severity
	Pattern     *regexp.Regexp
	Description string
}{
	{
		Category:    CategoryNonBehavioral,
		Severity:    SeverityWarning,
		Pattern:     regexp.MustCompile(`(?i)(must be|should be|will be)\s+(fast|efficient|scalable|maintainable|clean|simple)`),
		Description: "Non-behavioral quality attribute without measurable criteria",
	},
	{
		Category:    CategoryImplicit,
		Severity:    SeverityWarning,
		Pattern:     regexp.MustCompile(`(?i)(obviously|clearly|naturally|of course|as expected)`),
		Description: "Implicit assumption - may need explicit definition",
	},
	{
		Category:    CategoryMissingAC,
		Severity:    SeverityCritical,
		Pattern:     regexp.MustCompile(`(?i)(handle\s+errors?|error\s+handling)\s*[^:.]`),
		Description: "Error handling mentioned without acceptance criteria",
	},
	{
		Category:    CategoryFragmented,
		Severity:    SeverityInfo,
		Pattern:     regexp.MustCompile(`(?i)(see\s+also|refer\s+to|as\s+mentioned|above|below)`),
		Description: "Fragmented requirement - references another section",
	},
	{
		Category:    CategoryCrossCutting,
		Severity:    SeverityInfo,
		Pattern:     regexp.MustCompile(`(?i)(logging|authentication|authorization|security|caching|monitoring)`),
		Description: "Cross-cutting concern - ensure consistent handling",
	},
}

func AnalyzeSpec(specPath string) (*AnalysisResult, error) {
	content, err := os.ReadFile(specPath)
	if err != nil {
		return nil, errors.IOReadFailed(specPath, err)
	}

	checksum := computeChecksum(content)
	lines := strings.Split(string(content), "\n")
	weaknesses := detectWeaknesses(lines)

	summary := computeSummary(weaknesses)

	review := &SpecReview{
		Version:      "1.0",
		SpecChecksum: checksum,
		AnalyzedAt:   time.Now().UTC().Format(time.RFC3339),
		Weaknesses:   weaknesses,
		Status:       StatusPending,
		Summary:      summary,
	}

	if summary.BySeverity["critical"] > 0 {
		review.Status = StatusInReview
	}

	return &AnalysisResult{
		Review:      review,
		NewFindings: len(weaknesses),
		Blocking:    summary.Blocking,
	}, nil
}

func AnalyzeSpecContent(content []byte, sourceName string) (*AnalysisResult, error) {
	checksum := computeChecksum(content)
	lines := strings.Split(string(content), "\n")
	weaknesses := detectWeaknesses(lines)

	summary := computeSummary(weaknesses)

	review := &SpecReview{
		Version:      "1.0",
		SpecChecksum: checksum,
		AnalyzedAt:   time.Now().UTC().Format(time.RFC3339),
		Weaknesses:   weaknesses,
		Status:       StatusPending,
		Summary:      summary,
		Notes:        fmt.Sprintf("Analyzed from: %s", sourceName),
	}

	if summary.BySeverity["critical"] > 0 {
		review.Status = StatusInReview
	}

	return &AnalysisResult{
		Review:      review,
		NewFindings: len(weaknesses),
		Blocking:    summary.Blocking,
	}, nil
}

func detectWeaknesses(lines []string) []Weakness {
	var weaknesses []Weakness
	counters := make(map[Category]int)

	for lineNum, line := range lines {
		for _, wp := range weaknessPatterns {
			if wp.Pattern.MatchString(line) {
				counters[wp.Category]++
				id := fmt.Sprintf("W%d-%03d", categoryIndex(wp.Category), counters[wp.Category])

				quote := strings.TrimSpace(line)
				if len(quote) > 100 {
					quote = quote[:97] + "..."
				}

				weaknesses = append(weaknesses, Weakness{
					ID:          id,
					Category:    wp.Category,
					Severity:    wp.Severity,
					Location:    fmt.Sprintf("line %d", lineNum+1),
					Description: wp.Description,
					SpecQuote:   quote,
				})
			}
		}
	}

	return weaknesses
}

func categoryIndex(cat Category) int {
	switch cat {
	case CategoryNonBehavioral:
		return 1
	case CategoryImplicit:
		return 2
	case CategoryCrossCutting:
		return 3
	case CategoryMissingAC:
		return 4
	case CategoryFragmented:
		return 5
	case CategoryContradiction:
		return 6
	default:
		return 0
	}
}

func computeChecksum(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])[:16]
}

func computeSummary(weaknesses []Weakness) Summary {
	summary := Summary{
		Total:      len(weaknesses),
		BySeverity: make(map[string]int),
		ByCategory: make(map[string]int),
		Blocking:   false,
	}

	summary.BySeverity["critical"] = 0
	summary.BySeverity["warning"] = 0
	summary.BySeverity["info"] = 0

	for _, w := range weaknesses {
		summary.BySeverity[string(w.Severity)]++
		summary.ByCategory[string(w.Category)]++
	}

	summary.Blocking = summary.BySeverity["critical"] > 0

	return summary
}

type ReviewStatusResult struct {
	Status       ReviewStatus
	TotalIssues  int
	Critical     int
	Warnings     int
	Info         int
	Blocking     bool
	AnalyzedAt   string
	SpecChecksum string
}

func GetReviewStatus(planningDir string) (*ReviewStatusResult, error) {
	reviewPath := filepath.Join(planningDir, "artifacts", "spec-review.json")

	data, err := os.ReadFile(reviewPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &ReviewStatusResult{
				Status:      StatusPending,
				TotalIssues: 0,
				Critical:    0,
				Warnings:    0,
				Info:        0,
				Blocking:    false,
			}, nil
		}
		return nil, errors.IOReadFailed(reviewPath, err)
	}

	var review SpecReview
	if err := json.Unmarshal(data, &review); err != nil {
		return nil, errors.Internal("failed to parse spec review", err)
	}

	return &ReviewStatusResult{
		Status:       review.Status,
		TotalIssues:  review.Summary.Total,
		Critical:     review.Summary.BySeverity["critical"],
		Warnings:     review.Summary.BySeverity["warning"],
		Info:         review.Summary.BySeverity["info"],
		Blocking:     review.Summary.Blocking,
		AnalyzedAt:   review.AnalyzedAt,
		SpecChecksum: review.SpecChecksum,
	}, nil
}

func LoadReview(planningDir string) (*SpecReview, error) {
	reviewPath := filepath.Join(planningDir, "artifacts", "spec-review.json")

	data, err := os.ReadFile(reviewPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.IONotExists(reviewPath)
		}
		return nil, errors.IOReadFailed(reviewPath, err)
	}

	var review SpecReview
	if err := json.Unmarshal(data, &review); err != nil {
		return nil, errors.Internal("failed to parse spec review", err)
	}

	return &review, nil
}

func SaveReview(planningDir string, review *SpecReview) error {
	artifactsDir := filepath.Join(planningDir, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		return errors.IOWriteFailed(artifactsDir, err)
	}

	reviewPath := filepath.Join(artifactsDir, "spec-review.json")

	data, err := json.MarshalIndent(review, "", "  ")
	if err != nil {
		return errors.Internal("failed to marshal spec review", err)
	}

	if err := os.WriteFile(reviewPath, data, 0644); err != nil {
		return errors.IOWriteFailed(reviewPath, err)
	}

	return nil
}

func ResolveWeakness(review *SpecReview, weaknessID string, resolution string) error {
	found := false
	for i, w := range review.Weaknesses {
		if w.ID == weaknessID {
			review.Weaknesses[i].SuggestedRes = resolution
			found = true
			break
		}
	}

	if !found {
		return errors.ValidationFailed(fmt.Sprintf("weakness %s not found", weaknessID))
	}

	allResolved := true
	for _, w := range review.Weaknesses {
		if w.Severity == SeverityCritical && w.SuggestedRes == "" {
			allResolved = false
			break
		}
	}

	if allResolved && review.Summary.BySeverity["critical"] > 0 {
		review.Status = StatusResolved
	}

	return nil
}
