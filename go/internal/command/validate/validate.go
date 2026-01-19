package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dgordon/tasker/internal/command"
	statelib "github.com/dgordon/tasker/internal/state"
	"github.com/dgordon/tasker/internal/validate"
	"github.com/spf13/cobra"
)

// getPlanningDirFunc is a function variable for dependency injection in tests
var getPlanningDirFunc = command.GetPlanningDir

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validation commands",
	Long:  `Commands for validating decomposition artifacts including DAG structure and steel thread path.`,
}

var dagCmd = &cobra.Command{
	Use:   "dag",
	Short: "Validate task dependency graph",
	Long:  `Validates the task dependency graph for cycles and missing dependencies.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()

		tasks, err := loadTasksForValidation(planningDir)
		if err != nil {
			return fmt.Errorf("failed to load tasks: %w", err)
		}

		result := validate.ValidateDAG(tasks)

		if result.Valid {
			fmt.Println("DAG validation passed")
			fmt.Printf("  Tasks validated: %d\n", len(tasks))
			return nil
		}

		fmt.Println("DAG validation failed")
		for _, validationErr := range result.Errors {
			fmt.Printf("  ERROR: %s\n", validationErr.Error())
		}
		for _, warning := range result.Warnings {
			fmt.Printf("  WARNING: %s\n", warning)
		}

		return fmt.Errorf("DAG validation failed with %d errors", len(result.Errors))
	},
}

var gatesCmd = &cobra.Command{
	Use:   "gates",
	Short: "Validate phase gates",
	Long:  `Validates that all phase gates have been properly completed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()
		sm := statelib.NewStateManager(planningDir)

		state, err := sm.Load()
		if err != nil {
			return fmt.Errorf("failed to load state: %w", err)
		}

		fmt.Printf("Current phase: %s\n", state.Phase.Current)
		fmt.Printf("Completed phases: %s\n", strings.Join(state.Phase.Completed, ", "))

		if state.Artifacts.ValidationResults != nil {
			vr := state.Artifacts.ValidationResults
			fmt.Println("\nValidation Results:")

			if vr.SpecCoverage != nil {
				status := "FAIL"
				if vr.SpecCoverage.Passed {
					status = "PASS"
				}
				fmt.Printf("  Spec Coverage: %s (%.1f%% >= %.1f%%)\n",
					status, vr.SpecCoverage.Ratio*100, vr.SpecCoverage.Threshold*100)
			}

			if vr.PhaseLeakage != nil {
				status := "FAIL"
				if vr.PhaseLeakage.Passed {
					status = "PASS"
				}
				fmt.Printf("  Phase Leakage: %s\n", status)
			}

			if vr.DependencyExistence != nil {
				status := "FAIL"
				if vr.DependencyExistence.Passed {
					status = "PASS"
				}
				fmt.Printf("  Dependency Existence: %s\n", status)
			}
		}

		return nil
	},
}

