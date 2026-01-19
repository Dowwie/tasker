package state

import (
	"fmt"
	"strings"

	"github.com/dgordon/tasker/internal/command"
	statelib "github.com/dgordon/tasker/internal/state"
	"github.com/spf13/cobra"
)

// getPlanningDirFunc is a function variable for dependency injection in tests
var getPlanningDirFunc = command.GetPlanningDir

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "State management commands",
	Long:  `Commands for managing decomposition state including initialization and task lifecycle.`,
}

var initCmd = &cobra.Command{
	Use:   "init <target-dir>",
	Short: "Initialize a new decomposition",
	Long:  `Creates a new state.json file in the planning directory with default structure.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := args[0]
		planningDir := getPlanningDirFunc()

		state, err := statelib.InitDecomposition(planningDir, targetDir)
		if err != nil {
			return fmt.Errorf("failed to initialize decomposition: %w", err)
		}

		fmt.Printf("Initialized decomposition in %s\n", planningDir)
		fmt.Printf("Target directory: %s\n", state.TargetDir)
		fmt.Printf("Phase: %s\n", state.Phase.Current)
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current state status",
	Long:  `Displays the current decomposition status including phase and task counts.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()
		sm := statelib.NewStateManager(planningDir)

		state, err := sm.Load()
		if err != nil {
			return fmt.Errorf("failed to load state: %w", err)
		}

		fmt.Printf("Phase: %s\n", state.Phase.Current)
		fmt.Printf("Target: %s\n", state.TargetDir)
		fmt.Printf("Tasks: %d total\n", len(state.Tasks))

		statusCounts := make(map[string]int)
		for _, task := range state.Tasks {
			statusCounts[task.Status]++
		}

		for status, count := range statusCounts {
			fmt.Printf("  %s: %d\n", status, count)
		}

		if len(state.Execution.ActiveTasks) > 0 {
			fmt.Printf("Active: %s\n", strings.Join(state.Execution.ActiveTasks, ", "))
		}

		return nil
	},
}

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Task lifecycle commands",
	Long:  `Commands for managing individual task lifecycle: start, complete, fail, retry, skip.`,
}

var taskStartCmd = &cobra.Command{
	Use:   "start <task-id>",
	Short: "Start a task",
	Long:  `Marks a pending task as running and records the start time.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]
		planningDir := getPlanningDirFunc()
		sm := statelib.NewStateManager(planningDir)

		if err := statelib.StartTask(sm, taskID); err != nil {
			return fmt.Errorf("failed to start task: %w", err)
		}

		fmt.Printf("Started task %s\n", taskID)
		return nil
	},
}

var (
	filesCreated  []string
	filesModified []string
)

var taskCompleteCmd = &cobra.Command{
	Use:   "complete <task-id>",
	Short: "Complete a task",
	Long:  `Marks a running task as complete and records files created/modified.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]
		planningDir := getPlanningDirFunc()
		sm := statelib.NewStateManager(planningDir)

		if err := statelib.CompleteTask(sm, taskID, filesCreated, filesModified); err != nil {
			return fmt.Errorf("failed to complete task: %w", err)
		}

		fmt.Printf("Completed task %s\n", taskID)
		return nil
	},
}

var (
	failCategory  string
	failRetryable bool
)

var taskFailCmd = &cobra.Command{
	Use:   "fail <task-id> <error-message>",
	Short: "Fail a task",
	Long:  `Marks a running task as failed with error details and classification.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]
		errorMsg := args[1]
		planningDir := getPlanningDirFunc()
		sm := statelib.NewStateManager(planningDir)

		if err := statelib.FailTask(sm, taskID, errorMsg, failCategory, failRetryable); err != nil {
			return fmt.Errorf("failed to fail task: %w", err)
		}

		fmt.Printf("Failed task %s\n", taskID)
		return nil
	},
}

var taskRetryCmd = &cobra.Command{
	Use:   "retry <task-id>",
	Short: "Retry a failed task",
	Long:  `Resets a failed task to pending status for retry.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]
		planningDir := getPlanningDirFunc()
		sm := statelib.NewStateManager(planningDir)

		if err := statelib.RetryTask(sm, taskID); err != nil {
			return fmt.Errorf("failed to retry task: %w", err)
		}

		fmt.Printf("Reset task %s to pending\n", taskID)
		return nil
	},
}

