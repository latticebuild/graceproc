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
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A Linux thread-group leader may exit while sibling threads still own sockets.
// init runs on the startup thread, so this helper can reproduce that state
// without a C compiler or a scheduler-dependent race during process termination.
func init() {
	if os.Getenv("LATTICEBUILD_PROCESS_TEST_HELPER") == "exited-process" {
		os.Exit(0)
	}
	if os.Getenv("LATTICEBUILD_PROCESS_TEST_HELPER") != "exited-leader" {
		return
	}
	runtime.LockOSThread()
	if syscall.Gettid() != os.Getpid() {
		os.Exit(98)
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		os.Exit(98)
	}
	ready := make(chan struct{})
	go func() {
		fmt.Println(listener.Addr().String())
		close(ready)
		for {
			connection, err := listener.Accept()
			if err != nil {
				os.Exit(98)
			}
			_, _ = io.WriteString(connection, "alive")
			_ = connection.Close()
		}
	}()
	<-ready
	// Entering a syscall releases this thread's Go execution resources. SYS_EXIT
	// exits only this OS thread; process-wide exit would also stop the listener.
	_, _, _ = syscall.Syscall(syscall.SYS_EXIT, 0, 0, 0)
	os.Exit(98)
}

func TestCleanupAcceptsExitedUnreapedProcessGroup(t *testing.T) {
	if running, err := groupRunning(syscall.Getpgrp()); err != nil || !running {
		t.Fatalf("live group reported running=%v, err=%v", running, err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	command := exec.Command(executable)
	command.Env = append(os.Environ(), "LATTICEBUILD_PROCESS_TEST_HELPER=exited-process")
	configureProcess(command)
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		if err := command.Wait(); err != nil {
			t.Errorf("reap exited child: %v", err)
		}
	})
	// Observe the kernel's exited state without calling Wait, retaining the
	// unreaped one-thread group until both running and cleanup have been checked.
	poll := time.NewTicker(time.Millisecond)
	defer poll.Stop()
	for {
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", command.Process.Pid))
		if err != nil {
			t.Fatal(err)
		}
		fields := strings.Fields(string(data[strings.LastIndexByte(string(data), ')')+1:]))
		if len(fields) >= 18 && fields[0] == "Z" && fields[17] == "1" {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("child did not finish exiting: %s", data)
		case <-poll.C:
		}
	}
	if running, err := groupRunning(command.Process.Pid); err != nil || running {
		t.Fatalf("exited group reported running=%v, err=%v", running, err)
	}
	_, cleanup, err := controlProcess(command.Process)
	if err != nil {
		t.Fatal(err)
	}
	if err := cleanup(); err != nil {
		t.Fatalf("exited group cleanup failed: %v", err)
	}
}

func TestGroupRunningWithExitedLeader(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable)
	command.Env = append(os.Environ(), "LATTICEBUILD_PROCESS_TEST_HELPER=exited-leader")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
			t.Errorf("kill helper: %v", err)
		}
		var exit *exec.ExitError
		if err := command.Wait(); err != nil && !errors.As(err, &exit) {
			t.Errorf("join helper: %v", err)
		}
	})
	address, err := bufio.NewReader(output).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	poll := time.NewTicker(time.Millisecond)
	defer poll.Stop()
	for {
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", command.Process.Pid))
		if err != nil {
			t.Fatal(err)
		}
		fields := strings.Fields(string(data[strings.LastIndexByte(string(data), ')')+1:]))
		if len(fields) >= 18 && fields[0] == "Z" {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("leader did not exit: %s", data)
		case <-poll.C:
		}
	}
	connection, err := net.DialTimeout("tcp4", strings.TrimSpace(address), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := connection.Close(); err != nil {
			t.Errorf("close connection: %v", err)
		}
	}()
	if err := connection.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	alive := make([]byte, 5)
	if _, err := io.ReadFull(connection, alive); err != nil {
		t.Fatal(err)
	}
	if string(alive) != "alive" {
		t.Fatalf("listener response = %q", alive)
	}
	running, err := groupRunning(command.Process.Pid)
	if err != nil || !running {
		t.Fatalf("group with a live listener = (%v, %v), want (true, nil)", running, err)
	}
}
