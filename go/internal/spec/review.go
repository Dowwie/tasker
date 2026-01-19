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
	CategoryAmbiguity     Category = "ambiguity"
	CategoryChecklist     Category = "checklist_gap"
)

type ResolutionType string

const (
	ResolutionMandatory     ResolutionType = "mandatory"
	ResolutionOptional      ResolutionType = "optional"
	ResolutionDefer         ResolutionType = "defer"
	ResolutionClarified     ResolutionType = "clarified"
	ResolutionNotApplicable ResolutionType = "not_applicable"
)

type Resolution struct {
	WeaknessID       string         `json:"weakness_id"`
	Resolution       ResolutionType `json:"resolution"`
	UserResponse     string         `json:"user_response,omitempty"`
	BehavioralReframe string        `json:"behavioral_reframe,omitempty"`
	Notes            string         `json:"notes,omitempty"`
	ResolvedAt       string         `json:"resolved_at"`
}

type SpecResolutions struct {
	Version     string       `json:"version"`
	Resolutions []Resolution `json:"resolutions"`
}

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

type ChecklistSummary struct {
	Total           int `json:"total"`
	Complete        int `json:"complete"`
	Partial         int `json:"partial"`
	Missing         int `json:"missing"`
	NotApplicable   int `json:"na"`
	CriticalMissing int `json:"critical_missing"`
}

