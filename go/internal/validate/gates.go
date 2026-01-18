package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type SpecRequirement struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type TaskDefinition struct {
	ID                 string               `json:"id"`
	Name               string               `json:"name,omitempty"`
	Phase              int                  `json:"phase"`
	Behaviors          []string             `json:"behaviors,omitempty"`
	AcceptanceCriteria []AcceptanceCriterion `json:"acceptance_criteria,omitempty"`
	Context            TaskContext          `json:"context,omitempty"`
}

type TaskContext struct {
	Domain      string `json:"domain,omitempty"`
	Capability  string `json:"capability,omitempty"`
	SteelThread bool   `json:"steel_thread,omitempty"`
}

type AcceptanceCriterion struct {
	Criterion    string `json:"criterion"`
	Verification string `json:"verification"`
}

type Behavior struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type Capability struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Behaviors []Behavior `json:"behaviors,omitempty"`
}

type Domain struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Capabilities []Capability `json:"capabilities,omitempty"`
}

type CapabilityMap struct {
	Version string   `json:"version"`
	Domains []Domain `json:"domains,omitempty"`
}

type SpecCoverageResult struct {
	Passed            bool              `json:"passed"`
	Ratio             float64           `json:"ratio"`
	Threshold         float64           `json:"threshold"`
	TotalBehaviors    int               `json:"total_behaviors"`
	CoveredBehaviors  int               `json:"covered_behaviors"`
	UncoveredBehaviors []string         `json:"uncovered_behaviors,omitempty"`
	CoverageByDomain  map[string]float64 `json:"coverage_by_domain,omitempty"`
}

type PhaseLeakageViolation struct {
	TaskID   string `json:"task_id"`
	Behavior string `json:"behavior"`
	Evidence string `json:"evidence"`
}

type PhaseLeakageResult struct {
	Passed     bool                    `json:"passed"`
	Violations []PhaseLeakageViolation `json:"violations,omitempty"`
}

type ACQualityIssue struct {
	TaskID         string `json:"task_id"`
	CriterionIndex int    `json:"criterion_index"`
	Issue          string `json:"issue"`
}

type ACValidationResult struct {
	Passed bool             `json:"passed"`
	Issues []ACQualityIssue `json:"issues,omitempty"`
}

type GateResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Error  string `json:"error,omitempty"`
}

type AllGatesResult struct {
	Passed            bool               `json:"passed"`
	Gates             []GateResult       `json:"gates"`
	SpecCoverage      *SpecCoverageResult `json:"spec_coverage,omitempty"`
	PhaseLeakage      *PhaseLeakageResult `json:"phase_leakage,omitempty"`
	ACValidation      *ACValidationResult `json:"ac_validation,omitempty"`
}

func CheckSpecCoverage(tasks []TaskDefinition, capabilityMap *CapabilityMap, threshold float64) *SpecCoverageResult {
	if capabilityMap == nil {
		return &SpecCoverageResult{
			Passed:    false,
			Ratio:     0,
			Threshold: threshold,
		}
	}

	allBehaviors := make(map[string]string)
	behaviorToDomain := make(map[string]string)
	for _, domain := range capabilityMap.Domains {
		for _, cap := range domain.Capabilities {
			for _, behavior := range cap.Behaviors {
				allBehaviors[behavior.ID] = behavior.Name
				behaviorToDomain[behavior.ID] = domain.ID
			}
		}
	}

	coveredBehaviors := make(map[string]bool)
	for _, task := range tasks {
		for _, behaviorID := range task.Behaviors {
			if _, exists := allBehaviors[behaviorID]; exists {
				coveredBehaviors[behaviorID] = true
			}
		}
	}

	total := len(allBehaviors)
	covered := len(coveredBehaviors)
	ratio := 0.0
	if total > 0 {
		ratio = float64(covered) / float64(total)
	}

	var uncovered []string
	for id, name := range allBehaviors {
		if !coveredBehaviors[id] {
			uncovered = append(uncovered, fmt.Sprintf("%s (%s)", id, name))
		}
	}
	sort.Strings(uncovered)

	domainCoverage := make(map[string]float64)
	domainTotal := make(map[string]int)
	domainCovered := make(map[string]int)
	for id, domainID := range behaviorToDomain {
		domainTotal[domainID]++
		if coveredBehaviors[id] {
			domainCovered[domainID]++
		}
	}
	for domainID, total := range domainTotal {
		if total > 0 {
			domainCoverage[domainID] = float64(domainCovered[domainID]) / float64(total)
		}
	}

	return &SpecCoverageResult{
		Passed:            ratio >= threshold,
		Ratio:             ratio,
		Threshold:         threshold,
		TotalBehaviors:    total,
		CoveredBehaviors:  covered,
		UncoveredBehaviors: uncovered,
		CoverageByDomain:  domainCoverage,
	}
}

func DetectPhaseLeakage(tasks []TaskDefinition, currentPhase int, phaseKeywords map[int][]string) *PhaseLeakageResult {
	result := &PhaseLeakageResult{
		Passed: true,
	}

	if phaseKeywords == nil {
		phaseKeywords = getDefaultPhaseKeywords()
	}

	for _, task := range tasks {
		if task.Phase != currentPhase {
			continue
		}

		taskText := strings.ToLower(task.Name)
		for _, ac := range task.AcceptanceCriteria {
			taskText += " " + strings.ToLower(ac.Criterion)
			taskText += " " + strings.ToLower(ac.Verification)
		}

		for phase, keywords := range phaseKeywords {
			if phase <= currentPhase {
				continue
			}
			for _, keyword := range keywords {
				if strings.Contains(taskText, strings.ToLower(keyword)) {
					result.Passed = false
					result.Violations = append(result.Violations, PhaseLeakageViolation{
						TaskID:   task.ID,
						Behavior: "phase_leakage",
						Evidence: fmt.Sprintf("Phase %d keyword '%s' found in Phase %d task", phase, keyword, currentPhase),
					})
				}
			}
		}
	}

	return result
}

