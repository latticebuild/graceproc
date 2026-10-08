//go:build !windows

package graceproc

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestDescendantsReleasePortsBeforeReturn(t *testing.T) {
	for _, mode := range []string{"exit", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 12*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, executable, "-test.run=TestDescendantHelper", "--", mode)
			command.Env = append(os.Environ(), "LATTICEBUILD_PROCESS_TEST_HELPER=1")
			// Run owns Cmd.Wait. Own the pipe separately so Wait cannot close
			// its reader before we consume the short-lived helper's address.
			output, writer, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = output.Close(); _ = writer.Close() })
			command.Stdout = writer
			if err := output.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			signals := make(chan os.Signal, 1)
			finished := make(chan error, 1)
			go func() { _, err := Run(command, signals); finished <- err }()
			joined := false
			t.Cleanup(func() {
				cancel()
				if !joined {
					select {
					case <-finished:
					case <-time.After(12 * time.Second):
						t.Error("supervisor cleanup did not finish")
					}
				}
			})
			line, err := bufio.NewReader(output).ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			if mode == "cancel" {
				signals <- syscall.SIGTERM
			}
			select {
			case err := <-finished:
				joined = true
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(12 * time.Second):
				t.Fatal("supervisor did not join descendants")
			}
			listener, err := net.Listen("tcp4", strings.TrimSpace(line))
			if err != nil {
				t.Fatalf("descendant retained its port after Run: %v", err)
			}
			_ = listener.Close()
		})
	}
}

func TestDescendantHelper(t *testing.T) {
	if os.Getenv("LATTICEBUILD_PROCESS_TEST_HELPER") != "1" {
		return
	}
	mode := os.Args[len(os.Args)-1]
	if mode == "listen" {
		signal.Ignore(syscall.SIGTERM, syscall.SIGINT)
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			os.Exit(98)
		}
		fmt.Println(listener.Addr().String())
		_, _ = listener.Accept()
		os.Exit(99)
	}
	executable, err := os.Executable()
	if err != nil {
		os.Exit(98)
	}
	child := exec.Command(executable, "-test.run=TestDescendantHelper", "--", "listen")
	output, err := child.StdoutPipe()
	if err != nil {
		os.Exit(98)
	}
	if err := child.Start(); err != nil {
		os.Exit(98)
	}
	line, err := bufio.NewReader(output).ReadString('\n')
	if err != nil {
		os.Exit(98)
	}
	fmt.Print(line)
	if mode == "cancel" {
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGTERM)
		<-signals
	}
	// Deliberately leave the listening descendant for the supervisor to join.
	os.Exit(0)
}

func TestGracefulAndRepeatedCancellation(t *testing.T) {
	for _, count := range []int{1, 2} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			command := exec.CommandContext(t.Context(), executable, "-test.run=TestCancellationHelper", "--", strconv.Itoa(count))
			command.Env = append(os.Environ(), "LATTICEBUILD_PROCESS_TEST_HELPER=1")
			stdout, err := command.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			signals := make(chan os.Signal, 2)
			finished := make(chan int, 1)
			go func() {
				code, err := Run(command, signals)
				if err != nil {
					code = -1
				}
				finished <- code
			}()
			reader := bufio.NewReader(stdout)
			line, err := reader.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			pid, err := strconv.Atoi(strings.TrimSpace(line))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
					t.Errorf("cleaning up supervised child %d: %v", pid, err)
				}
			})
			signals <- syscall.SIGTERM
			if count == 2 {
				if _, err := reader.ReadString('\n'); err != nil {
					t.Fatal(err)
				}
				signals <- syscall.SIGTERM
			}
			select {
			case code := <-finished:
				if code != 143 {
					t.Fatalf("cancellation exit = %d, want 143", code)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("supervisor did not finish cancellation")
			}
			if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
				t.Fatalf("supervised child %d survived cancellation: %v", pid, err)
			}
		})
	}
}

func TestCancellationHelper(t *testing.T) {
	if os.Getenv("LATTICEBUILD_PROCESS_TEST_HELPER") != "1" {
		return
	}
	count, err := strconv.Atoi(os.Args[len(os.Args)-1])
	if err != nil {
		os.Exit(99)
	}
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGTERM)
	fmt.Println(os.Getpid())
	for range count {
		<-signals
		fmt.Println("received")
	}
	os.Exit(0)
}

func TestCustomGraceEscalatesAndJoinsResistantProcess(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=TestDescendantHelper", "--", "listen")
	command.Env = append(os.Environ(), "LATTICEBUILD_PROCESS_TEST_HELPER=1")
	output, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = output.Close() }()
	defer func() { _ = writer.Close() }()
	command.Stdout = writer
	if err := output.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	signals := make(chan os.Signal, 1)
	finished := make(chan error, 1)
	go func() { _, err := RunWithGrace(command, signals, 100*time.Millisecond); finished <- err }()
	joined := false
	t.Cleanup(func() {
		cancel()
		if !joined {
			<-finished
		}
	})
	line, err := bufio.NewReader(output).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	signals <- syscall.SIGTERM
	select {
	case err := <-finished:
		joined = true
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("custom grace was not honored")
	}
	listener, err := net.Listen("tcp4", strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("resistant process retained port: %v", err)
	}
	_ = listener.Close()
}
