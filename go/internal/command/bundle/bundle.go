package bundle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dgordon/tasker/internal/command"
	bundlelib "github.com/dgordon/tasker/internal/bundle"
	"github.com/spf13/cobra"
)

// getPlanningDirFunc is a function variable for dependency injection in tests
var getPlanningDirFunc = command.GetPlanningDir

var bundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Bundle management commands",
	Long:  `Commands for generating, validating, and managing task execution bundles.`,
}

var generateCmd = &cobra.Command{
	Use:   "generate <task-id>",
	Short: "Generate bundle for a task",
	Long:  `Generates a self-contained execution bundle for the specified task.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]
		planningDir := getPlanningDirFunc()
		gen := bundlelib.NewGenerator(planningDir)

		bundle, err := gen.GenerateBundle(taskID)
		if err != nil {
			return fmt.Errorf("failed to generate bundle: %w", err)
		}

		fmt.Printf("Generated bundle for %s: %s\n", bundle.TaskID, bundle.Name)
		fmt.Printf("  Version: %s\n", bundle.Version)
		fmt.Printf("  Phase: %d\n", bundle.Phase)
		fmt.Printf("  Behaviors: %d\n", len(bundle.Behaviors))
		fmt.Printf("  Files: %d\n", len(bundle.Files))
		fmt.Printf("  Target: %s\n", bundle.TargetDir)
		return nil
	},
}

var generateReadyCmd = &cobra.Command{
	Use:   "generate-ready",
	Short: "Generate bundles for all ready tasks",
	Long:  `Generates execution bundles for all tasks that are ready to execute.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()
		gen := bundlelib.NewGenerator(planningDir)

		bundles, errs := gen.GenerateReadyBundles()

		for _, bundle := range bundles {
			fmt.Printf("Generated: %s (%s)\n", bundle.TaskID, bundle.Name)
		}

		if len(errs) > 0 {
			fmt.Printf("\nErrors (%d):\n", len(errs))
			for _, err := range errs {
				fmt.Printf("  - %v\n", err)
			}
			return fmt.Errorf("failed to generate %d bundles", len(errs))
		}

		if len(bundles) == 0 {
			fmt.Println("No ready tasks found")
		} else {
			fmt.Printf("\nGenerated %d bundles\n", len(bundles))
		}

		return nil
	},
}

var validateCmd = &cobra.Command{
	Use:   "validate <task-id>",
	Short: "Validate bundle against schema",
	Long:  `Validates an existing bundle file against the JSON schema.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]
		planningDir := getPlanningDirFunc()
		gen := bundlelib.NewGenerator(planningDir)

		result, err := gen.ValidateBundle(taskID)
		if err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}

		if result.Valid {
			fmt.Printf("Bundle %s is valid\n", taskID)
		} else {
			fmt.Printf("Bundle %s validation failed:\n", taskID)
			for _, e := range result.Errors {
				fmt.Printf("  - %s: %s\n", e.Path, e.Message)
			}
			return fmt.Errorf("bundle validation failed")
		}

		return nil
	},
}

var validateIntegrityCmd = &cobra.Command{
	Use:   "validate-integrity <task-id>",
	Short: "Validate bundle integrity",
	Long: `Validates that all dependency files referenced in the bundle exist
and that artifacts haven't changed since bundle generation.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]
		planningDir := getPlanningDirFunc()
		gen := bundlelib.NewGenerator(planningDir)

		result, err := gen.ValidateIntegrity(taskID)
		if err != nil {
			return fmt.Errorf("integrity check failed: %w", err)
		}

		if result.Valid {
			fmt.Printf("Bundle %s integrity validated\n", taskID)
			return nil
		}

		if len(result.MissingFiles) > 0 {
			fmt.Printf("Missing dependency files:\n")
			for _, f := range result.MissingFiles {
				fmt.Printf("  - %s\n", f)
			}
		}

		if len(result.ChangedFiles) > 0 {
			fmt.Printf("Changed files since bundle generation:\n")
			for _, f := range result.ChangedFiles {
				fmt.Printf("  - %s\n", f)
			}
			fmt.Printf("\nConsider regenerating: tasker bundle generate %s\n", taskID)
		}

		return fmt.Errorf("bundle integrity check failed")
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List existing bundles",
	Long:  `Lists all existing bundle files with their status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()
		gen := bundlelib.NewGenerator(planningDir)

		bundles, err := gen.ListBundles()
		if err != nil {
			return fmt.Errorf("failed to list bundles: %w", err)
		}

		if len(bundles) == 0 {
			fmt.Println("No bundles found")
			return nil
		}

		fmt.Printf("Bundles (%d):\n", len(bundles))
		for _, b := range bundles {
			fmt.Printf("  %s: %s (phase %d)\n", b.TaskID, b.Name, b.Phase)
			fmt.Printf("    Created: %s\n", b.CreatedAt)
		}

		return nil
	},
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove all bundles",
	Long:  `Removes all bundle files from the bundles directory.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()
		gen := bundlelib.NewGenerator(planningDir)

		count, err := gen.CleanBundles()
		if err != nil {
			return fmt.Errorf("failed to clean bundles: %w", err)
		}

		fmt.Printf("Removed %d bundles\n", count)
		return nil
	},
}

