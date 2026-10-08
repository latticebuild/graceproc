package api_test

import (
	"fmt"
	"github.com/latticebuild/graceproc"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestExampleChild(t *testing.T) {
	if os.Getenv("GRACEPROC_EXAMPLE_CHILD") == "1" {
		os.Exit(0)
	}
}

func command() *exec.Cmd {
	executable, err := os.Executable()
	if err != nil {
		panic(err)
	}
	child := exec.Command(executable, "-test.run=^TestExampleChild$")
	child.Env = append(os.Environ(), "GRACEPROC_EXAMPLE_CHILD=1")
	return child
}

func ExampleRun() {
	code, err := graceproc.Run(command(), nil)
	fmt.Println(code, err)
	// Output: 0 <nil>
}

func ExampleRunWithGrace() {
	code, err := graceproc.RunWithGrace(command(), nil, time.Second)
	fmt.Println(code, err)
	// Output: 0 <nil>
}

func ExampleSignalCode() {
	fmt.Println(graceproc.SignalCode(os.Interrupt))
	// Output: 130
}

func ExampleSignals() {
	fmt.Println(graceproc.Signals()[0] == os.Interrupt)
	// Output: true
}
