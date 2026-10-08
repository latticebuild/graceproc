package graceproc

import (
	"fmt"

	"golang.org/x/sys/unix"
)

func groupRunning(group int) (bool, error) {
	entries, err := unix.SysctlKinfoProcSlice("kern.proc.pgrp", group)
	if err != nil {
		return false, err
	}
	if len(entries) > 100000 {
		return false, fmt.Errorf("process group inventory exceeds 100000 entries")
	}
	// Darwin killpg filters SZOMB (sys/proc.h) entries and returns EPERM when
	// only zombies remain. These exited processes cannot retain runtime inputs
	// or ports, even while their parent has not reaped them.
	const zombie = 5
	for _, entry := range entries {
		if entry.Proc.P_stat != zombie {
			return true, nil
		}
	}
	return false, nil
}
