package fsm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dgordon/tasker/internal/command"
	"github.com/dgordon/tasker/internal/errors"
	fsmlib "github.com/dgordon/tasker/internal/fsm"
	"github.com/dgordon/tasker/internal/schema"
	"github.com/spf13/cobra"
)

// getPlanningDirFunc is a function variable for dependency injection in tests
var getPlanningDirFunc = command.GetPlanningDir

var (
	outputDir string
	slug      string
)

var fsmCmd = &cobra.Command{
	Use:   "fsm",
	Short: "FSM compilation and management",
	Long:  `Commands for compiling, validating, and visualizing finite state machines from specifications.`,
}

var compileCmd = &cobra.Command{
	Use:   "compile <spec.json>",
	Short: "Compile FSM from spec JSON",
	Long:  `Compiles a finite state machine from a structured spec JSON file with workflows.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		specPath := args[0]

		data, err := os.ReadFile(specPath)
		if err != nil {
			return errors.IOReadFailed(specPath, err)
		}

		var specData fsmlib.SpecData
		if err := json.Unmarshal(data, &specData); err != nil {
			return errors.Internal("failed to parse spec JSON", err)
		}

		specSlug := slug
		if specSlug == "" {
			base := filepath.Base(specPath)
			specSlug = strings.TrimSuffix(base, filepath.Ext(base))
		}
		specChecksum := fsmlib.ComputeChecksum(string(data))

		compiler := fsmlib.NewCompiler()
		_, err = compiler.CompileFromSpec(&specData)
		if err != nil {
			return err
		}

		outDir := outputDir
		if outDir == "" {
			outDir = filepath.Join(getPlanningDirFunc(), "fsm")
		}

		index, err := compiler.Export(outDir, specSlug, specChecksum)
		if err != nil {
			return err
		}

		result := map[string]interface{}{
			"status": "success",
			"index":  index,
		}
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(output))

		return nil
	},
}

var fromCapMapCmd = &cobra.Command{
	Use:   "from-capability-map <capability-map.json> [spec.md]",
	Short: "Compile FSM from capability map",
	Long:  `Compiles a finite state machine from a capability map JSON file, optionally using spec markdown for context.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		capMapPath := args[0]

		data, err := os.ReadFile(capMapPath)
		if err != nil {
			return errors.IOReadFailed(capMapPath, err)
		}

		var capMap fsmlib.CapabilityMap
		if err := json.Unmarshal(data, &capMap); err != nil {
			return errors.Internal("failed to parse capability map JSON", err)
		}

		var specText string
		if len(args) > 1 {
			specData, err := os.ReadFile(args[1])
			if err == nil {
				specText = string(specData)
			}
		}

		specSlug := slug
		if specSlug == "" {
			base := filepath.Base(capMapPath)
			specSlug = strings.TrimSuffix(base, filepath.Ext(base))
			specSlug = strings.TrimSuffix(specSlug, ".capabilities")
		}
		specChecksum := capMap.SpecChecksum
		if specChecksum == "" {
			specChecksum = fsmlib.ComputeChecksum(string(data))
		}

		compiler := fsmlib.NewCompiler()
		_, err = compiler.CompileFromCapabilityMap(&capMap, specText)
		if err != nil {
			return err
		}

		outDir := outputDir
		if outDir == "" {
			outDir = filepath.Join(getPlanningDirFunc(), "fsm")
		}

		index, err := compiler.Export(outDir, specSlug, specChecksum)
		if err != nil {
			return err
		}

		result := map[string]interface{}{
			"status": "success",
			"index":  index,
		}
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(output))

		return nil
	},
}