var skipReason string

var taskSkipCmd = &cobra.Command{
	Use:   "skip <task-id>",
	Short: "Skip a task",
	Long:  `Marks a pending or blocked task as skipped without blocking dependents.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]
		planningDir := getPlanningDirFunc()
		sm := statelib.NewStateManager(planningDir)

		if err := statelib.SkipTask(sm, taskID, skipReason); err != nil {
			return fmt.Errorf("failed to skip task: %w", err)
		}

		fmt.Printf("Skipped task %s\n", taskID)
		return nil
	},
}

var readyCmd = &cobra.Command{
	Use:   "ready",
	Short: "List ready tasks",
	Long:  `Lists all tasks that are ready to execute (pending with all dependencies satisfied).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()
		sm := statelib.NewStateManager(planningDir)

		state, err := sm.Load()
		if err != nil {
			return fmt.Errorf("failed to load state: %w", err)
		}

		ready := statelib.GetReadyTasks(state)
		if len(ready) == 0 {
			fmt.Println("No tasks ready for execution")
			return nil
		}

		fmt.Printf("Ready tasks (%d):\n", len(ready))
		for _, task := range ready {
			fmt.Printf("  %s: %s (phase %d)\n", task.ID, task.Name, task.Phase)
		}

		return nil
	},
}

var getFieldCmd = &cobra.Command{
	Use:   "get-field <field-name>",
	Short: "Get a field from state.json",
	Long: `Extracts a top-level field value from state.json.
Supported fields: target_dir, version, phase, created_at, updated_at`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fieldName := args[0]
		planningDir := getPlanningDirFunc()
		sm := statelib.NewStateManager(planningDir)

		state, err := sm.Load()
		if err != nil {
			return fmt.Errorf("failed to load state: %w", err)
		}

		var value string
		switch fieldName {
		case "target_dir":
			value = state.TargetDir
		case "version":
			value = state.Version
		case "phase":
			value = state.Phase.Current
		case "created_at":
			value = state.CreatedAt
		case "updated_at":
			value = state.UpdatedAt
		default:
			return fmt.Errorf("unknown field: %s (supported: target_dir, version, phase, created_at, updated_at)", fieldName)
		}

		fmt.Print(value)
		return nil
	},
}

var advanceCmd = &cobra.Command{
	Use:   "advance",
	Short: "Advance to next phase",
	Long:  `Attempts to advance the decomposition to the next phase if requirements are met.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()
		sm := statelib.NewStateManager(planningDir)

		nextPhase, err := statelib.AdvancePhase(sm)
		if err != nil {
			return fmt.Errorf("failed to advance phase: %w", err)
		}

		fmt.Printf("Advanced to phase: %s\n", nextPhase)
		return nil
	},
}

var loadTasksCmd = &cobra.Command{
	Use:   "load-tasks",
	Short: "Load tasks from tasks directory",
	Long:  `Loads individual task files from the tasks/ directory into state.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()
		sm := statelib.NewStateManager(planningDir)

		tasks, err := statelib.LoadTasks(planningDir)
		if err != nil {
			return fmt.Errorf("failed to load tasks: %w", err)
		}

		state, err := sm.Load()
		if err != nil {
			return fmt.Errorf("failed to load state: %w", err)
		}

		for id, task := range tasks {
			if _, exists := state.Tasks[id]; !exists {
				state.Tasks[id] = task
			}
		}

		if err := sm.Save(state); err != nil {
			return fmt.Errorf("failed to save state: %w", err)
		}

		fmt.Printf("Loaded %d tasks\n", len(tasks))
		return nil
	},
}

var (
	haltReason string
)

