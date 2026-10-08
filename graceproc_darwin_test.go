package graceproc

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestCleanupAcceptsExitedUnreapedProcessGroup(t *testing.T) {
	if running, err := groupRunning(syscall.Getpgrp()); err != nil || !running {
		t.Fatalf("live group reported running=%v, err=%v", running, err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(executable, "-test.run=TestDarwinExitHelper")
	cmd.Env = append(os.Environ(), "LATTICEBUILD_PROCESS_TEST_EXIT=1")
	configureProcess(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		if err := cmd.Wait(); err != nil {
			t.Errorf("reap exited child: %v", err)
		}
	})
	// Wait for kernel-reported exit without reaping. This retains the zombie
	// group deterministically instead of racing a short-lived descendant.
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	poll := time.NewTicker(time.Millisecond)
	defer poll.Stop()
	for {
		status, err := unix.SysctlKinfoProc("kern.proc.pid", cmd.Process.Pid)
		if err != nil {
			t.Fatal(err)
		}
		if status != nil && status.Proc.P_stat == 5 {
			break
		}
		select {
		case <-deadline.C:
			t.Fatal("child did not reach exited state")
		case <-poll.C:
		}
	}
	if running, err := groupRunning(cmd.Process.Pid); err != nil || running {
		t.Fatalf("exited group reported running=%v, err=%v", running, err)
	}
	_, cleanup, err := controlProcess(cmd.Process)
	if err != nil {
		t.Fatal(err)
	}
	if err := cleanup(); err != nil {
		t.Fatalf("exited group cleanup failed: %v", err)
	}
}

func TestDarwinExitHelper(t *testing.T) {
	if os.Getenv("LATTICEBUILD_PROCESS_TEST_EXIT") == "1" {
		os.Exit(0)
	}
}
