package graceproc

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestJobExitRefusesUnrelatedNotifications(t *testing.T) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = windows.CloseHandle(job) })
	port, err := windows.CreateIoCompletionPort(windows.InvalidHandle, 0, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = windows.CloseHandle(port) })
	// ACTIVE_PROCESS_ZERO for another job and NEW_PROCESS for this job must
	// not report successful cleanup. Exercise the actual Windows queue.
	if err := windows.PostQueuedCompletionStatus(port, 4, uintptr(job)+1, nil); err != nil {
		t.Fatal(err)
	}
	if err := windows.PostQueuedCompletionStatus(port, 6, uintptr(job), nil); err != nil {
		t.Fatal(err)
	}
	err = waitJobExit(job, port, time.Now().Add(50*time.Millisecond))
	if err == nil || !strings.Contains(err.Error(), "did not finish cleanup") {
		t.Fatalf("unrelated notifications reported job exit: %v", err)
	}
}

func TestJobExitReportsQueueFailure(t *testing.T) {
	err := waitJobExit(0, windows.InvalidHandle, time.Now().Add(time.Second))
	var syscallErr *os.SyscallError
	if !errors.As(err, &syscallErr) || syscallErr.Syscall != "GetQueuedCompletionStatus" || !errors.Is(err, windows.ERROR_INVALID_HANDLE) {
		t.Fatalf("invalid completion port error = %v", err)
	}
}

func TestWindowsDescendantsReleasePortsBeforeReturn(t *testing.T) {
	for _, mode := range []string{"exit", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, executable, "-test.run=^TestWindowsDescendantHelper$", "--", mode)
			command.Env = append(os.Environ(), "LATTICEBUILD_GRACEPROC_TEST_HELPER=1")
			output, ready, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = output.Close()
				_ = ready.Close()
			})
			// Keep readiness readable after Run waits for a fast-exiting parent.
			command.Stdout = ready
			signals := make(chan os.Signal, 1)
			done := make(chan struct{})
			var code int
			var runErr error
			go func() {
				defer close(done)
				defer func() { _ = ready.Close() }()
				code, runErr = Run(command, signals)
			}()
			t.Cleanup(func() {
				cancel()
				select {
				case <-done:
				case <-time.After(12 * time.Second):
					t.Error("supervisor did not finish cleanup")
				}
			})
			line, err := bufio.NewReader(output).ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			fields := strings.Fields(line)
			if len(fields) != 2 {
				t.Fatalf("invalid descendant readiness: %q", line)
			}
			pid, err := strconv.ParseUint(fields[0], 10, 32)
			if err != nil {
				t.Fatal(err)
			}
			// Keep a handle so a failure can still clean up a descendant that
			// escaped job assignment, even after its fast parent has exited.
			handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, uint32(pid))
			if err == nil {
				t.Cleanup(func() {
					defer func() { _ = windows.CloseHandle(handle) }()
					_ = windows.TerminateProcess(handle, 1)
					if status, err := windows.WaitForSingleObject(handle, 5000); err != nil || status != windows.WAIT_OBJECT_0 {
						t.Errorf("descendant cleanup: status=%d, err=%v", status, err)
					}
				})
			} else if err != windows.ERROR_INVALID_PARAMETER {
				t.Fatal(err)
			}
			expected := 0
			if mode == "cancel" {
				signals <- os.Interrupt
				expected = SignalCode(os.Interrupt)
			}
			select {
			case <-done:
				if runErr != nil || code != expected {
					t.Fatalf("Run = (%d, %v), want (%d, nil)", code, runErr, expected)
				}
			case <-ctx.Done():
				t.Fatal("supervisor did not finish cancellation")
			}
			if handle != 0 {
				status, err := windows.WaitForSingleObject(handle, 0)
				if err != nil || status != windows.WAIT_OBJECT_0 {
					t.Fatalf("descendant not exited when Run returned: status=%d, err=%v", status, err)
				}
			}
			listener, err := net.Listen("tcp4", fields[1])
			if err != nil {
				t.Fatalf("descendant retained its port after Run: %v", err)
			}
			_ = listener.Close()
		})
	}
}

func TestWindowsDescendantHelper(t *testing.T) {
	if os.Getenv("LATTICEBUILD_GRACEPROC_TEST_HELPER") != "1" {
		return
	}
	// Force the supervisor to escalate when console signals are available.
	// Non-console hosts exercise its immediate job-termination fallback.
	signal.Ignore(os.Interrupt)
	mode := os.Args[len(os.Args)-1]
	if mode == "listen" {
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			os.Exit(98)
		}
		fmt.Printf("%d %s\n", os.Getpid(), listener.Addr())
		_, _ = listener.Accept()
		os.Exit(99)
	}
	executable, err := os.Executable()
	if err != nil {
		os.Exit(98)
	}
	child := exec.Command(executable, "-test.run=^TestWindowsDescendantHelper$", "--", "listen")
	output, err := child.StdoutPipe()
	if err != nil {
		os.Exit(98)
	}
	if err := child.Start(); err != nil {
		os.Exit(98)
	}
	reader := bufio.NewReader(output)
	line, err := reader.ReadString('\n')
	if err != nil {
		os.Exit(98)
	}
	fmt.Print(line)
	if mode == "cancel" {
		_, _ = io.Copy(io.Discard, reader)
	}
	// The descendant keeps listening after an ordinary parent exit.
	os.Exit(0)
}
