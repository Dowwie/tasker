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
	Short: "Agentic Engineering CLI",
	Long: `Tasker CLI manages the deterministic capabilities associated with Tasker's agentic development pipeline.  It is not intended for manual usage.`,
}

func init() {
	RootCmd.PersistentFlags().StringVarP(&planningDir, "planning-dir", "p", ".tasker", "Path to tasker working directory (typically $TARGET_DIR/.tasker)")
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