type Summary struct {
	Total      int               `json:"total"`
	BySeverity map[string]int    `json:"by_severity"`
	ByCategory map[string]int    `json:"by_category"`
	Blocking   bool              `json:"blocking"`
	Checklist  *ChecklistSummary `json:"checklist,omitempty"`
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
	Question    string
}{
	// W1: Non-behavioral
	{
		Category:    CategoryNonBehavioral,
		Severity:    SeverityWarning,
		Pattern:     regexp.MustCompile(`(?i)(must be|should be|will be)\s+(fast|efficient|scalable|maintainable|clean|simple)`),
		Description: "Non-behavioral quality attribute without measurable criteria",
	},
	{
		Category:    CategoryNonBehavioral,
		Severity:    SeverityCritical,
		Pattern:     regexp.MustCompile(`(?i)CREATE\s+TABLE`),
		Description: "DDL table definition - should be stated as behavioral requirement",
	},
	{
		Category:    CategoryNonBehavioral,
		Severity:    SeverityWarning,
		Pattern:     regexp.MustCompile(`(?i)CREATE\s+(UNIQUE\s+)?INDEX`),
		Description: "DDL index definition - should state query performance requirement",
	},
	{
		Category:    CategoryNonBehavioral,
		Severity:    SeverityCritical,
		Pattern:     regexp.MustCompile(`(?i)CONSTRAINT\s+\w+\s+(UNIQUE|CHECK|FOREIGN\s+KEY|PRIMARY\s+KEY)`),
		Description: "DDL constraint - should be stated as validation behavior",
	},
	// W2: Implicit
	{
		Category:    CategoryImplicit,
		Severity:    SeverityWarning,
		Pattern:     regexp.MustCompile(`(?i)(obviously|clearly|naturally|of course|as expected)`),
		Description: "Implicit assumption - may need explicit definition",
	},
	{
		Category:    CategoryImplicit,
		Severity:    SeverityWarning,
		Pattern:     regexp.MustCompile(`(?i)\w+\s+\w+.*NOT\s+NULL`),
		Description: "NOT NULL constraint implied but not stated as requirement",
	},
	{
		Category:    CategoryImplicit,
		Severity:    SeverityInfo,
		Pattern:     regexp.MustCompile(`(?i)\w+.*DEFAULT\s+`),
		Description: "Default value specified - confirm this is intentional",
	},
	// W3: Cross-cutting
	{
		Category:    CategoryCrossCutting,
		Severity:    SeverityInfo,
		Pattern:     regexp.MustCompile(`(?i)(logging|authentication|authorization|security|caching|monitoring)`),
		Description: "Cross-cutting concern - ensure consistent handling",
	},
	{
		Category:    CategoryCrossCutting,
		Severity:    SeverityWarning,
		Pattern:     regexp.MustCompile(`(?i)\|\s*Variable\s*\|\s*Type\s*\|`),
		Description: "Configuration table - ensure each var is wired to a component",
	},
	{
		Category:    CategoryCrossCutting,
		Severity:    SeverityWarning,
		Pattern:     regexp.MustCompile(`(?i)(OTEL|OpenTelemetry|Prometheus|metric|counter|gauge|histogram)`),
		Description: "Observability requirement - spans multiple components",
	},
	{
		Category:    CategoryCrossCutting,
		Severity:    SeverityWarning,
		Pattern:     regexp.MustCompile(`(?i)(Startup|Shutdown)\s+(Sequence|Tasks?|Order)`),
		Description: "Lifecycle requirement - ensure startup/shutdown tasks exist",
	},
	// W4: Missing acceptance criteria
	{
		Category:    CategoryMissingAC,
		Severity:    SeverityCritical,
		Pattern:     regexp.MustCompile(`(?i)(handle\s+errors?|error\s+handling)\s*[^:.]`),
		Description: "Error handling mentioned without acceptance criteria",
	},
	{
		Category:    CategoryMissingAC,
		Severity:    SeverityInfo,
		Pattern:     regexp.MustCompile(`(?i)must\s+be\s+(fast|quick|responsive)`),
		Description: "Performance requirement without specific metric",
	},
	{
		Category:    CategoryMissingAC,
		Severity:    SeverityInfo,
		Pattern:     regexp.MustCompile(`(?i)should\s+be\s+secure`),
		Description: "Security requirement without specifics",
	},
	// W5: Fragmented
	{
		Category:    CategoryFragmented,
		Severity:    SeverityInfo,
		Pattern:     regexp.MustCompile(`(?i)(see\s+also|refer\s+to|as\s+mentioned|above|below)`),
		Description: "Fragmented requirement - references another section",
	},
	{
		Category:    CategoryFragmented,
		Severity:    SeverityInfo,
		Pattern:     regexp.MustCompile(`(?i)see\s+Section\s+\d+`),
		Description: "Cross-reference to another section - requirement may be fragmented",
	},
	// W6: Contradictions (detected separately via default value analysis)
	// W7: Ambiguity
	{
		Category:    CategoryAmbiguity,
		Severity:    SeverityWarning,
		Pattern:     regexp.MustCompile(`(?i)\b(some|many|few|several|various|numerous|multiple)\s+\w+`),
		Description: "Vague quantifier - specify exact number or range",
		Question:    "How many specifically? Provide a number or range.",
	},
	{
		Category:    CategoryAmbiguity,
		Severity:    SeverityWarning,
		Pattern:     regexp.MustCompile(`(?i)\b(etc\.?|and so on|and more|similar\s+\w+)\b`),
		Description: "Undefined scope - list all items explicitly",
		Question:    "What specifically is included? List all items explicitly.",
	},
	{
		Category:    CategoryAmbiguity,
		Severity:    SeverityWarning,
		Pattern:     regexp.MustCompile(`(?i)\b(if applicable|when appropriate|as needed|when necessary|if required|where possible)\b`),
		Description: "Vague conditional - define the criteria",
		Question:    "Under what specific conditions does this apply?",
	},
	{
		Category:    CategoryAmbiguity,
		Severity:    SeverityCritical,
		Pattern:     regexp.MustCompile(`(?i)\b(may|might|could|possibly|optionally)\s+(be|have|include|support|allow)`),
		Description: "Weak requirement - clarify if required or optional",
		Question:    "Is this required or optional? If optional, under what conditions?",
	},
	{
		Category:    CategoryAmbiguity,
		Severity:    SeverityWarning,
		Pattern:     regexp.MustCompile(`(?i)\b(is|are|will be|should be|must be)\s+(handled|processed|validated|checked|verified|managed)\b`),
		Description: "Passive voice hiding actor - specify the component",
		Question:    "What component/system performs this action?",
	},
	{
		Category:    CategoryAmbiguity,
		Severity:    SeverityWarning,
		Pattern:     regexp.MustCompile(`(?i)\b(quickly|soon|immediately|eventually|periodically|regularly)\b`),
		Description: "Vague timing - specify exact timing requirement",
		Question:    "What is the specific timing? (e.g., <100ms, every 5 minutes)",
	},
	{
		Category:    CategoryAmbiguity,
		Severity:    SeverityWarning,
		Pattern:     regexp.MustCompile(`(?i)\b(properly|correctly|appropriately|adequately|sufficiently)\s+(handle|process|validate|manage)`),
		Description: "Vague behavior - define expected behavior explicitly",
		Question:    "What does this mean specifically? Define the expected behavior.",
	},
	{
		Category:    CategoryAmbiguity,
		Severity:    SeverityCritical,
		Pattern:     regexp.MustCompile(`(?i)\b\w+\s+or\s+\w+\s+(can|may|should|must|will)\b`),
		Description: "Unresolved or - specify which option or if both are valid",
		Question:    "Which one? Or are both valid? Specify the rule.",
	},
	{
		Category:    CategoryAmbiguity,
		Severity:    SeverityWarning,
		Pattern:     regexp.MustCompile(`(?i)\b(reasonable|appropriate|suitable|adequate|sufficient)\s+\w+`),
		Description: "Subjective qualifier - define acceptance criteria",
		Question:    "What makes this acceptable? Define the criteria.",
	},
	{
		Category:    CategoryAmbiguity,
		Severity:    SeverityWarning,
		Pattern:     regexp.MustCompile(`(?i)\b(standard|typical|normal|usual|common)\s+(practice|behavior|approach|way)`),
		Description: "External reference - document the expected behavior",
		Question:    "Which standard specifically? Document the expected behavior.",
	},
	{
		Category:    CategoryAmbiguity,
		Severity:    SeverityWarning,
		Pattern:     regexp.MustCompile(`(?i)\b(large|small|long|short|high|low|fast|slow)\s+(number|amount|size|duration|latency|throughput)`),
		Description: "Unquantified limit - provide a threshold",
		Question:    "What specific value constitutes this? Provide a threshold.",
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

	// W6: Contradiction detection (conflicting default values)
	contradictions := detectContradictions(string(content))
	weaknesses = append(weaknesses, contradictions...)

	checklist := VerifyChecklist(string(content))
	checklistWeaknesses := ChecklistToWeaknesses(checklist)
	weaknesses = append(weaknesses, checklistWeaknesses...)

	summary := computeSummary(weaknesses)
	summary.Checklist = &ChecklistSummary{
		Total:           len(checklist.Items),
		Complete:        checklist.Complete,
		Partial:         checklist.Partial,
		Missing:         checklist.Missing,
		NotApplicable:   checklist.NotApplicable,
		CriticalMissing: checklist.CriticalMissing,
	}

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

	// W6: Contradiction detection (conflicting default values)
	contradictions := detectContradictions(string(content))
	weaknesses = append(weaknesses, contradictions...)

	checklist := VerifyChecklist(string(content))
	checklistWeaknesses := ChecklistToWeaknesses(checklist)
	weaknesses = append(weaknesses, checklistWeaknesses...)

	summary := computeSummary(weaknesses)
	summary.Checklist = &ChecklistSummary{
		Total:           len(checklist.Items),
		Complete:        checklist.Complete,
		Partial:         checklist.Partial,
		Missing:         checklist.Missing,
		NotApplicable:   checklist.NotApplicable,
		CriticalMissing: checklist.CriticalMissing,
	}

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

// defaultValuePattern matches variable default value declarations
// Note: longer pattern (defaults?\s+to) must come before shorter (default) in alternation
var defaultValuePattern = regexp.MustCompile(`(?i)(\w+).*(?:defaults?\s+to|default)\s*[:\s]*[\x60'"]?(\w+)[\x60'"]?`)

// detectContradictions finds conflicting default values for the same variable
func detectContradictions(content string) []Weakness {
	var weaknesses []Weakness

	// Track default values: variable name -> list of (value, lineNum)
	type valueLocation struct {
		value   string
		lineNum int
	}
	defaultValues := make(map[string][]valueLocation)

	lines := strings.Split(content, "\n")
	for lineNum, line := range lines {
		matches := defaultValuePattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			if len(match) >= 3 {
				varName := strings.ToLower(match[1])
				value := match[2]
				defaultValues[varName] = append(defaultValues[varName], valueLocation{
					value:   value,
					lineNum: lineNum + 1,
				})
			}
		}
	}

	counter := 0
	for varName, values := range defaultValues {
		// Check for conflicting values
		uniqueValues := make(map[string]bool)
		for _, v := range values {
			uniqueValues[v.value] = true
		}

		if len(uniqueValues) > 1 {
			counter++
			// Build location string
			var locations []string
			var valueList []string
			for _, v := range values {
				locations = append(locations, fmt.Sprintf("line %d", v.lineNum))
			}
			for val := range uniqueValues {
				valueList = append(valueList, val)
			}

			weaknesses = append(weaknesses, Weakness{
				ID:          fmt.Sprintf("W6-%03d", counter),
				Category:    CategoryContradiction,
				Severity:    SeverityCritical,
				Location:    strings.Join(locations, ", "),
				Description: fmt.Sprintf("Conflicting default values for '%s': %s", varName, strings.Join(valueList, ", ")),
				SpecQuote:   fmt.Sprintf("Multiple defaults found: %s", strings.Join(valueList, ", ")),
				SuggestedRes: "Clarify which default value is authoritative",
			})
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
	case CategoryAmbiguity:
		return 7
	case CategoryChecklist:
		return 8
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

func LoadResolutions(planningDir string) (*SpecResolutions, error) {
	resolutionsPath := filepath.Join(planningDir, "artifacts", "spec-resolutions.json")

	data, err := os.ReadFile(resolutionsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &SpecResolutions{
				Version:     "1.0",
				Resolutions: []Resolution{},
			}, nil
		}
		return nil, errors.IOReadFailed(resolutionsPath, err)
	}

	var resolutions SpecResolutions
	if err := json.Unmarshal(data, &resolutions); err != nil {
		return nil, errors.Internal("failed to parse spec resolutions", err)
	}

	return &resolutions, nil
}

func SaveResolutions(planningDir string, resolutions *SpecResolutions) error {
	artifactsDir := filepath.Join(planningDir, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0755); err != nil {
		return errors.IOWriteFailed(artifactsDir, err)
	}

	resolutionsPath := filepath.Join(artifactsDir, "spec-resolutions.json")

	data, err := json.MarshalIndent(resolutions, "", "  ")
	if err != nil {
		return errors.Internal("failed to marshal spec resolutions", err)
	}

	if err := os.WriteFile(resolutionsPath, data, 0644); err != nil {
		return errors.IOWriteFailed(resolutionsPath, err)
	}

	return nil
}

func AddResolution(planningDir string, weaknessID string, resType ResolutionType, userResponse, behavioralReframe, notes string) error {
	resolutions, err := LoadResolutions(planningDir)
	if err != nil {
		return err
	}

	for _, r := range resolutions.Resolutions {
		if r.WeaknessID == weaknessID {
			return errors.ValidationFailed(fmt.Sprintf("weakness %s already resolved", weaknessID))
		}
	}

	resolution := Resolution{
		WeaknessID:        weaknessID,
		Resolution:        resType,
		UserResponse:      userResponse,
		BehavioralReframe: behavioralReframe,
		Notes:             notes,
		ResolvedAt:        time.Now().UTC().Format(time.RFC3339),
	}

	resolutions.Resolutions = append(resolutions.Resolutions, resolution)

	return SaveResolutions(planningDir, resolutions)
}

func GetUnresolvedWeaknesses(planningDir string) ([]Weakness, error) {
	review, err := LoadReview(planningDir)
	if err != nil {
		return nil, err
	}

	resolutions, err := LoadResolutions(planningDir)
	if err != nil {
		return nil, err
	}

	resolvedIDs := make(map[string]bool)
	for _, r := range resolutions.Resolutions {
		resolvedIDs[r.WeaknessID] = true
	}

	var unresolved []Weakness
	for _, w := range review.Weaknesses {
		if !resolvedIDs[w.ID] && w.Severity == SeverityCritical {
			unresolved = append(unresolved, w)
		}
	}

	return unresolved, nil
}

func IsReadyToProceed(planningDir string) (bool, int, error) {
	unresolved, err := GetUnresolvedWeaknesses(planningDir)
	if err != nil {
		return false, 0, err
	}
	return len(unresolved) == 0, len(unresolved), nil
}

type ChecklistItem struct {
	ID               string `json:"id"`
	Category         string `json:"category"`
	Question         string `json:"question"`
	Status           string `json:"status"`
	Evidence         string `json:"evidence,omitempty"`
	SeverityIfMissing string `json:"severity_if_missing"`
}

type ChecklistResult struct {
	Items           []ChecklistItem `json:"items"`
	Complete        int             `json:"complete"`
	Partial         int             `json:"partial"`
	Missing         int             `json:"missing"`
	NotApplicable   int             `json:"na"`
	CriticalMissing int             `json:"critical_missing"`
}

var checklistDefinitions = []struct {
	ID       string
	Category string
	Question string
	Severity string
	Patterns []string
}{
	// C1: Structure
	{"C1.1", "structure", "Problem statement or purpose defined?", "warning", []string{"purpose", "problem", "objective", "goal", "overview", "introduction"}},
	{"C1.2", "structure", "Functional requirements explicitly listed?", "warning", []string{"must", "shall", "requirement", "feature"}},
	{"C1.3", "structure", "Non-functional requirements stated?", "info", []string{"performance", "security", "scalab", "reliab", "availability"}},
	{"C1.4", "structure", "Scope clearly bounded?", "warning", []string{"scope", "out of scope", "not included", "boundaries", "limitations"}},
	// C2: Data Model
	{"C2.1", "data_model", "Entities/tables defined with purpose?", "critical", []string{"CREATE TABLE", "entity", "table", "model"}},
	{"C2.2", "data_model", "Fields defined with types?", "critical", []string{"str", "int", "bool", "float", "uuid", "timestamp", "VARCHAR", "INTEGER"}},
	{"C2.3", "data_model", "Required vs optional fields distinguished?", "warning", []string{"NOT NULL", "optional", "required"}},
	{"C2.4", "data_model", "Constraints stated (UNIQUE, CHECK, FK)?", "critical", []string{"UNIQUE", "CHECK", "FOREIGN KEY", "CONSTRAINT", "PRIMARY KEY"}},
	{"C2.5", "data_model", "Indexes specified for query patterns?", "warning", []string{"INDEX", "index"}},
	{"C2.6", "data_model", "Default values documented?", "info", []string{"DEFAULT", "default"}},
	// C3: API
	{"C3.1", "api", "Endpoints listed with HTTP methods?", "critical", []string{"GET /", "POST /", "PUT /", "PATCH /", "DELETE /"}},
	{"C3.2", "api", "Request schemas defined?", "critical", []string{"request", "body", "param", "schema"}},
	{"C3.3", "api", "Response schemas defined?", "critical", []string{"response", "return", "schema"}},
	{"C3.4", "api", "Error responses defined?", "warning", []string{"error response", "4xx", "5xx", "400", "401", "404", "500"}},
	{"C3.5", "api", "Authentication requirements per endpoint?", "critical", []string{"authentication", "authorization", "auth", "bearer", "api key", "jwt", "token"}},
	// C4: Behavior
	{"C4.1", "behavior", "Features described as observable behavior?", "critical", []string{"when", "then", "must", "shall", "should", "will"}},
	{"C4.2", "behavior", "State transitions defined?", "warning", []string{"state", "status", "transition", "workflow", "lifecycle"}},
	{"C4.3", "behavior", "Business rules stated?", "critical", []string{"rule", "validation", "must be", "cannot", "allowed", "prohibited"}},
	{"C4.4", "behavior", "Edge cases addressed?", "warning", []string{"edge case", "empty", "null", "zero", "maximum", "minimum", "boundary"}},
	// C5: Error Handling
	{"C5.1", "errors", "Error conditions enumerated?", "warning", []string{"error", "condition", "case", "when", "if"}},
	{"C5.2", "errors", "Error messages/codes defined?", "warning", []string{"error code", "error message", "400", "401", "404", "500"}},
	{"C5.3", "errors", "Retry behaviors specified?", "info", []string{"retry"}},
	// C6: Configuration
	{"C6.1", "config", "Environment variables listed?", "warning", []string{"environment", "env var", "ENV_"}},
	{"C6.2", "config", "Types for config values specified?", "warning", []string{"str", "int", "bool", "float", "string", "integer"}},
	{"C6.3", "config", "Defaults documented?", "info", []string{"default"}},
	// C7: Security
	{"C7.1", "security", "Authentication mechanism specified?", "critical", []string{"authentication", "authn", "login", "bearer", "jwt", "api key", "oauth"}},
	{"C7.2", "security", "Authorization rules defined?", "critical", []string{"authorization", "authz", "permission", "role", "access control", "rbac"}},
	{"C7.3", "security", "Sensitive data handling stated?", "warning", []string{"sensitive", "encrypt", "hash", "pii", "secret", "credential"}},
	// C8: Observability
	{"C8.1", "observability", "Logging requirements defined?", "info", []string{"log", "logging"}},
	{"C8.2", "observability", "Metrics specified?", "info", []string{"metric", "counter", "gauge", "histogram", "prometheus", "otel"}},
	{"C8.3", "observability", "Health check endpoints specified?", "info", []string{"health", "/health"}},
	// C9: Performance
	{"C9.1", "performance", "Response time SLAs defined?", "info", []string{"latency", "response time", "sla", "p50", "p95", "p99", "millisecond"}},
	{"C9.2", "performance", "Timeout values defined?", "info", []string{"timeout"}},
	// C10: Integration
	{"C10.1", "integration", "External dependencies listed?", "warning", []string{"external", "dependency", "third-party", "integration", "api call"}},
	{"C10.2", "integration", "External API contracts documented?", "warning", []string{"contract", "external api"}},
	// C11: Lifecycle
	{"C11.1", "lifecycle", "Startup sequence defined?", "info", []string{"startup", "initialization", "boot", "lifespan"}},
	{"C11.2", "lifecycle", "Graceful shutdown specified?", "info", []string{"shutdown", "graceful", "cleanup", "termination"}},
}

func VerifyChecklist(content string) *ChecklistResult {
	contentLower := strings.ToLower(content)

	result := &ChecklistResult{
		Items: make([]ChecklistItem, 0, len(checklistDefinitions)),
	}

	for _, def := range checklistDefinitions {
		item := ChecklistItem{
			ID:               def.ID,
			Category:         def.Category,
			Question:         def.Question,
			Status:           "missing",
			SeverityIfMissing: def.Severity,
		}

		matchCount := 0
		for _, pattern := range def.Patterns {
			patternLower := strings.ToLower(pattern)
			if strings.Contains(contentLower, patternLower) || strings.Contains(content, pattern) {
				matchCount++
			}
		}

		if matchCount >= 2 {
			item.Status = "complete"
			item.Evidence = fmt.Sprintf("Found %d matching patterns", matchCount)
			result.Complete++
		} else if matchCount == 1 {
			item.Status = "partial"
			item.Evidence = "Found partial evidence"
			result.Partial++
		} else {
			result.Missing++
			if def.Severity == "critical" {
				result.CriticalMissing++
			}
		}

		result.Items = append(result.Items, item)
	}

	return result
}

func ChecklistToWeaknesses(checklist *ChecklistResult) []Weakness {
	var weaknesses []Weakness
	counter := 0

	for _, item := range checklist.Items {
		if item.Status == "missing" && item.SeverityIfMissing == "critical" {
			counter++
			weaknesses = append(weaknesses, Weakness{
				ID:          fmt.Sprintf("CK-%s", item.ID),
				Category:    CategoryChecklist,
				Severity:    SeverityCritical,
				Location:    "spec-wide",
				Description: fmt.Sprintf("Checklist gap: %s", item.Question),
				SuggestedRes: fmt.Sprintf("Address checklist item %s", item.ID),
			})
		}
	}

	return weaknesses
}