func getDefaultPhaseKeywords() map[int][]string {
	return map[int][]string{
		2: {"deployment", "production", "scale", "performance optimization"},
		3: {"migration", "deprecation", "backward compatibility"},
	}
}

func ValidateAcceptanceCriteria(tasks []TaskDefinition) *ACValidationResult {
	result := &ACValidationResult{
		Passed: true,
	}

	verificationPattern := regexp.MustCompile(`^(go\s+test|pytest|npm\s+test|make\s+test|cargo\s+test|bash|sh|\./)`)

	for _, task := range tasks {
		if len(task.AcceptanceCriteria) == 0 {
			result.Passed = false
			result.Issues = append(result.Issues, ACQualityIssue{
				TaskID:         task.ID,
				CriterionIndex: -1,
				Issue:          "Task has no acceptance criteria",
			})
			continue
		}

		for i, ac := range task.AcceptanceCriteria {
			if strings.TrimSpace(ac.Criterion) == "" {
				result.Passed = false
				result.Issues = append(result.Issues, ACQualityIssue{
					TaskID:         task.ID,
					CriterionIndex: i,
					Issue:          "Empty criterion text",
				})
			}

			if strings.TrimSpace(ac.Verification) == "" {
				result.Passed = false
				result.Issues = append(result.Issues, ACQualityIssue{
					TaskID:         task.ID,
					CriterionIndex: i,
					Issue:          "Missing verification command",
				})
			} else if !verificationPattern.MatchString(ac.Verification) {
				result.Issues = append(result.Issues, ACQualityIssue{
					TaskID:         task.ID,
					CriterionIndex: i,
					Issue:          fmt.Sprintf("Verification command may not be executable: %s", ac.Verification),
				})
			}

			if len(strings.TrimSpace(ac.Criterion)) < 10 {
				result.Passed = false
				result.Issues = append(result.Issues, ACQualityIssue{
					TaskID:         task.ID,
					CriterionIndex: i,
					Issue:          "Criterion too short (less than 10 characters)",
				})
			}
		}
	}

	return result
}

func RunAllGates(planningDir string, currentPhase int, coverageThreshold float64) (*AllGatesResult, error) {
	result := &AllGatesResult{
		Passed: true,
	}

	tasks, err := LoadTaskDefinitions(planningDir)
	if err != nil {
		result.Passed = false
		result.Gates = append(result.Gates, GateResult{
			Name:   "load_tasks",
			Passed: false,
			Error:  err.Error(),
		})
		return result, nil
	}

	capMap, err := LoadCapabilityMap(planningDir)
	if err != nil {
		result.Gates = append(result.Gates, GateResult{
			Name:   "load_capability_map",
			Passed: false,
			Error:  err.Error(),
		})
	}

	specCoverage := CheckSpecCoverage(tasks, capMap, coverageThreshold)
	result.SpecCoverage = specCoverage
	result.Gates = append(result.Gates, GateResult{
		Name:   "spec_coverage",
		Passed: specCoverage.Passed,
	})
	if !specCoverage.Passed {
		result.Passed = false
	}

	phaseLeakage := DetectPhaseLeakage(tasks, currentPhase, nil)
	result.PhaseLeakage = phaseLeakage
	result.Gates = append(result.Gates, GateResult{
		Name:   "phase_leakage",
		Passed: phaseLeakage.Passed,
	})
	if !phaseLeakage.Passed {
		result.Passed = false
	}

	acValidation := ValidateAcceptanceCriteria(tasks)
	result.ACValidation = acValidation
	result.Gates = append(result.Gates, GateResult{
		Name:   "acceptance_criteria",
		Passed: acValidation.Passed,
	})
	if !acValidation.Passed {
		result.Passed = false
	}

	return result, nil
}

func LoadTaskDefinitions(planningDir string) ([]TaskDefinition, error) {
	tasksDir := filepath.Join(planningDir, "tasks")

	if _, err := os.Stat(tasksDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("tasks directory not found: %s", tasksDir)
	}

	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks directory: %w", err)
	}

	var tasks []TaskDefinition
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		taskPath := filepath.Join(tasksDir, entry.Name())
		data, err := os.ReadFile(taskPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read task file %s: %w", entry.Name(), err)
		}

		var task TaskDefinition
		if err := json.Unmarshal(data, &task); err != nil {
			return nil, fmt.Errorf("failed to parse task file %s: %w", entry.Name(), err)
		}

		tasks = append(tasks, task)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].ID < tasks[j].ID
	})

	return tasks, nil
}

func LoadCapabilityMap(planningDir string) (*CapabilityMap, error) {
	capMapPath := filepath.Join(planningDir, "artifacts", "capability-map.json")

	data, err := os.ReadFile(capMapPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("capability map not found: %s", capMapPath)
		}
		return nil, fmt.Errorf("failed to read capability map: %w", err)
	}

	var capMap CapabilityMap
	if err := json.Unmarshal(data, &capMap); err != nil {
		return nil, fmt.Errorf("failed to parse capability map: %w", err)
	}

	return &capMap, nil
}
