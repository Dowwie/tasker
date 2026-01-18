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

func init() {
	taskCompleteCmd.Flags().StringSliceVar(&filesCreated, "created", []string{}, "Files created by this task")
	taskCompleteCmd.Flags().StringSliceVar(&filesModified, "modified", []string{}, "Files modified by this task")

	taskFailCmd.Flags().StringVar(&failCategory, "category", "unknown", "Failure category (test, build, spec, env)")
	taskFailCmd.Flags().BoolVar(&failRetryable, "retryable", true, "Whether the task can be retried")

	taskSkipCmd.Flags().StringVar(&skipReason, "reason", "", "Reason for skipping the task")

	taskCmd.AddCommand(taskStartCmd)
	taskCmd.AddCommand(taskCompleteCmd)
	taskCmd.AddCommand(taskFailCmd)
	taskCmd.AddCommand(taskRetryCmd)
	taskCmd.AddCommand(taskSkipCmd)

	stateCmd.AddCommand(initCmd)
	stateCmd.AddCommand(statusCmd)
	stateCmd.AddCommand(taskCmd)
	stateCmd.AddCommand(readyCmd)

	command.RootCmd.AddCommand(stateCmd)
}