var haltCmd = &cobra.Command{
	Use:   "halt",
	Short: "Request graceful halt",
	Long:  `Requests a graceful halt of execution. Running tasks will complete before stopping.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()
		sm := statelib.NewStateManager(planningDir)

		if err := sm.RequestHalt(haltReason, "cli"); err != nil {
			return fmt.Errorf("failed to request halt: %w", err)
		}

		fmt.Println("Halt requested")
		if haltReason != "" {
			fmt.Printf("Reason: %s\n", haltReason)
		}
		return nil
	},
}

var checkHaltCmd = &cobra.Command{
	Use:   "check-halt",
	Short: "Check if halt requested",
	Long:  `Checks if a halt has been requested. Exits with code 1 if halted.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()
		sm := statelib.NewStateManager(planningDir)

		halted, err := sm.CheckHalt()
		if err != nil {
			return fmt.Errorf("failed to check halt: %w", err)
		}

		if halted {
			fmt.Println("HALTED")
			return fmt.Errorf("halt requested")
		}

		fmt.Println("OK")
		return nil
	},
}

var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume execution",
	Long:  `Clears the halt flag and resumes execution.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()
		sm := statelib.NewStateManager(planningDir)

		if err := sm.ResumeExecution(); err != nil {
			return fmt.Errorf("failed to resume: %w", err)
		}

		fmt.Println("Execution resumed")
		return nil
	},
}

var haltStatusCmd = &cobra.Command{
	Use:   "halt-status",
	Short: "Show halt status",
	Long:  `Shows the current halt status including reason and timestamp.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()
		sm := statelib.NewStateManager(planningDir)

		status, err := sm.GetHaltStatus()
		if err != nil {
			return fmt.Errorf("failed to get halt status: %w", err)
		}

		if status.Halted {
			fmt.Println("Status: HALTED")
			if status.Reason != "" {
				fmt.Printf("Reason: %s\n", status.Reason)
			}
			if status.RequestedAt != "" {
				fmt.Printf("Requested at: %s\n", status.RequestedAt)
			}
			if status.RequestedBy != "" {
				fmt.Printf("Requested by: %s\n", status.RequestedBy)
			}
		} else {
			fmt.Println("Status: RUNNING")
		}
		return nil
	},
}

var (
	metricsFormat string
)

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Show performance metrics",
	Long:  `Displays performance metrics including task success rate, duration, and costs.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()
		sm := statelib.NewStateManager(planningDir)

		metrics, err := sm.GetMetrics()
		if err != nil {
			return fmt.Errorf("failed to get metrics: %w", err)
		}

		if metricsFormat == "json" {
			fmt.Printf(`{"total_tasks":%d,"completed":%d,"failed":%d,"success_rate":%.1f,"total_tokens":%d,"total_cost_usd":%.4f}`,
				metrics.TotalTasks, metrics.CompletedTasks, metrics.FailedTasks,
				metrics.SuccessRate, metrics.TotalTokens, metrics.TotalCostUSD)
			fmt.Println()
		} else {
			fmt.Printf("Total tasks: %d\n", metrics.TotalTasks)
			fmt.Printf("Completed: %d\n", metrics.CompletedTasks)
			fmt.Printf("Failed: %d\n", metrics.FailedTasks)
			fmt.Printf("Success rate: %.1f%%\n", metrics.SuccessRate)
			fmt.Printf("Total tokens: %d\n", metrics.TotalTokens)
			fmt.Printf("Total cost: $%.4f\n", metrics.TotalCostUSD)
			if metrics.AvgDurationSecs > 0 {
				fmt.Printf("Avg duration: %.1fs\n", metrics.AvgDurationSecs)
			}
		}
		return nil
	},
}

var planningMetricsCmd = &cobra.Command{
	Use:   "planning-metrics",
	Short: "Show planning metrics",
	Long:  `Displays planning quality metrics including phase progress and task distribution.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()
		sm := statelib.NewStateManager(planningDir)

		metrics, err := sm.GetPlanningMetrics()
		if err != nil {
			return fmt.Errorf("failed to get planning metrics: %w", err)
		}

		fmt.Printf("Total phases: %d\n", metrics.TotalPhases)
		fmt.Printf("Current phase: %d\n", metrics.CurrentPhase)
		fmt.Printf("Overall progress: %.1f%%\n", metrics.OverallProgress)
		fmt.Printf("Ready tasks: %d\n", metrics.ReadyTasks)
		fmt.Printf("Blocked tasks: %d\n", metrics.BlockedTasks)
		fmt.Printf("Remaining tasks: %d\n", metrics.EstimatedRemaining)
		return nil
	},
}

