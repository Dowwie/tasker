package tui

import (
	"fmt"

	"github.com/dgordon/tasker/internal/command"
	"github.com/dgordon/tasker/internal/tui"
	"github.com/spf13/cobra"
)

// getPlanningDirFunc is a function variable for dependency injection in tests
var getPlanningDirFunc = command.GetPlanningDir

// tuiRunFunc is a function variable for dependency injection in tests
var tuiRunFunc = tui.Run

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch interactive TUI mode",
	Long: `Launches the interactive terminal user interface for managing tasks.

The TUI provides keyboard-driven navigation through tasks and their states.

Navigation keys:
  j/k or up/down  - Navigate through task list
  enter           - Select/view task details
  r               - Refresh state
  q               - Quit`,
	RunE: func(cmd *cobra.Command, args []string) error {
		planningDir := getPlanningDirFunc()
		if err := tuiRunFunc(planningDir); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		return nil
	},
}

func init() {
	command.RootCmd.AddCommand(tuiCmd)
}