var validateCmd = &cobra.Command{
	Use:   "validate <fsm-dir>",
	Short: "Validate FSM artifacts",
	Long:  `Validates FSM artifacts (index, states, transitions) against their schemas.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fsmDir := args[0]

		indexPath := filepath.Join(fsmDir, "index.json")
		if _, err := os.Stat(indexPath); os.IsNotExist(err) {
			return errors.IONotExists(indexPath)
		}

		validator := schema.Default()
		if validator == nil {
			return errors.ConfigMissing("schema directory not configured")
		}

		result, err := validator.ValidateFile(schema.SchemaFSMIndex, indexPath)
		if err != nil {
			return err
		}

		fmt.Printf("index.json: ")
		if result.Valid {
			fmt.Println("VALID")
		} else {
			fmt.Println("INVALID")
			for _, e := range result.Errors {
				fmt.Printf("  %s: %s\n", e.Path, e.Message)
			}
		}

		indexData, err := os.ReadFile(indexPath)
		if err != nil {
			return errors.IOReadFailed(indexPath, err)
		}

		var index fsmlib.IndexFile
		if err := json.Unmarshal(indexData, &index); err != nil {
			return errors.Internal("failed to parse index.json", err)
		}

		allValid := result.Valid

		for _, machine := range index.Machines {
			if statesFile, ok := machine.Files["states"]; ok {
				statesPath := filepath.Join(fsmDir, statesFile)
				if _, err := os.Stat(statesPath); err == nil {
					statesResult, err := validator.ValidateFile(schema.SchemaFSMStates, statesPath)
					if err != nil {
						fmt.Printf("%s: ERROR - %v\n", statesFile, err)
						allValid = false
					} else if statesResult.Valid {
						fmt.Printf("%s: VALID\n", statesFile)
					} else {
						fmt.Printf("%s: INVALID\n", statesFile)
						for _, e := range statesResult.Errors {
							fmt.Printf("  %s: %s\n", e.Path, e.Message)
						}
						allValid = false
					}
				}
			}

			if transFile, ok := machine.Files["transitions"]; ok {
				transPath := filepath.Join(fsmDir, transFile)
				if _, err := os.Stat(transPath); err == nil {
					transResult, err := validator.ValidateFile(schema.SchemaFSMTransitions, transPath)
					if err != nil {
						fmt.Printf("%s: ERROR - %v\n", transFile, err)
						allValid = false
					} else if transResult.Valid {
						fmt.Printf("%s: VALID\n", transFile)
					} else {
						fmt.Printf("%s: INVALID\n", transFile)
						for _, e := range transResult.Errors {
							fmt.Printf("  %s: %s\n", e.Path, e.Message)
						}
						allValid = false
					}
				}
			}
		}

		if !allValid {
			return errors.ValidationFailed("FSM validation failed")
		}

		fmt.Println("\nAll FSM artifacts are valid")
		return nil
	},
}

var (
	mermaidOutputDir string
	mermaidNotes     bool
)

var mermaidCmd = &cobra.Command{
	Use:   "mermaid <fsm-dir>",
	Short: "Generate Mermaid diagrams and notes from FSM",
	Long:  `Generates Mermaid state diagrams (.mmd) and optional notes (.notes.md) from FSM artifacts.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fsmDir := args[0]

		indexPath := filepath.Join(fsmDir, "index.json")
		indexData, err := os.ReadFile(indexPath)
		if err != nil {
			return errors.IOReadFailed(indexPath, err)
		}

		var index fsmlib.IndexFile
		if err := json.Unmarshal(indexData, &index); err != nil {
			return errors.Internal("failed to parse index.json", err)
		}

		outDir := mermaidOutputDir
		if outDir == "" {
			outDir = fsmDir
		}

		if err := os.MkdirAll(outDir, 0755); err != nil {
			return errors.Internal("failed to create output directory", err)
		}

		var generated []string

		for i := range index.Machines {
			machine := &index.Machines[i]
			statesFile := machine.GetStatesFile()
			transFile := machine.GetTransitionsFile()

			if statesFile == "" || transFile == "" {
				continue
			}

			statesPath := filepath.Join(fsmDir, statesFile)
			statesData, err := os.ReadFile(statesPath)
			if err != nil {
				return errors.IOReadFailed(statesPath, err)
			}

			var statesFileData fsmlib.StatesFile
			if err := json.Unmarshal(statesData, &statesFileData); err != nil {
				return errors.Internal("failed to parse states file", err)
			}

			transPath := filepath.Join(fsmDir, transFile)
			transData, err := os.ReadFile(transPath)
			if err != nil {
				return errors.IOReadFailed(transPath, err)
			}

			var transFileData fsmlib.TransitionsFile
			if err := json.Unmarshal(transData, &transFileData); err != nil {
				return errors.Internal("failed to parse transitions file", err)
			}

			mermaid := fsmlib.GenerateMermaid(machine.Name, &statesFileData, &transFileData)

			diagramFile := machine.GetDiagramFile()
			if diagramFile == "" {
				diagramFile = machine.ID + ".mmd"
			}
			diagramPath := filepath.Join(outDir, diagramFile)
			if err := os.WriteFile(diagramPath, []byte(mermaid), 0644); err != nil {
				return errors.Internal("failed to write diagram file", err)
			}
			generated = append(generated, diagramPath)

			if mermaidNotes {
				notes := fsmlib.GenerateNotes(machine.Name, &statesFileData, &transFileData)

				notesFile := machine.GetNotesFile()
				if notesFile == "" {
					notesFile = machine.ID + ".notes.md"
				}
				notesPath := filepath.Join(outDir, notesFile)
				if err := os.WriteFile(notesPath, []byte(notes), 0644); err != nil {
					return errors.Internal("failed to write notes file", err)
				}
				generated = append(generated, notesPath)
			}
		}

		result := map[string]interface{}{
			"status":    "success",
			"generated": generated,
		}
		output, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(output))

		return nil
	},
}

