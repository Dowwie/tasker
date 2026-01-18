package bundle

import (
	"fmt"

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

func init() {
	bundleCmd.AddCommand(generateCmd)
	bundleCmd.AddCommand(generateReadyCmd)
	bundleCmd.AddCommand(validateCmd)
	bundleCmd.AddCommand(validateIntegrityCmd)
	bundleCmd.AddCommand(listCmd)
	bundleCmd.AddCommand(cleanCmd)

	command.RootCmd.AddCommand(bundleCmd)
}
