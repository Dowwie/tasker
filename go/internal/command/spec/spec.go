package spec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

var (
	sessionTopic     string
	sessionTargetDir string
)

var sessionInitCmd = &cobra.Command{
	Use:   "init <topic>",
	Short: "Initialize a new spec session",
	Long:  `Creates a new spec-session.json file and discovery log.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		topic := args[0]

		baseDir := "."
		if sessionTargetDir != "" {
			baseDir = sessionTargetDir
		}

		session, err := speclib.InitSession(baseDir, topic, sessionTargetDir)
		if err != nil {
			return fmt.Errorf("failed to initialize session: %w", err)
		}

		if outputJSON {
			data, _ := json.MarshalIndent(session, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		fmt.Printf("Session initialized: %s\n", topic)
		fmt.Printf("Phase: %s\n", session.Phase)
		fmt.Printf("Target: %s\n", session.TargetDir)
		return nil
	},
}

var sessionAdvanceCmd = &cobra.Command{
	Use:   "advance",
	Short: "Advance to next phase",
	Long:  `Advances the spec session to the next phase.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		baseDir := "."
		if sessionTargetDir != "" {
			baseDir = sessionTargetDir
		}

		session, err := speclib.LoadSessionFromBaseDir(baseDir)
		if err != nil {
			return fmt.Errorf("failed to load session: %w", err)
		}
		if session == nil {
			return fmt.Errorf("no active session found")
		}

		oldPhase := session.Phase
		nextPhase, err := speclib.AdvancePhase(session)
		if err != nil {
			return fmt.Errorf("failed to advance phase: %w", err)
		}

		if err := speclib.SaveSessionToDir(baseDir, session); err != nil {
			return fmt.Errorf("failed to save session: %w", err)
		}

		fmt.Printf("Advanced from %s to %s\n", oldPhase, nextPhase)
		return nil
	},
}

var sessionGateCmd = &cobra.Command{
	Use:   "gate",
	Short: "Check if session is ready for export",
	Long:  `Validates that the session meets requirements for spec export.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		baseDir := "."
		if sessionTargetDir != "" {
			baseDir = sessionTargetDir
		}

		session, err := speclib.LoadSessionFromBaseDir(baseDir)
		if err != nil {
			return fmt.Errorf("failed to load session: %w", err)
		}
		if session == nil {
			return fmt.Errorf("no active session found")
		}

		result := speclib.CheckGate(session, baseDir)

		if outputJSON {
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		fmt.Println(result.Message)
		if !result.Passed {
			fmt.Println("\nIssues:")
			for _, issue := range result.Issues {
				fmt.Printf("  - %s\n", issue)
			}
			return fmt.Errorf("gate check failed")
		}

		return nil
	},
}

var (
	questionBlocking bool
)

var sessionAddQuestionCmd = &cobra.Command{
	Use:   "add-question <question>",
	Short: "Add an open question to the session",
	Long:  `Records an open question, optionally marking it as blocking.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		question := args[0]

		baseDir := "."
		if sessionTargetDir != "" {
			baseDir = sessionTargetDir
		}

		session, err := speclib.LoadSessionFromBaseDir(baseDir)
		if err != nil {
			return fmt.Errorf("failed to load session: %w", err)
		}
		if session == nil {
			return fmt.Errorf("no active session found")
		}

		speclib.AddOpenQuestion(session, question, questionBlocking)

		if err := speclib.SaveSessionToDir(baseDir, session); err != nil {
			return fmt.Errorf("failed to save session: %w", err)
		}

		blockingStr := ""
		if questionBlocking {
			blockingStr = " (blocking)"
		}
		fmt.Printf("Added question%s: %s\n", blockingStr, question)
		return nil
	},
}

