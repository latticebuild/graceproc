// Package graceproc owns cancellation and cleanup of native tool invocations.
package graceproc

import (
	"errors"
	"os"
	"os/exec"
	"time"
)

// Run forwards cancellation to the child process group, allowing graceful
// shutdown before forcing termination. Before returning successfully, cleanup
// confirms that the owned Unix process group or Windows job has no live members,
// allowing callers to reuse their ports. Unix descendants that create a separate
// process group or session are outside that ownership.
func Run(cmd *exec.Cmd, signals <-chan os.Signal) (code int, err error) {
	return RunWithGrace(cmd, signals, 3*time.Second)
}

// RunWithGrace gives a supervising process more time than its child's own
// cleanup budget. Each of the two escalation stages lasts grace; the default
// Run policy remains three seconds per stage.
func RunWithGrace(cmd *exec.Cmd, signals <-chan os.Signal, grace time.Duration) (code int, err error) {
	if grace <= 0 {
		return 1, errors.New("process grace period must be positive")
	}
	configureProcess(cmd)
	if err := cmd.Start(); err != nil {
		return 1, err
	}
	forward, cleanup, err := controlProcess(cmd.Process)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return 1, err
	}
	defer func() {
		err = errors.Join(err, cleanup())
		if err != nil && code == 0 {
			code = 1
		}
	}()
	finished := make(chan error, 1)
	go func() { finished <- cmd.Wait() }()
	var interrupted os.Signal
	var deadline <-chan time.Time
	var timer *time.Timer
	forced := false
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	for {
		select {
		case err := <-finished:
			if interrupted != nil {
				return SignalCode(interrupted), nil
			}
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				return processExitCode(exit), nil
			}
			if err != nil {
				return 1, err
			}
			return 0, nil
		case sig, open := <-signals:
			if !open {
				signals = nil
				continue
			}
			if interrupted != nil {
				if forced {
					_ = cleanup()
				} else {
					forward(sig)
					forced = true
					timer.Reset(grace)
				}
			} else {
				interrupted = sig
				forward(sig)
				timer = time.NewTimer(grace)
				deadline = timer.C
			}
		case <-deadline:
			if !forced {
				// Nextest owns separate process groups for its tests. A second
				// signal asks it to terminate those groups before we stop it.
				forward(interrupted)
				forced = true
				timer.Reset(grace)
			} else {
				_ = cleanup()
				deadline = nil
			}
		}
	}
}
