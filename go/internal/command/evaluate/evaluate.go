package evaluate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dgordon/tasker/internal/command"
	"github.com/dgordon/tasker/internal/util"
	"github.com/spf13/cobra"
)

var (
	outputFormat string
	metricsOnly  bool
	outputFile   string
)

var evaluateCmd = &cobra.Command{
	Use:   "evaluate",
	Short: "Generate execution evaluation report",
	Long: `Generates a comprehensive evaluation report for the completed execution.

The report includes:
- Planning quality (verdict, issues at planning time)
- Execution summary (completed, failed, success rate)
- First-attempt success rate and average attempts
- Verification breakdown (functional criteria, code quality)
- Cost analysis (tokens, cost per task)
- Failure analysis (which tasks failed and why)
- Improvement patterns (common issues across tasks)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := os.Getenv("TASKER_DIR")
		if planningDir == "" {
			planningDir = ".tasker"
		}

		result, err := util.RunEvaluation(planningDir)
		if err != nil {
			return fmt.Errorf("evaluation failed: %w", err)
		}

		var output string
		if outputFormat == "json" {
			var data []byte
			if metricsOnly {
				data, err = json.MarshalIndent(result.Metrics, "", "  ")
			} else {
				data, err = json.MarshalIndent(result, "", "  ")
			}
			if err != nil {
				return fmt.Errorf("failed to marshal JSON: %w", err)
			}
			output = string(data)
		} else {
			if metricsOnly {
				output = formatMetricsOnly(result.Metrics)
			} else {
				output = util.FormatEvaluationReport(result)
			}
		}

		if outputFile != "" {
			dir := filepath.Dir(outputFile)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}
			if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
				return fmt.Errorf("failed to write output file: %w", err)
			}
			fmt.Printf("Report written to %s\n", outputFile)
		} else {
			fmt.Print(output)
		}

		return nil
	},
}

func formatMetricsOnly(m *util.EvaluationMetrics) string {
	return fmt.Sprintf(`Execution Metrics
-----------------
Tasks:      %d completed, %d failed
Success:    %.0f%% (%.0f%% first-attempt)
Attempts:   %.2f avg
Tokens:     %d total (%d per task)
Cost:       $%.2f total ($%.4f per task)
Criteria:   %d pass, %d partial, %d fail
`,
		m.CompletedCount, m.FailedCount,
		m.TaskSuccessRate*100, m.FirstAttemptSuccessRate*100,
		m.AvgAttempts,
		m.TotalTokens, m.TokensPerTask,
		m.TotalCostUSD, m.CostPerTask,
		m.CriteriaPass, m.CriteriaPartial, m.CriteriaFail,
	)
}

func init() {
	evaluateCmd.Flags().StringVar(&outputFormat, "format", "text", "Output format: text or json")
	evaluateCmd.Flags().BoolVar(&metricsOnly, "metrics-only", false, "Output only metrics without full report")
	evaluateCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write report to file instead of stdout")
	command.RootCmd.AddCommand(evaluateCmd)
}