var sessionResolveQuestionCmd = &cobra.Command{
	Use:   "resolve-question <question> <resolution>",
	Short: "Resolve an open question",
	Long:  `Marks an open question as resolved with the provided resolution.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		question := args[0]
		resolution := args[1]

		baseDir := "."
		if sessionTargetDir != "" {
			baseDir = sessionTargetDir
		}

		session, err := speclib.LoadSessionFromBaseDir(baseDir)
		if err != nil {
			return fmt.Errorf("failed to load session: %w", err)
		}
		if session == nil {
			return fmt.Errorf("no active session found")
		}

		found := speclib.ResolveOpenQuestion(session, question, resolution)
		if !found {
			return fmt.Errorf("question not found: %s", question)
		}

		if err := speclib.SaveSessionToDir(baseDir, session); err != nil {
			return fmt.Errorf("failed to save session: %w", err)
		}

		fmt.Printf("Resolved question: %s\n", question)
		return nil
	},
}

var (
	decisionADRID string
)

var sessionAddDecisionCmd = &cobra.Command{
	Use:   "add-decision <decision>",
	Short: "Record a decision",
	Long:  `Records a decision, optionally linking it to an ADR.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		decision := args[0]

		baseDir := "."
		if sessionTargetDir != "" {
			baseDir = sessionTargetDir
		}

		session, err := speclib.LoadSessionFromBaseDir(baseDir)
		if err != nil {
			return fmt.Errorf("failed to load session: %w", err)
		}
		if session == nil {
			return fmt.Errorf("no active session found")
		}

		speclib.AddDecision(session, decision, decisionADRID)

		if err := speclib.SaveSessionToDir(baseDir, session); err != nil {
			return fmt.Errorf("failed to save session: %w", err)
		}

		adrInfo := ""
		if decisionADRID != "" {
			adrInfo = fmt.Sprintf(" (ADR: %s)", decisionADRID)
		}
		fmt.Printf("Added decision%s: %s\n", adrInfo, decision)
		return nil
	},
}

