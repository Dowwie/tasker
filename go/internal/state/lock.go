package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

var ErrStateLocked = errors.New("state file is locked")

type FileLock struct {
	file     *os.File
	path     string
	isWrite  bool
}

func (fl *FileLock) Release() error {
	if fl.file == nil {
		return nil
	}

	err := syscall.Flock(int(fl.file.Fd()), syscall.LOCK_UN)
	if err != nil {
		fl.file.Close()
		return fmt.Errorf("failed to release lock: %w", err)
	}

	return fl.file.Close()
}

func acquireFileLock(path string, exclusive bool) (*FileLock, error) {
	lockPath := path + ".lock"

	lockDir := filepath.Dir(lockPath)
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create lock directory: %w", err)
	}

	flags := os.O_CREATE | os.O_RDWR
	file, err := os.OpenFile(lockPath, flags, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}

	lockType := syscall.LOCK_SH
	if exclusive {
		lockType = syscall.LOCK_EX
	}

	done := make(chan error, 1)
	go func() {
		done <- syscall.Flock(int(file.Fd()), lockType)
	}()

	timeout := 10 * time.Second
	select {
	case err := <-done:
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to acquire lock: %w", err)
		}
	case <-time.After(timeout):
		file.Close()
		return nil, fmt.Errorf("timeout acquiring lock after %v", timeout)
	}

	return &FileLock{
		file:    file,
		path:    lockPath,
		isWrite: exclusive,
	}, nil
}

func AcquireReadLock(path string) (*FileLock, error) {
	return acquireFileLock(path, false)
}

func AcquireWriteLock(path string) (*FileLock, error) {
	return acquireFileLock(path, true)
}
