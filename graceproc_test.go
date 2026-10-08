package graceproc

import (
	"os"
	"os/exec"
	"strconv"
	"testing"
)

func TestExitStatus(t *testing.T) {
	for _, expected := range []int{0, 17} {
		t.Run(strconv.Itoa(expected), func(t *testing.T) {
			command := helperCommand(t, "exit", strconv.Itoa(expected))
			signals := make(chan os.Signal)
			close(signals)
			code, err := Run(command, signals)
			if err != nil || code != expected {
				t.Fatalf("Run = (%d, %v), want (%d, nil)", code, err, expected)
			}
		})
	}
}

func TestMissingExecutable(t *testing.T) {
	command := exec.CommandContext(t.Context(), t.TempDir()+"/does-not-exist")
	if _, err := Run(command, nil); err == nil {
		t.Fatal("missing executable was reported as a successful start")
	}
}

func TestProcessHelper(t *testing.T) {
	if os.Getenv("LATTICEBUILD_PROCESS_TEST_HELPER") != "1" {
		return
	}
	if len(os.Args) >= 3 && os.Args[len(os.Args)-2] == "exit" {
		code, err := strconv.Atoi(os.Args[len(os.Args)-1])
		if err != nil {
			os.Exit(99)
		}
		os.Exit(code)
	}
}

func helperCommand(t *testing.T, arguments ...string) *exec.Cmd {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), executable, append([]string{"-test.run=TestProcessHelper", "--"}, arguments...)...)
	command.Env = append(os.Environ(), "LATTICEBUILD_PROCESS_TEST_HELPER=1")
	return command
}