type ResultFiles struct {
	Created  []string `json:"created,omitempty"`
	Modified []string `json:"modified,omitempty"`
}

type ResultGit struct {
	Committed     bool   `json:"committed"`
	CommitSHA     string `json:"commit_sha,omitempty"`
	CommitMessage string `json:"commit_message,omitempty"`
	CommittedBy   string `json:"committed_by,omitempty"`
}

type Result struct {
	Version     string      `json:"version"`
	TaskID      string      `json:"task_id"`
	Name        string      `json:"name"`
	Status      string      `json:"status"`
	StartedAt   string      `json:"started_at,omitempty"`
	CompletedAt string      `json:"completed_at,omitempty"`
	Files       ResultFiles `json:"files,omitempty"`
	Git         *ResultGit  `json:"git,omitempty"`
}

func loadResult(planningDir, taskID string) (*Result, string, error) {
	resultPath := filepath.Join(planningDir, "bundles", fmt.Sprintf("%s-result.json", taskID))
	data, err := os.ReadFile(resultPath)
	if err != nil {
		return nil, resultPath, fmt.Errorf("failed to read result file: %w", err)
	}

	var result Result
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, resultPath, fmt.Errorf("failed to parse result JSON: %w", err)
	}

	return &result, resultPath, nil
}

var resultInfoCmd = &cobra.Command{
	Use:   "result-info <task-id>",
	Short: "Get name and status from result file",
	Long:  `Outputs the task name and status from the result file, tab-separated.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]
		planningDir := getPlanningDirFunc()

		result, _, err := loadResult(planningDir, taskID)
		if err != nil {
			return err
		}

		name := result.Name
		if name == "" {
			name = taskID
		}
		fmt.Printf("%s\t%s\n", name, result.Status)
		return nil
	},
}

var resultFilesCmd = &cobra.Command{
	Use:   "result-files <task-id>",
	Short: "Get files from result file",
	Long:  `Outputs all files (created and modified) from the result file, one per line.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]
		planningDir := getPlanningDirFunc()

		result, _, err := loadResult(planningDir, taskID)
		if err != nil {
			return err
		}

		for _, f := range result.Files.Created {
			if f != "" {
				fmt.Println(f)
			}
		}
		for _, f := range result.Files.Modified {
			if f != "" {
				fmt.Println(f)
			}
		}
		return nil
	},
}

var (
	gitSHA     string
	gitMessage string
)

var updateGitCmd = &cobra.Command{
	Use:   "update-git <task-id>",
	Short: "Update result file with git commit info",
	Long:  `Adds git commit information to the result file.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]
		planningDir := getPlanningDirFunc()

		result, resultPath, err := loadResult(planningDir, taskID)
		if err != nil {
			return err
		}

		result.Git = &ResultGit{
			Committed:     true,
			CommitSHA:     gitSHA,
			CommitMessage: gitMessage,
			CommittedBy:   "hook",
		}

		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to serialize result: %w", err)
		}

		if err := os.WriteFile(resultPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write result file: %w", err)
		}

		return nil
	},
}

func init() {
	updateGitCmd.Flags().StringVar(&gitSHA, "sha", "", "Git commit SHA")
	updateGitCmd.Flags().StringVar(&gitMessage, "msg", "", "Git commit message")
	updateGitCmd.MarkFlagRequired("sha")
	updateGitCmd.MarkFlagRequired("msg")

	bundleCmd.AddCommand(generateCmd)
	bundleCmd.AddCommand(generateReadyCmd)
	bundleCmd.AddCommand(validateCmd)
	bundleCmd.AddCommand(validateIntegrityCmd)
	bundleCmd.AddCommand(listCmd)
	bundleCmd.AddCommand(cleanCmd)
	bundleCmd.AddCommand(resultInfoCmd)
	bundleCmd.AddCommand(resultFilesCmd)
	bundleCmd.AddCommand(updateGitCmd)

	command.RootCmd.AddCommand(bundleCmd)
}