var failureMetricsCmd = &cobra.Command{
	Use:   "failure-metrics",
	Short: "Show failure breakdown",
	Long:  `Displays failure classification breakdown by category.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()
		sm := statelib.NewStateManager(planningDir)

		metrics, err := sm.GetFailureMetrics()
		if err != nil {
			return fmt.Errorf("failed to get failure metrics: %w", err)
		}

		fmt.Printf("Total failures: %d\n", metrics.TotalFailures)
		fmt.Printf("Retryable: %d\n", metrics.RetryableCount)
		fmt.Printf("Non-retryable: %d\n", metrics.NonRetryableCount)

		if len(metrics.ByCategory) > 0 {
			fmt.Println("\nBy category:")
			for cat, count := range metrics.ByCategory {
				fmt.Printf("  %s: %d\n", cat, count)
			}
		}
		return nil
	},
}

var (
	logTokensInput  int
	logTokensOutput int
	logTokensCost   float64
	logTokensModel  string
)

var logTokensCmd = &cobra.Command{
	Use:   "log-tokens <task-id>",
	Short: "Log token usage",
	Long:  `Records token usage for a task including input/output tokens and cost.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]
		planningDir := getPlanningDirFunc()
		sm := statelib.NewStateManager(planningDir)

		if err := sm.LogTokens(taskID, logTokensInput, logTokensOutput, logTokensCost, logTokensModel); err != nil {
			return fmt.Errorf("failed to log tokens: %w", err)
		}

		fmt.Printf("Logged %d tokens for %s\n", logTokensInput+logTokensOutput, taskID)
		return nil
	},
}

var validateCmd = &cobra.Command{
	Use:   "validate <artifact-type>",
	Short: "Validate an artifact",
	Long:  `Validates an artifact against its schema. Supported types: capability_map, physical_map`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		artifactType := args[0]
		planningDir := getPlanningDirFunc()

		pathMap := map[string]string{
			"capability_map": "artifacts/capability-map.json",
			"physical_map":   "artifacts/physical-map.json",
		}

		schemaMap := map[string]string{
			"capability_map": "../schemas/capability-map.schema.json",
			"physical_map":   "../schemas/physical-map.schema.json",
		}

		artifactPath, ok := pathMap[artifactType]
		if !ok {
			return fmt.Errorf("unknown artifact type: %s (supported: capability_map, physical_map)", artifactType)
		}

		result, err := statelib.ValidateArtifact(planningDir, artifactPath, schemaMap[artifactType])
		if err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}

		if result.Valid {
			fmt.Printf("%s: valid\n", artifactType)
		} else {
			fmt.Printf("%s: invalid\n", artifactType)
			for _, e := range result.Errors {
				fmt.Printf("  - %s\n", e)
			}
			return fmt.Errorf("validation failed")
		}

		sm := statelib.NewStateManager(planningDir)
		state, err := sm.Load()
		if err != nil {
			return fmt.Errorf("failed to load state: %w", err)
		}

		artifactRef := &statelib.ArtifactRef{
			Path:        artifactPath,
			Valid:       result.Valid,
			ValidatedAt: result.ValidatedAt,
		}

		switch artifactType {
		case "capability_map":
			state.Artifacts.CapabilityMap = artifactRef
		case "physical_map":
			state.Artifacts.PhysicalMap = artifactRef
		}

		if err := sm.Save(state); err != nil {
			return fmt.Errorf("failed to save state: %w", err)
		}

		return nil
	},
}

var (
	validateTasksVerdict  string
	validateTasksSummary  string
	validateTasksIssues   []string
)

