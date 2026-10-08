package main

import (
	"fmt"
	"github.com/latticebuild/graceproc"
	"os"
	"os/exec"
	"os/signal"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: supervise COMMAND [ARG...]")
		os.Exit(2)
	}
	command := exec.Command(os.Args[1], os.Args[2:]...)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)
	code, err := graceproc.Run(command, signals)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}