var steelThreadCmd = &cobra.Command{
	Use:   "steel-thread",
	Short: "Validate steel thread path",
	Long:  `Validates that the steel thread path is properly defined and all tasks are connected.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()

		tasks, err := loadTasksForValidation(planningDir)
		if err != nil {
			return fmt.Errorf("failed to load tasks: %w", err)
		}

		steelThreadTasks := make([]string, 0)
		for _, task := range tasks {
			if task.SteelThread {
				steelThreadTasks = append(steelThreadTasks, task.ID)
			}
		}

		if len(steelThreadTasks) == 0 {
			fmt.Println("No steel thread tasks defined")
			return nil
		}

		validationErr := validate.ValidateSteelThread(tasks)
		if validationErr != nil {
			fmt.Println("Steel thread validation failed")
			fmt.Printf("  ERROR: %s\n", validationErr.Error())
			return fmt.Errorf("steel thread validation failed: %s", validationErr.Message)
		}

		order, err := validate.TopologicalSort(filterSteelThread(tasks))
		if err != nil {
			return fmt.Errorf("failed to compute steel thread order: %w", err)
		}

		fmt.Println("Steel thread validation passed")
		fmt.Printf("  Steel thread tasks: %d\n", len(steelThreadTasks))
		fmt.Printf("  Execution order: %s\n", strings.Join(order, " -> "))

		return nil
	},
}

var specCoverageThreshold float64

var specCoverageCmd = &cobra.Command{
	Use:   "spec-coverage",
	Short: "Check spec requirement coverage",
	Long:  `Validates that tasks cover the required percentage of spec behaviors.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()

		tasks, err := validate.LoadTaskDefinitions(planningDir)
		if err != nil {
			return fmt.Errorf("failed to load tasks: %w", err)
		}

		capMap, err := validate.LoadCapabilityMap(planningDir)
		if err != nil {
			fmt.Printf("Warning: %v\n", err)
			fmt.Println("Spec coverage validation skipped (no capability map)")
			return nil
		}

		result := validate.CheckSpecCoverage(tasks, capMap, specCoverageThreshold)

		fmt.Println("Spec Coverage Report")
		fmt.Println(strings.Repeat("=", 40))
		fmt.Printf("Coverage: %.1f%%\n", result.Ratio*100)
		fmt.Printf("Threshold: %.1f%%\n", result.Threshold*100)
		fmt.Printf("Covered: %d/%d behaviors\n", result.CoveredBehaviors, result.TotalBehaviors)

		if len(result.UncoveredBehaviors) > 0 {
			fmt.Println("\nUncovered behaviors:")
			for _, b := range result.UncoveredBehaviors {
				fmt.Printf("  - %s\n", b)
			}
		}

		if len(result.CoverageByDomain) > 0 {
			fmt.Println("\nCoverage by domain:")
			for domain, coverage := range result.CoverageByDomain {
				fmt.Printf("  %s: %.1f%%\n", domain, coverage*100)
			}
		}

		if result.Passed {
			fmt.Println("\nSpec coverage: PASS")
			return nil
		}

		fmt.Println("\nSpec coverage: FAIL")
		return fmt.Errorf("spec coverage %.1f%% below threshold %.1f%%", result.Ratio*100, result.Threshold*100)
	},
}

var phaseLeakageCmd = &cobra.Command{
	Use:   "phase-leakage",
	Short: "Detect Phase 2+ content in Phase 1 tasks",
	Long:  `Validates that Phase 1 tasks don't contain content that belongs in later phases.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()

		tasks, err := validate.LoadTaskDefinitions(planningDir)
		if err != nil {
			return fmt.Errorf("failed to load tasks: %w", err)
		}

		result := validate.DetectPhaseLeakage(tasks, 1, nil)

		if result.Passed {
			fmt.Println("No phase leakage detected")
			return nil
		}

		fmt.Println("Phase leakage violations:")
		for _, v := range result.Violations {
			fmt.Printf("  %s: %s\n", v.TaskID, v.Behavior)
			fmt.Printf("    Evidence: %s\n", v.Evidence)
		}

		return fmt.Errorf("phase leakage detected: %d violation(s)", len(result.Violations))
	},
}

var dependencyExistenceCmd = &cobra.Command{
	Use:   "dependency-existence",
	Short: "Verify all task dependencies exist",
	Long:  `Validates that all declared task dependencies reference existing tasks.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()

		tasks, err := loadTasksForValidation(planningDir)
		if err != nil {
			return fmt.Errorf("failed to load tasks: %w", err)
		}

		missingErrs := validate.CheckDependencyExistence(tasks)

		if len(missingErrs) == 0 {
			fmt.Println("All dependencies exist")
			return nil
		}

		fmt.Println("Missing dependency violations:")
		for _, e := range missingErrs {
			fmt.Printf("  %s: depends on non-existent %v\n", e.TaskID, e.MissingDeps)
		}

		return fmt.Errorf("missing dependencies: %d task(s) reference non-existent dependencies", len(missingErrs))
	},
}