var (
	coverageOutput      string
	steelThreadThresh   float64
	nonSteelThreadThresh float64
)

var coverageReportCmd = &cobra.Command{
	Use:   "coverage-report <fsm-dir> <tasks-dir>",
	Short: "Generate FSM coverage report",
	Long:  `Generates a report showing which FSM transitions are covered by tasks.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fsmDir := args[0]
		tasksDir := args[1]

		indexPath := filepath.Join(fsmDir, "index.json")
		indexData, err := os.ReadFile(indexPath)
		if err != nil {
			return errors.IOReadFailed(indexPath, err)
		}

		var index fsmlib.IndexFile
		if err := json.Unmarshal(indexData, &index); err != nil {
			return errors.Internal("failed to parse index.json", err)
		}

		tasksCoverage, err := extractTaskCoverage(tasksDir)
		if err != nil {
			return err
		}

		result, err := fsmlib.CheckTaskCoverage(&index, fsmDir, tasksCoverage, steelThreadThresh, nonSteelThreadThresh)
		if err != nil {
			return err
		}

		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}

		if coverageOutput != "" {
			if err := os.MkdirAll(filepath.Dir(coverageOutput), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(coverageOutput, output, 0644); err != nil {
				return err
			}
			fmt.Printf("Coverage report written to: %s\n", coverageOutput)
		} else {
			fmt.Println(string(output))
		}

		if !result.Passed {
			return errors.ValidationFailed("FSM coverage below threshold")
		}
		return nil
	},
}

var taskCoverageCmd = &cobra.Command{
	Use:   "task-coverage <fsm-dir> <tasks-dir>",
	Short: "Validate task coverage of FSM transitions",
	Long:  `Validates that tasks adequately cover FSM transitions.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fsmDir := args[0]
		tasksDir := args[1]

		indexPath := filepath.Join(fsmDir, "index.json")
		indexData, err := os.ReadFile(indexPath)
		if err != nil {
			return errors.IOReadFailed(indexPath, err)
		}

		var index fsmlib.IndexFile
		if err := json.Unmarshal(indexData, &index); err != nil {
			return errors.Internal("failed to parse index.json", err)
		}

		tasksCoverage, err := extractTaskCoverage(tasksDir)
		if err != nil {
			return err
		}

		result, err := fsmlib.CheckTaskCoverage(&index, fsmDir, tasksCoverage, steelThreadThresh, nonSteelThreadThresh)
		if err != nil {
			return err
		}

		if result.Passed {
			fmt.Println("Task coverage validation PASSED")
			if result.SteelThreadCoverage != nil {
				fmt.Printf("Steel thread coverage: %.1f%%\n", result.SteelThreadCoverage.CoveragePercent)
			}
			if result.NonSteelCoverage != nil {
				fmt.Printf("Non-steel thread coverage: %.1f%%\n", result.NonSteelCoverage.CoveragePercent)
			}
		} else {
			fmt.Println("Task coverage validation FAILED")
			for _, issue := range result.Issues {
				fmt.Printf("  - %s: %s\n", issue.Invariant, issue.Message)
			}
			return errors.ValidationFailed("Task coverage validation failed")
		}
		return nil
	},
}

