package stop

import (
	"fmt"

	"github.com/dgordon/tasker/internal/command"
	statelib "github.com/dgordon/tasker/internal/state"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop execution gracefully",
	Long: `Requests a graceful stop of task execution.

The current running task will complete before stopping. No new tasks will be started.
Use 'tasker resume' to clear the stop and continue execution.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := command.GetPlanningDir()
		sm := statelib.NewStateManager(planningDir)

		if err := sm.RequestHalt("user_stop", "cli"); err != nil {
			return fmt.Errorf("failed to request stop: %w", err)
		}

		fmt.Println("Stop requested. Current task will complete before halting.")
		fmt.Println("Use 'tasker resume' to continue execution.")
		return nil
	},
}

var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume execution after stop",
	Long: `Clears the stop flag and allows execution to continue.

After resuming, run '/execute' again to continue processing tasks.

This is a convenience alias for 'tasker state resume'.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := command.GetPlanningDir()
		sm := statelib.NewStateManager(planningDir)

		status, err := sm.GetHaltStatus()
		if err != nil {
			return fmt.Errorf("failed to check status: %w", err)
		}

		if !status.Halted {
			fmt.Println("Execution is not stopped. Nothing to resume.")
			return nil
		}

		if err := sm.ResumeExecution(); err != nil {
			return fmt.Errorf("failed to resume: %w", err)
		}

		fmt.Println("Execution resumed. Run '/execute' to continue processing tasks.")
		return nil
	},
}

func init() {
	command.RootCmd.AddCommand(stopCmd)
	command.RootCmd.AddCommand(resumeCmd)
}