var acceptanceCriteriaCmd = &cobra.Command{
	Use:   "acceptance-criteria",
	Short: "Check acceptance criteria quality",
	Long:  `Validates that acceptance criteria are specific, measurable, and have valid verification commands.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()

		tasks, err := validate.LoadTaskDefinitions(planningDir)
		if err != nil {
			return fmt.Errorf("failed to load tasks: %w", err)
		}

		result := validate.ValidateAcceptanceCriteria(tasks)

		if result.Passed {
			fmt.Println("All acceptance criteria meet quality standards")
			return nil
		}

		fmt.Println("Acceptance criteria quality issues:")
		for _, issue := range result.Issues {
			if issue.CriterionIndex >= 0 {
				fmt.Printf("  %s criterion %d: %s\n", issue.TaskID, issue.CriterionIndex, issue.Issue)
			} else {
				fmt.Printf("  %s: %s\n", issue.TaskID, issue.Issue)
			}
		}

		return fmt.Errorf("acceptance criteria quality issues: %d problem(s)", len(result.Issues))
	},
}

var planningGatesThreshold float64

var verificationCommandsCmd = &cobra.Command{
	Use:   "verification-commands",
	Short: "Validate verification command syntax",
	Long:  `Validates that verification commands in acceptance criteria are parseable and use recognized tools.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()

		tasks, err := validate.LoadTaskDefinitions(planningDir)
		if err != nil {
			return fmt.Errorf("failed to load tasks: %w", err)
		}

		result := validate.ValidateAcceptanceCriteria(tasks)

		verificationIssues := []validate.ACQualityIssue{}
		for _, issue := range result.Issues {
			if strings.Contains(issue.Issue, "verification") || strings.Contains(issue.Issue, "Verification") {
				verificationIssues = append(verificationIssues, issue)
			}
		}

		if len(verificationIssues) == 0 {
			fmt.Println("All verification commands are valid")
			return nil
		}

		fmt.Println("Verification command issues:")
		for _, issue := range verificationIssues {
			if issue.CriterionIndex >= 0 {
				fmt.Printf("  %s criterion %d: %s\n", issue.TaskID, issue.CriterionIndex, issue.Issue)
			} else {
				fmt.Printf("  %s: %s\n", issue.TaskID, issue.Issue)
			}
		}

		return fmt.Errorf("verification command issues: %d problem(s)", len(verificationIssues))
	},
}