var checkpointCmd = &cobra.Command{
	Use:   "checkpoint",
	Short: "Checkpoint management commands",
	Long:  `Commands for managing execution checkpoints for crash recovery.`,
}

var checkpointCreateCmd = &cobra.Command{
	Use:   "create <task-id>...",
	Short: "Create a checkpoint",
	Long:  `Creates a new checkpoint with the specified tasks.`,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()

		cp, err := statelib.CreateCheckpoint(planningDir, args)
		if err != nil {
			return fmt.Errorf("failed to create checkpoint: %w", err)
		}

		fmt.Printf("Checkpoint created: %s\n", cp.BatchID)
		fmt.Printf("Tasks: %s\n", strings.Join(args, ", "))
		return nil
	},
}

var (
	checkpointUpdateStatus string
)

var checkpointUpdateCmd = &cobra.Command{
	Use:   "update <task-id>",
	Short: "Update task status in checkpoint",
	Long:  `Updates a task's status in the current checkpoint.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]
		planningDir := getPlanningDirFunc()

		if err := statelib.UpdateCheckpointTask(planningDir, taskID, checkpointUpdateStatus); err != nil {
			return fmt.Errorf("failed to update checkpoint: %w", err)
		}

		fmt.Printf("Updated %s: %s\n", taskID, checkpointUpdateStatus)
		return nil
	},
}

var checkpointCompleteCmd = &cobra.Command{
	Use:   "complete",
	Short: "Mark checkpoint as complete",
	Long:  `Marks the current checkpoint as complete.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()

		if err := statelib.CompleteCheckpoint(planningDir); err != nil {
			return fmt.Errorf("failed to complete checkpoint: %w", err)
		}

		fmt.Println("Checkpoint marked complete")
		return nil
	},
}

var checkpointStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show checkpoint status",
	Long:  `Displays the current checkpoint status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()

		cp, err := statelib.LoadCheckpoint(planningDir)
		if err != nil {
			return fmt.Errorf("failed to load checkpoint: %w", err)
		}

		if cp == nil {
			fmt.Println("No active checkpoint")
			return nil
		}

		fmt.Printf("Batch ID: %s\n", cp.BatchID)
		fmt.Printf("Status: %s\n", cp.Status)
		fmt.Printf("Spawned: %s\n", cp.SpawnedAt)
		fmt.Printf("Pending: %d (%s)\n", len(cp.Tasks.Pending), strings.Join(cp.Tasks.Pending, ", "))
		fmt.Printf("Completed: %d (%s)\n", len(cp.Tasks.Completed), strings.Join(cp.Tasks.Completed, ", "))
		fmt.Printf("Failed: %d (%s)\n", len(cp.Tasks.Failed), strings.Join(cp.Tasks.Failed, ", "))
		return nil
	},
}

var checkpointRecoverCmd = &cobra.Command{
	Use:   "recover",
	Short: "Recover orphaned tasks",
	Long:  `Attempts to recover tasks that completed but weren't recorded in the checkpoint.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()

		info, err := statelib.RecoverCheckpoint(planningDir)
		if err != nil {
			return fmt.Errorf("failed to recover: %w", err)
		}

		if len(info.RecoveredTasks) > 0 {
			fmt.Printf("Recovered %d tasks:\n", len(info.RecoveredTasks))
			for _, t := range info.RecoveredTasks {
				fmt.Printf("  %s: %s\n", t.TaskID, t.Status)
			}
		}

		if len(info.OrphanedTasks) > 0 {
			fmt.Printf("Orphaned tasks (need manual intervention): %s\n", strings.Join(info.OrphanedTasks, ", "))
		}

		if len(info.RecoveredTasks) == 0 && len(info.OrphanedTasks) == 0 {
			fmt.Println("No tasks to recover")
		}

		return nil
	},
}

var checkpointClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear checkpoint",
	Long:  `Removes the checkpoint file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()

		if err := statelib.ClearCheckpoint(planningDir); err != nil {
			return fmt.Errorf("failed to clear checkpoint: %w", err)
		}

		fmt.Println("Checkpoint cleared")
		return nil
	},
}

var (
	verifyVerdict        string
	verifyRecommendation string
)

var recordVerificationCmd = &cobra.Command{
	Use:   "record-verification <task-id>",
	Short: "Record verification results",
	Long:  `Records verification results for a completed task.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]
		planningDir := getPlanningDirFunc()
		sm := statelib.NewStateManager(planningDir)

		if err := statelib.RecordVerification(sm, taskID, strings.ToUpper(verifyVerdict), strings.ToUpper(verifyRecommendation), nil, nil, nil); err != nil {
			return fmt.Errorf("failed to record verification: %w", err)
		}

		fmt.Printf("Verification recorded for %s: %s (%s)\n", taskID, verifyVerdict, verifyRecommendation)
		return nil
	},
}

var (
	calibrationOutcome string
	calibrationNotes   string
)

var recordCalibrationCmd = &cobra.Command{
	Use:   "record-calibration <task-id>",
	Short: "Record calibration data",
	Long:  `Records calibration data for verifier accuracy tracking.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskID := args[0]
		planningDir := getPlanningDirFunc()
		sm := statelib.NewStateManager(planningDir)

		if err := statelib.RecordCalibration(sm, taskID, calibrationOutcome, calibrationNotes); err != nil {
			return fmt.Errorf("failed to record calibration: %w", err)
		}

		fmt.Printf("Calibration recorded for %s: %s\n", taskID, calibrationOutcome)
		return nil
	},
}

var calibrationScoreCmd = &cobra.Command{
	Use:   "calibration-score",
	Short: "Show calibration score",
	Long:  `Shows the current verifier calibration score.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()
		sm := statelib.NewStateManager(planningDir)

		score, cal, err := statelib.GetCalibrationScore(sm)
		if err != nil {
			return fmt.Errorf("failed to get calibration score: %w", err)
		}

		fmt.Printf("Calibration score: %.1f%%\n", score*100)
		if cal != nil {
			fmt.Printf("Total verified: %d\n", cal.TotalVerified)
			fmt.Printf("Correct: %d\n", cal.Correct)
			fmt.Printf("False positives: %d\n", len(cal.FalsePositives))
			fmt.Printf("False negatives: %d\n", len(cal.FalseNegatives))
		}
		return nil
	},
}

