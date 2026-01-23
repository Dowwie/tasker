package tui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dgordon/tasker/internal/command"
	"github.com/dgordon/tasker/internal/tui"
	"github.com/spf13/cobra"
)

// getPlanningDirFunc is a function variable for dependency injection in tests
var getPlanningDirFunc = command.GetPlanningDir

// tuiRunFunc is a function variable for dependency injection in tests
var tuiRunFunc = tui.Run

// findPlanningDirFunc finds .tasker directory by walking up from CWD
var findPlanningDirFunc = findPlanningDir

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

		// If using default, try to find .tasker by walking up from CWD
		if planningDir == ".tasker" {
			if found := findPlanningDirFunc(); found != "" {
				planningDir = found
			}
		}

		if err := tuiRunFunc(planningDir); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		return nil
	},
}

// findPlanningDir searches for .tasker directory by checking env var
// then walking up from CWD
func findPlanningDir() string {
	if dir := os.Getenv("TASKER_DIR"); dir != "" {
		return dir
	}

	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	for dir := cwd; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, ".tasker")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return ""
}

func init() {
	command.RootCmd.AddCommand(tuiCmd)
}