var allCmd = &cobra.Command{
	Use:   "all",
	Short: "Run all validations",
	Long:  `Runs all validation checks: DAG, steel thread, spec coverage, phase leakage, dependency existence, and acceptance criteria.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()

		fmt.Println("Running all validations...")
		fmt.Println(strings.Repeat("=", 60))
		fmt.Println()

		allPassed := true

		tasks, err := loadTasksForValidation(planningDir)
		if err != nil {
			fmt.Printf("DAG Validation: SKIP (failed to load tasks: %v)\n", err)
		} else {
			dagResult := validate.ValidateDAG(tasks)
			if dagResult.Valid {
				fmt.Println("DAG Validation: PASS")
			} else {
				fmt.Println("DAG Validation: FAIL")
				for _, e := range dagResult.Errors {
					fmt.Printf("  - %s\n", e.Error())
				}
				allPassed = false
			}
		}
		fmt.Println()

		tasks2, _ := loadTasksForValidation(planningDir)
		steelThreadTasks := make([]string, 0)
		for _, task := range tasks2 {
			if task.SteelThread {
				steelThreadTasks = append(steelThreadTasks, task.ID)
			}
		}
		if len(steelThreadTasks) == 0 {
			fmt.Println("Steel Thread Validation: SKIP (no steel thread tasks)")
		} else {
			stErr := validate.ValidateSteelThread(tasks2)
			if stErr == nil {
				fmt.Println("Steel Thread Validation: PASS")
			} else {
				fmt.Println("Steel Thread Validation: FAIL")
				fmt.Printf("  - %s\n", stErr.Message)
				allPassed = false
			}
		}
		fmt.Println()

		result, err := validate.RunAllGates(planningDir, 1, 0.9)
		if err != nil {
			fmt.Printf("Planning Gates: ERROR (%v)\n", err)
		} else {
			for _, gate := range result.Gates {
				status := "PASS"
				if !gate.Passed {
					status = "FAIL"
					allPassed = false
				}
				gateName := strings.ReplaceAll(gate.Name, "_", " ")
				fmt.Printf("%s: %s\n", gateName, status)
			}
		}
		fmt.Println()

		fmt.Println(strings.Repeat("-", 60))
		if allPassed {
			fmt.Println("All validations PASSED")
			return nil
		}

		fmt.Println("Some validations FAILED")
		return fmt.Errorf("validation failed")
	},
}

var refactorPriorityCmd = &cobra.Command{
	Use:   "refactor-priority",
	Short: "Show refactor override resolution",
	Long:  `Shows how refactor tasks override original spec requirements.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()
		tasksDir := filepath.Join(planningDir, "tasks")

		entries, err := os.ReadDir(tasksDir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No tasks directory found")
				return nil
			}
			return fmt.Errorf("failed to read tasks directory: %w", err)
		}

		type override struct {
			TaskID     string   `json:"task_id"`
			Supersedes []string `json:"supersedes"`
			Directive  string   `json:"directive"`
		}

		var overrides []override
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}

			taskPath := filepath.Join(tasksDir, entry.Name())
			data, err := os.ReadFile(taskPath)
			if err != nil {
				continue
			}

			var task map[string]interface{}
			if err := json.Unmarshal(data, &task); err != nil {
				continue
			}

			taskType, _ := task["task_type"].(string)
			if taskType != "refactor" {
				continue
			}

			taskID, _ := task["id"].(string)
			refactorCtx, ok := task["refactor_context"].(map[string]interface{})
			if !ok {
				continue
			}

			var supersedes []string
			if sections, ok := refactorCtx["original_spec_sections"].([]interface{}); ok {
				for _, s := range sections {
					if str, ok := s.(string); ok {
						supersedes = append(supersedes, str)
					}
				}
			}

			directive, _ := refactorCtx["refactor_directive"].(string)

			overrides = append(overrides, override{
				TaskID:     taskID,
				Supersedes: supersedes,
				Directive:  directive,
			})
		}

		fmt.Println("Refactor Priority Resolution")
		fmt.Println(strings.Repeat("=", 40))
		fmt.Printf("Refactor overrides: %d\n", len(overrides))

		if len(overrides) > 0 {
			fmt.Println("\nOverrides:")
			for _, o := range overrides {
				fmt.Printf("  %s: supersedes %v\n", o.TaskID, o.Supersedes)
				if o.Directive != "" {
					directive := o.Directive
					if len(directive) > 60 {
						directive = directive[:60] + "..."
					}
					fmt.Printf("    Directive: %s\n", directive)
				}
			}
		}

		return nil
	},
}