var validateTasksCmd = &cobra.Command{
	Use:   "validate-tasks",
	Short: "Register task validation results",
	Long:  `Registers the result of task validation. Verdict must be READY, READY_WITH_NOTES, or BLOCKED.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()
		sm := statelib.NewStateManager(planningDir)

		state, err := sm.Load()
		if err != nil {
			return fmt.Errorf("failed to load state: %w", err)
		}

		verdict := strings.ToUpper(validateTasksVerdict)
		validVerdicts := map[string]bool{"READY": true, "READY_WITH_NOTES": true, "BLOCKED": true}
		if !validVerdicts[verdict] {
			return fmt.Errorf("invalid verdict: %s (must be READY, READY_WITH_NOTES, or BLOCKED)", verdict)
		}

		isValid := verdict == "READY" || verdict == "READY_WITH_NOTES"

		state.Artifacts.TaskValidation = &statelib.TaskValidation{
			Verdict:     verdict,
			Valid:       isValid,
			Summary:     validateTasksSummary,
			Issues:      validateTasksIssues,
			ValidatedAt: state.UpdatedAt,
		}

		if err := sm.Save(state); err != nil {
			return fmt.Errorf("failed to save state: %w", err)
		}

		fmt.Printf("Task validation: %s\n", verdict)
		return nil
	},
}

func init() {
	taskCompleteCmd.Flags().StringSliceVar(&filesCreated, "created", []string{}, "Files created by this task")
	taskCompleteCmd.Flags().StringSliceVar(&filesModified, "modified", []string{}, "Files modified by this task")

	taskFailCmd.Flags().StringVar(&failCategory, "category", "unknown", "Failure category (test, build, spec, env)")
	taskFailCmd.Flags().BoolVar(&failRetryable, "retryable", true, "Whether the task can be retried")

	taskSkipCmd.Flags().StringVar(&skipReason, "reason", "", "Reason for skipping the task")

	haltCmd.Flags().StringVar(&haltReason, "reason", "", "Reason for halt")

	metricsCmd.Flags().StringVar(&metricsFormat, "format", "text", "Output format (text or json)")

	logTokensCmd.Flags().IntVar(&logTokensInput, "input", 0, "Input tokens")
	logTokensCmd.Flags().IntVar(&logTokensOutput, "output", 0, "Output tokens")
	logTokensCmd.Flags().Float64Var(&logTokensCost, "cost", 0.0, "Cost in USD")
	logTokensCmd.Flags().StringVar(&logTokensModel, "model", "", "Model name")

	validateTasksCmd.Flags().StringVar(&validateTasksVerdict, "verdict", "", "Validation verdict (READY, READY_WITH_NOTES, BLOCKED)")
	validateTasksCmd.Flags().StringVar(&validateTasksSummary, "summary", "", "Validation summary")
	validateTasksCmd.Flags().StringSliceVar(&validateTasksIssues, "issues", []string{}, "List of issues found")
	validateTasksCmd.MarkFlagRequired("verdict")

	checkpointUpdateCmd.Flags().StringVar(&checkpointUpdateStatus, "status", "", "Task status (success, failed)")
	checkpointUpdateCmd.MarkFlagRequired("status")

	checkpointCmd.AddCommand(checkpointCreateCmd)
	checkpointCmd.AddCommand(checkpointUpdateCmd)
	checkpointCmd.AddCommand(checkpointCompleteCmd)
	checkpointCmd.AddCommand(checkpointStatusCmd)
	checkpointCmd.AddCommand(checkpointRecoverCmd)
	checkpointCmd.AddCommand(checkpointClearCmd)

	recordVerificationCmd.Flags().StringVar(&verifyVerdict, "verdict", "", "Verification verdict (PASS, FAIL, CONDITIONAL)")
	recordVerificationCmd.Flags().StringVar(&verifyRecommendation, "recommendation", "", "Recommendation (PROCEED, BLOCK)")
	recordVerificationCmd.MarkFlagRequired("verdict")
	recordVerificationCmd.MarkFlagRequired("recommendation")

	recordCalibrationCmd.Flags().StringVar(&calibrationOutcome, "outcome", "", "Actual outcome (correct, false_positive, false_negative)")
	recordCalibrationCmd.Flags().StringVar(&calibrationNotes, "notes", "", "Optional notes")
	recordCalibrationCmd.MarkFlagRequired("outcome")

	taskCmd.AddCommand(taskStartCmd)
	taskCmd.AddCommand(taskCompleteCmd)
	taskCmd.AddCommand(taskFailCmd)
	taskCmd.AddCommand(taskRetryCmd)
	taskCmd.AddCommand(taskSkipCmd)

	stateCmd.AddCommand(initCmd)
	stateCmd.AddCommand(statusCmd)
	stateCmd.AddCommand(taskCmd)
	stateCmd.AddCommand(readyCmd)
	stateCmd.AddCommand(getFieldCmd)
	stateCmd.AddCommand(advanceCmd)
	stateCmd.AddCommand(loadTasksCmd)
	stateCmd.AddCommand(haltCmd)
	stateCmd.AddCommand(checkHaltCmd)
	stateCmd.AddCommand(resumeCmd)
	stateCmd.AddCommand(haltStatusCmd)
	stateCmd.AddCommand(metricsCmd)
	stateCmd.AddCommand(planningMetricsCmd)
	stateCmd.AddCommand(failureMetricsCmd)
	stateCmd.AddCommand(logTokensCmd)
	stateCmd.AddCommand(validateCmd)
	stateCmd.AddCommand(validateTasksCmd)
	stateCmd.AddCommand(checkpointCmd)
	stateCmd.AddCommand(recordVerificationCmd)
	stateCmd.AddCommand(recordCalibrationCmd)
	stateCmd.AddCommand(calibrationScoreCmd)

	command.RootCmd.AddCommand(stateCmd)
}
