package tui

import (
	"errors"
	"testing"
)

func withMocks(planningDir string, runErr error, fn func()) {
	originalPlanningDir := getPlanningDirFunc
	originalTuiRun := tuiRunFunc

	getPlanningDirFunc = func() string { return planningDir }
	tuiRunFunc = func(dir string) error { return runErr }

	defer func() {
		getPlanningDirFunc = originalPlanningDir
		tuiRunFunc = originalTuiRun
	}()

	fn()
}

func TestTuiCmd(t *testing.T) {
	t.Run("succeeds when TUI runs without error", func(t *testing.T) {
		withMocks("/test/planning", nil, func() {
			err := tuiCmd.RunE(tuiCmd, []string{})
			if err != nil {
				t.Errorf("tuiCmd should succeed, got: %v", err)
			}
		})
	})

	t.Run("fails when TUI returns error", func(t *testing.T) {
		withMocks("/test/planning", errors.New("tui failed"), func() {
			err := tuiCmd.RunE(tuiCmd, []string{})
			if err == nil {
				t.Error("tuiCmd should fail when TUI returns error")
			}
		})
	})
}
