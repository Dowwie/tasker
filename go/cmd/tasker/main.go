package main

import (
	"fmt"

	"github.com/dgordon/tasker/internal/command"
	_ "github.com/dgordon/tasker/internal/command/fsm"
	_ "github.com/dgordon/tasker/internal/command/state"
	_ "github.com/dgordon/tasker/internal/command/tui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func init() {
	command.RootCmd.Version = fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date)
	command.RootCmd.SetVersionTemplate("tasker {{.Version}}\n")
}

func main() {
	command.Execute()
}
