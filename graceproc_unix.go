//go:build !windows

package graceproc

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// Signals are the supported cancellation signals on this platform.
func Signals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}
}

// SignalCode returns the conventional exit status for cancellation.
func SignalCode(signal os.Signal) int {
	return 128 + int(signal.(syscall.Signal))
}

func processExitCode(err *exec.ExitError) int {
	if status, ok := err.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return SignalCode(status.Signal())
	}
	return err.ExitCode()
}

func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func controlProcess(process *os.Process) (func(os.Signal), func() error, error) {
	forward := func(signal os.Signal) { _ = syscall.Kill(-process.Pid, signal.(syscall.Signal)) }
	var once sync.Once
	var result error
	cleanup := func() error {
		once.Do(func() {
			var signalError error
			if err := syscall.Kill(-process.Pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
				signalError = fmt.Errorf("signal process group %d: %w", process.Pid, err)
				if err != syscall.EPERM {
					result = signalError
					return
				}
				// Darwin can reject a group while its members are exiting,
				// including zombies. Other Unix groups may disappear between
				// signal and inspection. Wait for confirmed exit, retaining the
				// signal failure if inspection fails or live members remain.
			}
			deadline := time.NewTimer(3 * time.Second)
			defer deadline.Stop()
			poll := time.NewTicker(10 * time.Millisecond)
			defer poll.Stop()
			for {
				running, err := groupRunning(process.Pid)
				if err != nil || !running {
					result = err
					if err != nil && signalError != nil {
						result = fmt.Errorf("%w; inspect group: %v", signalError, err)
					}
					return
				}
				select {
				case <-deadline.C:
					result = fmt.Errorf("process group %d did not finish cleanup", process.Pid)
					if signalError != nil {
						result = signalError
					}
					return
				case <-poll.C:
				}
			}
		})
		return result
	}
	return forward, cleanup, nil
}
