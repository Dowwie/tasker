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
	validateCmd.AddCommand(dagCmd)
	validateCmd.AddCommand(gatesCmd)
	validateCmd.AddCommand(steelThreadCmd)

	command.RootCmd.AddCommand(validateCmd)
}
