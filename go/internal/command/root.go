package command

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	planningDir string
)

var RootCmd = &cobra.Command{
	Use:   "tasker",
	Short: "Task decomposition and execution management",
	Long: `Tasker is a CLI for managing task decomposition workflows.

It provides commands for state management, task lifecycle operations,
and planning validation.`,
}

func init() {
	RootCmd.PersistentFlags().StringVarP(&planningDir, "planning-dir", "p", "project-planning", "Path to planning directory")
}

func GetPlanningDir() string {
	return planningDir
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