var sessionNextADRCmd = &cobra.Command{
	Use:   "next-adr",
	Short: "Get the next ADR number",
	Long:  `Returns the next available ADR number based on existing ADRs.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := "."
		if sessionTargetDir != "" {
			targetDir = sessionTargetDir
		}

		num, err := speclib.GetNextADRNumber(targetDir)
		if err != nil {
			return fmt.Errorf("failed to get next ADR number: %w", err)
		}

		if outputJSON {
			fmt.Printf(`{"next_adr_number": %d}%s`, num, "\n")
			return nil
		}

		fmt.Printf("%04d\n", num)
		return nil
	},
}

var sessionStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show session status",
	Long:  `Displays the current spec session status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		baseDir := "."
		if sessionTargetDir != "" {
			baseDir = sessionTargetDir
		}

		status, err := speclib.GetSessionStatus(baseDir)
		if err != nil {
			return fmt.Errorf("failed to get session status: %w", err)
		}

		if outputJSON {
			data, _ := json.MarshalIndent(status, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		if status.Status == "no_session" {
			fmt.Println(status.Message)
			return nil
		}

		fmt.Println("Spec Session Status")
		fmt.Println("===================")
		fmt.Printf("Topic: %s\n", status.Topic)
		fmt.Printf("Phase: %s (%d/%d)\n", status.Phase, status.PhaseIndex+1, status.TotalPhases)
		fmt.Printf("Discovery rounds: %d\n", status.DiscoveryRounds)
		fmt.Printf("Open questions: %d blocking, %d non-blocking\n",
			status.OpenQuestions.Blocking, status.OpenQuestions.NonBlocking)
		fmt.Printf("Decisions: %d\n", status.Decisions)
		fmt.Printf("ADRs: %d\n", status.ADRs)
		fmt.Printf("Specs: %d\n", status.Specs)
		if status.StartedAt != "" {
			fmt.Printf("Started: %s\n", status.StartedAt)
		}

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
	Short: "Generate spec artifacts",
	Long:  `Commands for generating specs and ADRs from session data.`,
}

var (
	generateForce     bool
	generateTargetDir string
)

var generateSpecCmd = &cobra.Command{
	Use:   "spec",
	Short: "Generate spec markdown from session",
	Long:  `Generates a spec markdown file from the current session data.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		baseDir := "."
		if generateTargetDir != "" {
			baseDir = generateTargetDir
		}

		session, err := speclib.LoadSessionFromBaseDir(baseDir)
		if err != nil {
			return fmt.Errorf("failed to load session: %w", err)
		}
		if session == nil {
			return fmt.Errorf("no active session found")
		}

		opts := speclib.GenerateOptions{
			Force:     generateForce,
			TargetDir: generateTargetDir,
		}

		result, err := speclib.GenerateSpec(session, opts)
		if err != nil {
			return fmt.Errorf("failed to generate spec: %w", err)
		}

		if outputJSON {
			data, _ := json.MarshalIndent(map[string]interface{}{
				"title":        result.Title,
				"slug":         result.Slug,
				"output_path":  result.OutputPath,
				"generated_at": result.GeneratedAt,
			}, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		fmt.Printf("Generated spec: %s\n", result.OutputPath)
		fmt.Printf("Title: %s\n", result.Title)
		fmt.Printf("Slug: %s\n", result.Slug)
		return nil
	},
}

var (
	adrTitle        string
	adrContext      string
	adrDecision     string
	adrConsequences []string
	adrSupersedes   string
	adrRelated      []string
	adrAppliesTo    []string
)

var generateADRCmd = &cobra.Command{
	Use:   "adr",
	Short: "Generate an ADR markdown file",
	Long:  `Generates an Architecture Decision Record markdown file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if adrTitle == "" {
			return fmt.Errorf("--title is required")
		}
		if adrContext == "" {
			return fmt.Errorf("--context is required")
		}
		if adrDecision == "" {
			return fmt.Errorf("--decision is required")
		}

		targetDir := "."
		if generateTargetDir != "" {
			targetDir = generateTargetDir
		}

		nextNum, err := speclib.GetNextADRNumber(targetDir)
		if err != nil {
			return fmt.Errorf("failed to get next ADR number: %w", err)
		}

		var appliesTo []speclib.SpecReference
		for _, spec := range adrAppliesTo {
			appliesTo = append(appliesTo, speclib.SpecReference{Slug: spec})
		}

		input := speclib.ADRInput{
			Number:       nextNum,
			Title:        adrTitle,
			Context:      adrContext,
			Decision:     adrDecision,
			Consequences: adrConsequences,
			Supersedes:   adrSupersedes,
			RelatedADRs:  adrRelated,
			AppliesTo:    appliesTo,
		}

		opts := speclib.GenerateOptions{
			Force:     generateForce,
			TargetDir: targetDir,
		}

		result, err := speclib.GenerateADR(input, opts)
		if err != nil {
			return fmt.Errorf("failed to generate ADR: %w", err)
		}

		if outputJSON {
			data, _ := json.MarshalIndent(map[string]interface{}{
				"number":      result.Number,
				"title":       result.Title,
				"slug":        result.Slug,
				"output_path": result.OutputPath,
			}, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		fmt.Printf("Generated ADR: %s\n", result.OutputPath)
		fmt.Printf("Number: %04d\n", result.Number)
		fmt.Printf("Title: %s\n", result.Title)
		return nil
	},
}

var reportCmd = &cobra.Command{
	Use:   "report [spec-path]",
	Short: "Generate human-readable spec review report",
	Long:  `Generates a formatted report of spec weaknesses and resolutions.`,
	Args:  cobra.MaximumNArgs(1),
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
			planningDir := getPlanningDirFunc()
			specPath := filepath.Join(planningDir, "inputs", "spec.md")
			result, err = speclib.AnalyzeSpec(specPath)
			if err != nil {
				return fmt.Errorf("failed to analyze spec: %w", err)
			}
		}

		printDetailedReport(result)
		return nil
	},
}

func printDetailedReport(result *speclib.AnalysisResult) {
	review := result.Review

	fmt.Println("SPEC REVIEW REPORT")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()

	fmt.Println("SUMMARY")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("Status: %s\n", review.Status)
	fmt.Printf("Analyzed: %s\n", review.AnalyzedAt)
	fmt.Printf("Checksum: %s\n", review.SpecChecksum)
	fmt.Println()

	fmt.Printf("Total issues: %d\n", review.Summary.Total)
	fmt.Printf("  Critical: %d\n", review.Summary.BySeverity["critical"])
	fmt.Printf("  Warning: %d\n", review.Summary.BySeverity["warning"])
	fmt.Printf("  Info: %d\n", review.Summary.BySeverity["info"])
	fmt.Println()

	if review.Summary.Blocking {
		fmt.Println("⚠️  BLOCKING: Critical issues must be resolved before proceeding")
		fmt.Println()
	}

	if len(review.Summary.ByCategory) > 0 {
		fmt.Println("BY CATEGORY")
		fmt.Println(strings.Repeat("-", 40))
		for cat, count := range review.Summary.ByCategory {
			fmt.Printf("  %s: %d\n", cat, count)
		}
		fmt.Println()
	}

	if len(review.Weaknesses) > 0 {
		fmt.Println("DETAILED FINDINGS")
		fmt.Println(strings.Repeat("-", 40))
		for i, w := range review.Weaknesses {
			fmt.Printf("\n%d. [%s] %s\n", i+1, w.Severity, w.ID)
			fmt.Printf("   Category: %s\n", w.Category)
			fmt.Printf("   Location: %s\n", w.Location)
			fmt.Printf("   Description: %s\n", w.Description)
			if w.SpecQuote != "" {
				quote := w.SpecQuote
				if len(quote) > 80 {
					quote = quote[:77] + "..."
				}
				fmt.Printf("   Quote: \"%s\"\n", quote)
			}
			if w.SuggestedRes != "" {
				fmt.Printf("   Suggested: %s\n", w.SuggestedRes)
			}
		}
		fmt.Println()
	}

	fmt.Println("RECOMMENDATIONS")
	fmt.Println(strings.Repeat("-", 40))
	if review.Summary.Blocking {
		fmt.Println("1. Address all critical issues before proceeding")
		fmt.Println("2. Use 'tasker spec resolve <id> <resolution>' to mark issues resolved")
		fmt.Println("3. Re-run review after making changes")
	} else if review.Summary.Total > 0 {
		fmt.Println("1. Consider addressing warnings to improve spec quality")
		fmt.Println("2. Spec is ready for task decomposition")
	} else {
		fmt.Println("✓ Spec looks good! Ready for task decomposition.")
	}
}

var addResolutionCmd = &cobra.Command{
	Use:   "add-resolution <weakness-id> <resolution-type>",
	Short: "Add a resolution to spec-resolutions.json",
	Long: `Records a weakness resolution to spec-resolutions.json for downstream consumption.

Resolution types:
  mandatory       - MUST be implemented as specified
  optional        - Nice-to-have, not blocking
  defer           - Deferred to later phase
  clarified       - User provided clarification
  not_applicable  - Not a requirement`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		weaknessID := args[0]
		resTypeStr := args[1]

		validTypes := map[string]speclib.ResolutionType{
			"mandatory":      speclib.ResolutionMandatory,
			"optional":       speclib.ResolutionOptional,
			"defer":          speclib.ResolutionDefer,
			"clarified":      speclib.ResolutionClarified,
			"not_applicable": speclib.ResolutionNotApplicable,
		}

		resType, ok := validTypes[resTypeStr]
		if !ok {
			return fmt.Errorf("invalid resolution type: %s (valid: mandatory, optional, defer, clarified, not_applicable)", resTypeStr)
		}

		planningDir := getPlanningDirFunc()

		if err := speclib.AddResolution(planningDir, weaknessID, resType, resolutionResponse, resolutionReframe, resolutionNotes); err != nil {
			return fmt.Errorf("failed to add resolution: %w", err)
		}

		fmt.Printf("Added resolution: %s -> %s\n", weaknessID, resTypeStr)

		ready, remaining, _ := speclib.IsReadyToProceed(planningDir)
		if ready {
			fmt.Println("Status: READY - all critical weaknesses resolved")
		} else {
			fmt.Printf("Status: BLOCKED - %d unresolved critical weaknesses\n", remaining)
		}

		return nil
	},
}

