package spec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dgordon/tasker/internal/command"
	speclib "github.com/dgordon/tasker/internal/spec"
	"github.com/spf13/cobra"
)

// getPlanningDirFunc is a function variable for dependency injection in tests
var getPlanningDirFunc = command.GetPlanningDir

var specCmd = &cobra.Command{
	Use:   "spec",
	Short: "Spec management commands",
	Long:  `Commands for spec analysis, review, and generation.`,
}

var reviewCmd = &cobra.Command{
	Use:   "review [spec-path]",
	Short: "Analyze spec for weaknesses",
	Long: `Runs weakness detection on a specification file and generates a spec-review.json.

If no spec-path is provided, reads from stdin.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var result *speclib.AnalysisResult
		var err error

		if len(args) > 0 {
			specPath := args[0]
			result, err = speclib.AnalyzeSpec(specPath)
			if err != nil {
				return fmt.Errorf("failed to analyze spec: %w", err)
			}
		} else {
			data, err := readStdin()
			if err != nil {
				return fmt.Errorf("failed to read from stdin: %w", err)
			}
			result, err = speclib.AnalyzeSpecContent(data, "stdin")
			if err != nil {
				return fmt.Errorf("failed to analyze spec: %w", err)
			}
		}

		if outputJSON {
			return outputReviewJSON(result.Review)
		}

		printReviewSummary(result)

		if saveReview {
			planningDir := getPlanningDirFunc()
			if err := speclib.SaveReview(planningDir, result.Review); err != nil {
				return fmt.Errorf("failed to save review: %w", err)
			}
			fmt.Printf("\nReview saved to %s/artifacts/spec-review.json\n", planningDir)
		}

		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current review status",
	Long:  `Displays the current spec review status and findings summary.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()

		status, err := speclib.GetReviewStatus(planningDir)
		if err != nil {
			return fmt.Errorf("failed to get review status: %w", err)
		}

		if outputJSON {
			return outputStatusJSON(status)
		}

		printStatusSummary(status)
		return nil
	},
}

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Spec session management",
	Long:  `Commands for managing spec review sessions.`,
}

var sessionShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current session details",
	Long:  `Displays details of the current spec review session.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()

		review, err := speclib.LoadReview(planningDir)
		if err != nil {
			fmt.Println("No active spec review session")
			return nil
		}

		if outputJSON {
			return outputReviewJSON(review)
		}

		printSessionDetails(review)
		return nil
	},
}

var resolveCmd = &cobra.Command{
	Use:   "resolve <weakness-id> <resolution>",
	Short: "Resolve a weakness",
	Long:  `Marks a weakness as resolved with the provided resolution.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		weaknessID := args[0]
		resolution := args[1]

		planningDir := getPlanningDirFunc()

		review, err := speclib.LoadReview(planningDir)
		if err != nil {
			return fmt.Errorf("failed to load review: %w", err)
		}

		if err := speclib.ResolveWeakness(review, weaknessID, resolution); err != nil {
			return fmt.Errorf("failed to resolve weakness: %w", err)
		}

		if err := speclib.SaveReview(planningDir, review); err != nil {
			return fmt.Errorf("failed to save review: %w", err)
		}

		fmt.Printf("Resolved weakness %s\n", weaknessID)
		fmt.Printf("Review status: %s\n", review.Status)
		return nil
	},
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate spec from templates",
	Long:  `Generates spec artifacts from templates (placeholder for future implementation).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Spec generation not yet implemented")
		return nil
	},
}

var (
	outputJSON bool
	saveReview bool
)

func init() {
	reviewCmd.Flags().BoolVar(&outputJSON, "json", false, "Output in JSON format")
	reviewCmd.Flags().BoolVar(&saveReview, "save", false, "Save review to artifacts directory")

	statusCmd.Flags().BoolVar(&outputJSON, "json", false, "Output in JSON format")

	sessionShowCmd.Flags().BoolVar(&outputJSON, "json", false, "Output in JSON format")

	sessionCmd.AddCommand(sessionShowCmd)

	specCmd.AddCommand(reviewCmd)
	specCmd.AddCommand(statusCmd)
	specCmd.AddCommand(sessionCmd)
	specCmd.AddCommand(resolveCmd)
	specCmd.AddCommand(generateCmd)

	command.RootCmd.AddCommand(specCmd)
}

func readStdin() ([]byte, error) {
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return nil, fmt.Errorf("no input provided via stdin")
	}

	return os.ReadFile(os.Stdin.Name())
}

func outputReviewJSON(review *speclib.SpecReview) error {
	data, err := json.MarshalIndent(review, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal review: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func outputStatusJSON(status *speclib.ReviewStatusResult) error {
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func printReviewSummary(result *speclib.AnalysisResult) {
	review := result.Review

	fmt.Println("Spec Review Analysis")
	fmt.Println("====================")
	fmt.Printf("Checksum: %s\n", review.SpecChecksum)
	fmt.Printf("Analyzed: %s\n", review.AnalyzedAt)
	fmt.Printf("Status: %s\n", review.Status)
	fmt.Println()

	fmt.Println("Summary")
	fmt.Println("-------")
	fmt.Printf("Total weaknesses: %d\n", review.Summary.Total)
	fmt.Printf("  Critical: %d\n", review.Summary.BySeverity["critical"])
	fmt.Printf("  Warning: %d\n", review.Summary.BySeverity["warning"])
	fmt.Printf("  Info: %d\n", review.Summary.BySeverity["info"])

	if review.Summary.Blocking {
		fmt.Println("\nBLOCKING: Critical issues must be resolved before proceeding")
	}

	if len(review.Weaknesses) > 0 {
		fmt.Println("\nWeaknesses Found")
		fmt.Println("----------------")
		for _, w := range review.Weaknesses {
			fmt.Printf("[%s] %s (%s) - %s\n", w.ID, w.Severity, w.Category, w.Location)
			fmt.Printf("  %s\n", w.Description)
			if w.SpecQuote != "" {
				fmt.Printf("  Quote: \"%s\"\n", w.SpecQuote)
			}
			fmt.Println()
		}
	}
}

func printStatusSummary(status *speclib.ReviewStatusResult) {
	fmt.Println("Spec Review Status")
	fmt.Println("==================")
	fmt.Printf("Status: %s\n", status.Status)

	if status.AnalyzedAt != "" {
		fmt.Printf("Last analyzed: %s\n", status.AnalyzedAt)
	}

	if status.SpecChecksum != "" {
		fmt.Printf("Spec checksum: %s\n", status.SpecChecksum)
	}

	fmt.Println()
	fmt.Printf("Total issues: %d\n", status.TotalIssues)
	fmt.Printf("  Critical: %d\n", status.Critical)
	fmt.Printf("  Warnings: %d\n", status.Warnings)
	fmt.Printf("  Info: %d\n", status.Info)

	if status.Blocking {
		fmt.Println("\nBLOCKING: Critical issues must be resolved")
	} else if status.TotalIssues == 0 {
		fmt.Println("\nNo issues found - ready to proceed")
	}
}

func printSessionDetails(review *speclib.SpecReview) {
	fmt.Println("Spec Review Session")
	fmt.Println("===================")
	fmt.Printf("Version: %s\n", review.Version)
	fmt.Printf("Status: %s\n", review.Status)
	fmt.Printf("Checksum: %s\n", review.SpecChecksum)
	fmt.Printf("Analyzed: %s\n", review.AnalyzedAt)

	if review.Notes != "" {
		fmt.Printf("Notes: %s\n", review.Notes)
	}

	fmt.Println()
	fmt.Printf("Weaknesses: %d total\n", review.Summary.Total)

	resolved := 0
	unresolved := 0
	for _, w := range review.Weaknesses {
		if w.SuggestedRes != "" {
			resolved++
		} else {
			unresolved++
		}
	}

	fmt.Printf("  Resolved: %d\n", resolved)
	fmt.Printf("  Unresolved: %d\n", unresolved)

	if len(review.Summary.ByCategory) > 0 {
		fmt.Println("\nBy Category:")
		for cat, count := range review.Summary.ByCategory {
			fmt.Printf("  %s: %d\n", cat, count)
		}
	}

	if unresolved > 0 {
		fmt.Println("\nUnresolved Weaknesses:")
		for _, w := range review.Weaknesses {
			if w.SuggestedRes == "" {
				fmt.Printf("  [%s] %s - %s\n", w.ID, w.Severity, w.Description)
				fmt.Printf("    Location: %s\n", w.Location)
			}
		}
	}
}

func FindSpecFile(planningDir string) (string, error) {
	candidates := []string{
		filepath.Join(planningDir, "spec.md"),
		filepath.Join(planningDir, "SPEC.md"),
		filepath.Join(planningDir, "specification.md"),
		filepath.Join(filepath.Dir(planningDir), "spec.md"),
		filepath.Join(filepath.Dir(planningDir), "SPEC.md"),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no spec file found in expected locations")
}
