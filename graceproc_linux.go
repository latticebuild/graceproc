package graceproc

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func groupRunning(group int) (bool, error) {
	if err := syscall.Kill(-group, 0); err != nil {
		return false, absentGroup(err)
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false, err
	}
	if len(entries) > 100_000 {
		return false, fmt.Errorf("process inventory exceeds 100000 entries")
	}
	for _, entry := range entries {
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		data, err := os.ReadFile("/proc/" + entry.Name() + "/stat")
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH) {
			continue
		}
		if err != nil {
			return false, err
		}
		// comm may contain spaces and parentheses. State and pgrp follow its
		// last closing parenthesis. An exited thread-group leader can still
		// have live sibling threads retaining sockets, so also read num_threads.
		end := strings.LastIndexByte(string(data), ')')
		if end < 0 {
			return false, fmt.Errorf("incomplete process status")
		}
		fields := strings.Fields(string(data[end+1:]))
		if len(fields) < 18 {
			return false, fmt.Errorf("incomplete process status")
		}
		if fields[2] != strconv.Itoa(group) {
			continue
		}
		threads, err := strconv.Atoi(fields[17])
		if err != nil || threads <= 0 {
			return false, fmt.Errorf("invalid process thread count %q", fields[17])
		}
		if (fields[0] != "Z" && fields[0] != "X") || threads > 1 {
			return true, nil
		}
	}
	return false, nil
}

func absentGroup(err error) error {
	if err == syscall.ESRCH {
		return nil
	}
	return err
}
