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

var mermaidCmd = &cobra.Command{
	Use:   "mermaid <fsm-dir>",
	Short: "Generate Mermaid diagram from FSM",
	Long:  `Generates a Mermaid state diagram from FSM artifacts.`,
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

		for _, machine := range index.Machines {
			statesFile, hasStates := machine.Files["states"]
			transFile, hasTrans := machine.Files["transitions"]

			if !hasStates || !hasTrans {
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

			mermaid := generateMermaid(machine.Name, &statesFileData, &transFileData)
			fmt.Println(mermaid)
		}

		return nil
	},
}

func generateMermaid(machineName string, states *fsmlib.StatesFile, transitions *fsmlib.TransitionsFile) string {
	var sb strings.Builder

	sb.WriteString("stateDiagram-v2\n")
	sb.WriteString(fmt.Sprintf("    title %s\n\n", machineName))

	stateMap := make(map[string]fsmlib.State)
	for _, s := range states.States {
		stateMap[s.ID] = s
	}

	for _, s := range states.States {
		safeName := strings.ReplaceAll(s.Name, "\"", "'")
		switch s.Type {
		case "initial":
			sb.WriteString(fmt.Sprintf("    [*] --> %s\n", s.ID))
			sb.WriteString(fmt.Sprintf("    %s: %s\n", s.ID, safeName))
		case "success":
			sb.WriteString(fmt.Sprintf("    %s --> [*]\n", s.ID))
			sb.WriteString(fmt.Sprintf("    %s: %s\n", s.ID, safeName))
		case "failure":
			sb.WriteString(fmt.Sprintf("    %s: %s [FAILURE]\n", s.ID, safeName))
		default:
			sb.WriteString(fmt.Sprintf("    %s: %s\n", s.ID, safeName))
		}
	}

	sb.WriteString("\n")

	for _, t := range transitions.Transitions {
		safeTrigger := strings.ReplaceAll(t.Trigger, "\"", "'")
		if len(safeTrigger) > 30 {
			safeTrigger = safeTrigger[:27] + "..."
		}
		sb.WriteString(fmt.Sprintf("    %s --> %s: %s\n", t.FromState, t.ToState, safeTrigger))
	}

	return sb.String()
}

func init() {
	compileCmd.Flags().StringVar(&outputDir, "output-dir", "", "Output directory (default: planning-dir/fsm)")
	compileCmd.Flags().StringVar(&slug, "slug", "", "Spec slug (default: derived from filename)")

	fromCapMapCmd.Flags().StringVar(&outputDir, "output-dir", "", "Output directory (default: planning-dir/fsm)")
	fromCapMapCmd.Flags().StringVar(&slug, "slug", "", "Spec slug (default: derived from filename)")

	fsmCmd.AddCommand(compileCmd)
	fsmCmd.AddCommand(fromCapMapCmd)
	fsmCmd.AddCommand(validateCmd)
	fsmCmd.AddCommand(mermaidCmd)

	command.RootCmd.AddCommand(fsmCmd)
}
