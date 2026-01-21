package stop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dgordon/tasker/internal/command"
	statelib "github.com/dgordon/tasker/internal/state"
)

func TestStopAndResume(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "stop-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	planningDir := filepath.Join(tmpDir, ".tasker")
	if err := os.MkdirAll(planningDir, 0755); err != nil {
		t.Fatalf("failed to create planning dir: %v", err)
	}

	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("failed to create target dir: %v", err)
	}

	if _, err := statelib.InitDecomposition(planningDir, targetDir); err != nil {
		t.Fatalf("failed to init decomposition: %v", err)
	}

	sm := statelib.NewStateManager(planningDir)

	halted, err := sm.CheckHalt()
	if err != nil {
		t.Fatalf("failed to check halt: %v", err)
	}
	if halted {
		t.Error("expected not halted initially")
	}

	if err := sm.RequestHalt("test stop", "cli"); err != nil {
		t.Fatalf("failed to request halt: %v", err)
	}

	halted, err = sm.CheckHalt()
	if err != nil {
		t.Fatalf("failed to check halt after stop: %v", err)
	}
	if !halted {
		t.Error("expected halted after stop")
	}

	if err := sm.ResumeExecution(); err != nil {
		t.Fatalf("failed to resume: %v", err)
	}

	halted, err = sm.CheckHalt()
	if err != nil {
		t.Fatalf("failed to check halt after resume: %v", err)
	}
	if halted {
		t.Error("expected not halted after resume")
	}
}

func TestStopCommandRegistration(t *testing.T) {
	found := false
	for _, cmd := range command.RootCmd.Commands() {
		if cmd.Use == "stop" {
			found = true
			break
		}
	}
	if !found {
		t.Error("stop command not registered on root")
	}

	found = false
	for _, cmd := range command.RootCmd.Commands() {
		if cmd.Use == "resume" {
			found = true
			break
		}
	}
	if !found {
		t.Error("resume command not registered on root")
	}
}
