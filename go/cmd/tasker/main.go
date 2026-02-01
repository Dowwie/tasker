package main

import (
	"fmt"

	"github.com/Dowwie/tasker/internal/command"
	_ "github.com/Dowwie/tasker/internal/command/archive"
	_ "github.com/Dowwie/tasker/internal/command/bundle"
	_ "github.com/Dowwie/tasker/internal/command/evaluate"
	_ "github.com/Dowwie/tasker/internal/command/fsm"
	_ "github.com/Dowwie/tasker/internal/command/hook"
	_ "github.com/Dowwie/tasker/internal/command/spec"
	_ "github.com/Dowwie/tasker/internal/command/state"
	_ "github.com/Dowwie/tasker/internal/command/stop"
	_ "github.com/Dowwie/tasker/internal/command/transform"
	_ "github.com/Dowwie/tasker/internal/command/tui"
	_ "github.com/Dowwie/tasker/internal/command/validate"
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