var planningGatesCmd = &cobra.Command{
	Use:   "planning-gates",
	Short: "Run all planning validation gates",
	Long:  `Runs all planning validation gates: spec coverage, phase leakage, dependency existence, and acceptance criteria.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()

		result, err := validate.RunAllGates(planningDir, 1, planningGatesThreshold)
		if err != nil {
			return fmt.Errorf("failed to run planning gates: %w", err)
		}

		fmt.Println("Planning Gates Report")
		fmt.Println(strings.Repeat("=", 40))
		fmt.Println()

		if result.SpecCoverage != nil {
			status := "FAIL"
			if result.SpecCoverage.Passed {
				status = "PASS"
			}
			fmt.Printf("Spec Coverage: %s\n", status)
			fmt.Printf("  Coverage: %.1f%%\n", result.SpecCoverage.Ratio*100)
		}
		fmt.Println()

		if result.PhaseLeakage != nil {
			status := "FAIL"
			if result.PhaseLeakage.Passed {
				status = "PASS"
			}
			fmt.Printf("Phase Leakage: %s\n", status)
			if len(result.PhaseLeakage.Violations) > 0 {
				fmt.Printf("  Violations: %d\n", len(result.PhaseLeakage.Violations))
			}
		}
		fmt.Println()

		if result.ACValidation != nil {
			status := "FAIL"
			if result.ACValidation.Passed {
				status = "PASS"
			}
			fmt.Printf("Acceptance Criteria: %s\n", status)
			if len(result.ACValidation.Issues) > 0 {
				fmt.Printf("  Issues: %d\n", len(result.ACValidation.Issues))
			}
		}
		fmt.Println()

		fmt.Println(strings.Repeat("-", 40))
		if result.Passed {
			fmt.Println("All planning gates PASSED")
			return nil
		}

		fmt.Println("Planning gates BLOCKED:")
		for _, gate := range result.Gates {
			if !gate.Passed {
				fmt.Printf("  - %s", gate.Name)
				if gate.Error != "" {
					fmt.Printf(": %s", gate.Error)
				}
				fmt.Println()
			}
		}

		return fmt.Errorf("planning gates failed")
	},
}

type taskDefinition struct {
	ID          string   `json:"id"`
	DependsOn   []string `json:"depends_on,omitempty"`
	Blocks      []string `json:"blocks,omitempty"`
	SteelThread bool     `json:"steel_thread,omitempty"`
}

func loadTasksForValidation(planningDir string) (map[string]validate.Task, error) {
	tasksDir := filepath.Join(planningDir, "tasks")

	if _, err := os.Stat(tasksDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("tasks directory not found: %s", tasksDir)
	}

	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks directory: %w", err)
	}

	tasks := make(map[string]validate.Task)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		taskPath := filepath.Join(tasksDir, entry.Name())
		data, err := os.ReadFile(taskPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read task file %s: %w", entry.Name(), err)
		}

		var def taskDefinition
		if err := json.Unmarshal(data, &def); err != nil {
			return nil, fmt.Errorf("failed to parse task file %s: %w", entry.Name(), err)
		}

		if def.ID == "" {
			return nil, fmt.Errorf("task file %s missing required field 'id'", entry.Name())
		}

		tasks[def.ID] = validate.Task{
			ID:          def.ID,
			DependsOn:   def.DependsOn,
			Blocks:      def.Blocks,
			SteelThread: def.SteelThread,
		}
	}

	return tasks, nil
}

func filterSteelThread(tasks map[string]validate.Task) map[string]validate.Task {
	result := make(map[string]validate.Task)
	for id, task := range tasks {
		if task.SteelThread {
			result[id] = task
		}
	}
	return result
}

func init() {
	specCoverageCmd.Flags().Float64Var(&specCoverageThreshold, "threshold", 0.9, "Minimum coverage threshold (0.0-1.0)")
	planningGatesCmd.Flags().Float64Var(&planningGatesThreshold, "threshold", 0.9, "Minimum spec coverage threshold (0.0-1.0)")

	validateCmd.AddCommand(dagCmd)
	validateCmd.AddCommand(gatesCmd)
	validateCmd.AddCommand(steelThreadCmd)
	validateCmd.AddCommand(specCoverageCmd)
	validateCmd.AddCommand(phaseLeakageCmd)
	validateCmd.AddCommand(dependencyExistenceCmd)
	validateCmd.AddCommand(acceptanceCriteriaCmd)
	validateCmd.AddCommand(planningGatesCmd)
	validateCmd.AddCommand(verificationCommandsCmd)
	validateCmd.AddCommand(allCmd)
	validateCmd.AddCommand(refactorPriorityCmd)

	command.RootCmd.AddCommand(validateCmd)
}