var executeCoverageReportCmd = &cobra.Command{
	Use:   "execute-coverage-report <fsm-dir> <bundles-dir>",
	Short: "Generate execution coverage report",
	Long:  `Generates a report showing FSM transition coverage from execution bundles.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fsmDir := args[0]
		bundlesDir := args[1]

		indexPath := filepath.Join(fsmDir, "index.json")
		indexData, err := os.ReadFile(indexPath)
		if err != nil {
			return errors.IOReadFailed(indexPath, err)
		}

		var index fsmlib.IndexFile
		if err := json.Unmarshal(indexData, &index); err != nil {
			return errors.Internal("failed to parse index.json", err)
		}

		tasksCoverage, err := extractBundleCoverage(bundlesDir)
		if err != nil {
			return err
		}

		result, err := fsmlib.CheckTaskCoverage(&index, fsmDir, tasksCoverage, steelThreadThresh, nonSteelThreadThresh)
		if err != nil {
			return err
		}

		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}

		if coverageOutput != "" {
			if err := os.MkdirAll(filepath.Dir(coverageOutput), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(coverageOutput, output, 0644); err != nil {
				return err
			}
			fmt.Printf("Execution coverage report written to: %s\n", coverageOutput)
		} else {
			fmt.Println(string(output))
		}

		return nil
	},
}

func extractTaskCoverage(tasksDir string) ([]fsmlib.TaskTransitionCoverage, error) {
	var result []fsmlib.TaskTransitionCoverage

	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, errors.IOReadFailed(tasksDir, err)
	}

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

		taskID, _ := task["id"].(string)
		if taskID == "" {
			continue
		}

		var transitions []string
		if fsm, ok := task["state_machine"].(map[string]interface{}); ok {
			if trans, ok := fsm["transitions"].([]interface{}); ok {
				for _, t := range trans {
					if tStr, ok := t.(string); ok {
						transitions = append(transitions, tStr)
					}
				}
			}
		}

		if len(transitions) > 0 {
			result = append(result, fsmlib.TaskTransitionCoverage{
				TaskID:             taskID,
				TransitionsCovered: transitions,
			})
		}
	}

	return result, nil
}

func extractBundleCoverage(bundlesDir string) ([]fsmlib.TaskTransitionCoverage, error) {
	var result []fsmlib.TaskTransitionCoverage

	entries, err := os.ReadDir(bundlesDir)
	if err != nil {
		return nil, errors.IOReadFailed(bundlesDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, "-bundle.json") {
			continue
		}

		bundlePath := filepath.Join(bundlesDir, name)
		data, err := os.ReadFile(bundlePath)
		if err != nil {
			continue
		}

		var bundle map[string]interface{}
		if err := json.Unmarshal(data, &bundle); err != nil {
			continue
		}

		taskID, _ := bundle["task_id"].(string)
		if taskID == "" {
			continue
		}

		var transitions []string
		if fsm, ok := bundle["state_machine"].(map[string]interface{}); ok {
			if trans, ok := fsm["transitions"].([]interface{}); ok {
				for _, t := range trans {
					if tStr, ok := t.(string); ok {
						transitions = append(transitions, tStr)
					}
				}
			}
		}

		if len(transitions) > 0 {
			result = append(result, fsmlib.TaskTransitionCoverage{
				TaskID:             taskID,
				TransitionsCovered: transitions,
			})
		}
	}

	return result, nil
}

func init() {
	compileCmd.Flags().StringVar(&outputDir, "output-dir", "", "Output directory (default: planning-dir/fsm)")
	compileCmd.Flags().StringVar(&slug, "slug", "", "Spec slug (default: derived from filename)")

	fromCapMapCmd.Flags().StringVar(&outputDir, "output-dir", "", "Output directory (default: planning-dir/fsm)")
	fromCapMapCmd.Flags().StringVar(&slug, "slug", "", "Spec slug (default: derived from filename)")

	coverageReportCmd.Flags().StringVar(&coverageOutput, "output", "", "Output file for coverage report")
	coverageReportCmd.Flags().Float64Var(&steelThreadThresh, "steel-threshold", 1.0, "Steel thread coverage threshold (0-1)")
	coverageReportCmd.Flags().Float64Var(&nonSteelThreadThresh, "non-steel-threshold", 0.9, "Non-steel thread coverage threshold (0-1)")

	taskCoverageCmd.Flags().Float64Var(&steelThreadThresh, "steel-threshold", 1.0, "Steel thread coverage threshold (0-1)")
	taskCoverageCmd.Flags().Float64Var(&nonSteelThreadThresh, "non-steel-threshold", 0.9, "Non-steel thread coverage threshold (0-1)")

	executeCoverageReportCmd.Flags().StringVar(&coverageOutput, "output", "", "Output file for coverage report")
	executeCoverageReportCmd.Flags().Float64Var(&steelThreadThresh, "steel-threshold", 1.0, "Steel thread coverage threshold (0-1)")
	executeCoverageReportCmd.Flags().Float64Var(&nonSteelThreadThresh, "non-steel-threshold", 0.9, "Non-steel thread coverage threshold (0-1)")

	mermaidCmd.Flags().StringVar(&mermaidOutputDir, "output-dir", "", "Output directory (default: same as fsm-dir)")
	mermaidCmd.Flags().BoolVar(&mermaidNotes, "notes", true, "Generate notes markdown files")

	fsmCmd.AddCommand(compileCmd)
	fsmCmd.AddCommand(fromCapMapCmd)
	fsmCmd.AddCommand(validateCmd)
	fsmCmd.AddCommand(mermaidCmd)
	fsmCmd.AddCommand(coverageReportCmd)
	fsmCmd.AddCommand(taskCoverageCmd)
	fsmCmd.AddCommand(executeCoverageReportCmd)

	command.RootCmd.AddCommand(fsmCmd)
}