var unresolvedCmd = &cobra.Command{
	Use:   "unresolved",
	Short: "List unresolved critical weaknesses",
	Long:  `Shows all critical weaknesses that have not been resolved yet.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()

		unresolved, err := speclib.GetUnresolvedWeaknesses(planningDir)
		if err != nil {
			return fmt.Errorf("failed to get unresolved weaknesses: %w", err)
		}

		if outputJSON {
			data, err := json.MarshalIndent(unresolved, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal: %w", err)
			}
			fmt.Println(string(data))
			return nil
		}

		if len(unresolved) == 0 {
			fmt.Println("No unresolved critical weaknesses!")
			return nil
		}

		fmt.Printf("Unresolved Critical Weaknesses: %d\n\n", len(unresolved))
		for _, w := range unresolved {
			fmt.Printf("[%s] %s\n", w.ID, w.Description)
			if w.SpecQuote != "" {
				fmt.Printf("    Quote: %s\n", w.SpecQuote)
			}
			if w.SuggestedRes != "" {
				fmt.Printf("    Suggested: %s\n", w.SuggestedRes)
			}
			fmt.Println()
		}

		return nil
	},
}

var checklistCmd = &cobra.Command{
	Use:   "checklist [spec-path]",
	Short: "Show spec completeness checklist",
	Long:  `Verifies spec against completeness checklist (C1-C11 categories).`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var content []byte
		var err error

		if len(args) > 0 {
			content, err = os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("failed to read spec: %w", err)
			}
		} else {
			planningDir := getPlanningDirFunc()
			specPath := filepath.Join(planningDir, "inputs", "spec.md")
			content, err = os.ReadFile(specPath)
			if err != nil {
				return fmt.Errorf("failed to read spec from %s: %w", specPath, err)
			}
		}

		result := speclib.VerifyChecklist(string(content))

		if outputJSON {
			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal: %w", err)
			}
			fmt.Println(string(data))
			return nil
		}

		fmt.Println("SPEC COMPLETENESS CHECKLIST")
		fmt.Println("===========================")
		fmt.Println()

		statusIcons := map[string]string{
			"complete": "[✓]",
			"partial":  "[◐]",
			"missing":  "[✗]",
			"na":       "[-]",
		}

		currentCategory := ""
		for _, item := range result.Items {
			if item.Category != currentCategory {
				currentCategory = item.Category
				fmt.Printf("\n%s\n", strings.ToUpper(strings.ReplaceAll(currentCategory, "_", " ")))
				fmt.Println(strings.Repeat("-", 40))
			}

			icon := statusIcons[item.Status]
			severityMark := ""
			if item.SeverityIfMissing == "critical" {
				severityMark = "*"
			}
			fmt.Printf("  %s %s%s: %s\n", icon, item.ID, severityMark, item.Question)
			if item.Evidence != "" {
				fmt.Printf("      -> %s\n", item.Evidence)
			}
		}

		fmt.Println()
		fmt.Printf("Summary: %d complete, %d partial, %d missing (%d critical)\n",
			result.Complete, result.Partial, result.Missing, result.CriticalMissing)
		fmt.Println()
		fmt.Println("Legend: ✓=complete  ◐=partial  ✗=missing  -=N/A  *=critical if missing")

		return nil
	},
}

var (
	outputJSON        bool
	saveReview        bool
	resolutionResponse string
	resolutionReframe  string
	resolutionNotes    string
)

func init() {
	reviewCmd.Flags().BoolVar(&outputJSON, "json", false, "Output in JSON format")
	reviewCmd.Flags().BoolVar(&saveReview, "save", false, "Save review to artifacts directory")

	statusCmd.Flags().BoolVar(&outputJSON, "json", false, "Output in JSON format")

	sessionShowCmd.Flags().BoolVar(&outputJSON, "json", false, "Output in JSON format")

	addResolutionCmd.Flags().StringVar(&resolutionResponse, "response", "", "User's response text")
	addResolutionCmd.Flags().StringVar(&resolutionReframe, "reframe", "", "Behavioral reframe for non-behavioral requirements")
	addResolutionCmd.Flags().StringVar(&resolutionNotes, "notes", "", "Additional notes or context")

	unresolvedCmd.Flags().BoolVar(&outputJSON, "json", false, "Output in JSON format")

	checklistCmd.Flags().BoolVar(&outputJSON, "json", false, "Output in JSON format")

	sessionInitCmd.Flags().StringVar(&sessionTargetDir, "target-dir", "", "Target directory for the session")
	sessionInitCmd.Flags().BoolVar(&outputJSON, "json", false, "Output in JSON format")

	sessionAdvanceCmd.Flags().StringVar(&sessionTargetDir, "target-dir", "", "Target directory for the session")

	sessionGateCmd.Flags().StringVar(&sessionTargetDir, "target-dir", "", "Target directory for the session")
	sessionGateCmd.Flags().BoolVar(&outputJSON, "json", false, "Output in JSON format")

	sessionAddQuestionCmd.Flags().StringVar(&sessionTargetDir, "target-dir", "", "Target directory for the session")
	sessionAddQuestionCmd.Flags().BoolVar(&questionBlocking, "blocking", false, "Mark question as blocking")

	sessionResolveQuestionCmd.Flags().StringVar(&sessionTargetDir, "target-dir", "", "Target directory for the session")

	sessionAddDecisionCmd.Flags().StringVar(&sessionTargetDir, "target-dir", "", "Target directory for the session")
	sessionAddDecisionCmd.Flags().StringVar(&decisionADRID, "adr", "", "Link to ADR ID")

	sessionNextADRCmd.Flags().StringVar(&sessionTargetDir, "target-dir", "", "Target directory")
	sessionNextADRCmd.Flags().BoolVar(&outputJSON, "json", false, "Output in JSON format")

	sessionStatusCmd.Flags().StringVar(&sessionTargetDir, "target-dir", "", "Target directory for the session")
	sessionStatusCmd.Flags().BoolVar(&outputJSON, "json", false, "Output in JSON format")

	generateSpecCmd.Flags().BoolVar(&generateForce, "force", false, "Overwrite existing file")
	generateSpecCmd.Flags().StringVar(&generateTargetDir, "target-dir", "", "Target directory")
	generateSpecCmd.Flags().BoolVar(&outputJSON, "json", false, "Output in JSON format")

	generateADRCmd.Flags().StringVar(&adrTitle, "title", "", "ADR title (required)")
	generateADRCmd.Flags().StringVar(&adrContext, "context", "", "ADR context (required)")
	generateADRCmd.Flags().StringVar(&adrDecision, "decision", "", "ADR decision (required)")
	generateADRCmd.Flags().StringSliceVar(&adrConsequences, "consequence", []string{}, "ADR consequences (can be repeated)")
	generateADRCmd.Flags().StringVar(&adrSupersedes, "supersedes", "", "ADR that this supersedes")
	generateADRCmd.Flags().StringSliceVar(&adrRelated, "related", []string{}, "Related ADR IDs (can be repeated)")
	generateADRCmd.Flags().StringSliceVar(&adrAppliesTo, "applies-to", []string{}, "Spec slugs this applies to (can be repeated)")
	generateADRCmd.Flags().BoolVar(&generateForce, "force", false, "Overwrite existing file")
	generateADRCmd.Flags().StringVar(&generateTargetDir, "target-dir", "", "Target directory")
	generateADRCmd.Flags().BoolVar(&outputJSON, "json", false, "Output in JSON format")

	sessionCmd.AddCommand(sessionShowCmd)
	sessionCmd.AddCommand(sessionInitCmd)
	sessionCmd.AddCommand(sessionAdvanceCmd)
	sessionCmd.AddCommand(sessionGateCmd)
	sessionCmd.AddCommand(sessionAddQuestionCmd)
	sessionCmd.AddCommand(sessionResolveQuestionCmd)
	sessionCmd.AddCommand(sessionAddDecisionCmd)
	sessionCmd.AddCommand(sessionNextADRCmd)
	sessionCmd.AddCommand(sessionStatusCmd)

	generateCmd.AddCommand(generateSpecCmd)
	generateCmd.AddCommand(generateADRCmd)

	specCmd.AddCommand(reviewCmd)
	specCmd.AddCommand(statusCmd)
	specCmd.AddCommand(sessionCmd)
	specCmd.AddCommand(resolveCmd)
	specCmd.AddCommand(generateCmd)
	specCmd.AddCommand(addResolutionCmd)
	specCmd.AddCommand(unresolvedCmd)
	specCmd.AddCommand(checklistCmd)
	specCmd.AddCommand(reportCmd)

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
