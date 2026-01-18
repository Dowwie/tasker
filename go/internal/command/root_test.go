package command

import (
	"testing"
)

func TestGetPlanningDir(t *testing.T) {
	t.Run("returns default planning dir", func(t *testing.T) {
		result := GetPlanningDir()
		if result != "project-planning" {
			t.Errorf("expected default 'project-planning', got: %s", result)
		}
	})
}

func TestRootCmd(t *testing.T) {
	t.Run("has expected use", func(t *testing.T) {
		if RootCmd.Use != "tasker" {
			t.Errorf("expected Use 'tasker', got: %s", RootCmd.Use)
		}
	})

	t.Run("has planning-dir flag", func(t *testing.T) {
		flag := RootCmd.PersistentFlags().Lookup("planning-dir")
		if flag == nil {
			t.Error("expected planning-dir flag to exist")
		}
		if flag.Shorthand != "p" {
			t.Errorf("expected shorthand 'p', got: %s", flag.Shorthand)
		}
	})
}
