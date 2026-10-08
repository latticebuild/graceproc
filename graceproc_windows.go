package graceproc

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Signals are the supported cancellation signals on this platform.
func Signals() []os.Signal { return []os.Signal{os.Interrupt, syscall.SIGTERM} }

// SignalCode returns the conventional exit status for cancellation.
func SignalCode(signal os.Signal) int { return 128 + int(signal.(syscall.Signal)) }

func processExitCode(err *exec.ExitError) int { return err.ExitCode() }

func configureProcess(cmd *exec.Cmd) {
	// The primary thread cannot run (or spawn outside the job) until assigned.
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_SUSPENDED
}

// A private job ensures cancellation and normal parent exit cannot leave
// grandchildren running. Windows supports nested jobs on every supported host.
func controlProcess(process *os.Process) (func(os.Signal), func() error, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, nil, os.NewSyscallError("CreateJobObjectW", err)
	}
	port, err := windows.CreateIoCompletionPort(windows.InvalidHandle, 0, 0, 1)
	if err != nil {
		_ = windows.CloseHandle(job)
		return nil, nil, os.NewSyscallError("CreateIoCompletionPort", err)
	}
	var once sync.Once
	var result error
	assigned := false
	cleanup := func() error {
		once.Do(func() {
			defer func() { _ = windows.CloseHandle(job) }()
			defer func() { _ = windows.CloseHandle(port) }()
			if !assigned {
				return
			}
			deadline := time.Now().Add(3 * time.Second)
			if err := windows.TerminateJobObject(job, 1); err != nil {
				result = os.NewSyscallError("TerminateJobObject", err)
				return
			}
			result = waitJobExit(job, port, deadline)
		})
		return result
	}
	// x/sys does not define JOBOBJECT_ASSOCIATE_COMPLETION_PORT. Associate it
	// while the job is empty, before its suspended parent is assigned.
	association := struct {
		Key  uintptr
		Port windows.Handle
	}{uintptr(job), port}
	var pinner runtime.Pinner
	pinner.Pin(&association)
	_, err = windows.SetInformationJobObject(job, windows.JobObjectAssociateCompletionPortInformation, uintptr(unsafe.Pointer(&association)), uint32(unsafe.Sizeof(association)))
	pinner.Unpin()
	if err != nil {
		_ = cleanup()
		return nil, nil, os.NewSyscallError("SetInformationJobObject", err)
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	// The x/sys wrapper takes a uintptr, so keep the buffer pinned.
	pinner.Pin(&limits)
	_, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits)))
	pinner.Unpin()
	if err != nil {
		_ = cleanup()
		return nil, nil, os.NewSyscallError("SetInformationJobObject", err)
	}
	handle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(process.Pid))
	if err != nil {
		_ = cleanup()
		return nil, nil, err
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	if err := windows.AssignProcessToJobObject(job, handle); err != nil {
		_ = cleanup()
		return nil, nil, os.NewSyscallError("AssignProcessToJobObject", err)
	}
	assigned = true
	if err := resumeProcess(uint32(process.Pid)); err != nil {
		_ = cleanup()
		return nil, nil, err
	}
	forward := func(os.Signal) {
		// CTRL_BREAK_EVENT reaches the new group, including Node descendants.
		if err := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(process.Pid)); err != nil {
			_ = cleanup()
		}
	}
	return forward, cleanup, nil
}

var getQueuedCompletionStatus = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetQueuedCompletionStatus")

func waitJobExit(job, port windows.Handle, deadline time.Time) error {
	// Job messages carry a numeric process ID in the OVERLAPPED slot. Reading
	// into uintptr avoids presenting that value to Go's GC as a pointer.
	var message uint32
	var key, payload uintptr
	var pinner runtime.Pinner
	pinner.Pin(&message)
	pinner.Pin(&key)
	pinner.Pin(&payload)
	defer pinner.Unpin()
	for range 1_000_000 {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("process job did not finish cleanup")
		}
		milliseconds := uint32((remaining + time.Millisecond - 1) / time.Millisecond)
		ok, _, err := getQueuedCompletionStatus.Call(uintptr(port), uintptr(unsafe.Pointer(&message)), uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&payload)), uintptr(milliseconds))
		if ok == 0 {
			if err == syscall.Errno(windows.WAIT_TIMEOUT) {
				return fmt.Errorf("process job did not finish cleanup")
			}
			return os.NewSyscallError("GetQueuedCompletionStatus", err)
		}
		// JOB_OBJECT_MSG_ACTIVE_PROCESS_ZERO is absent from x/sys/windows.
		if key == uintptr(job) && message == 4 {
			return nil
		}
	}
	return fmt.Errorf("process job notifications exceed work limit")
}

// os/exec closes CreateProcess's primary thread handle. Find that thread in a
// bounded snapshot; CREATE_SUSPENDED means the child has not created more threads.
func resumeProcess(pid uint32) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer func() { _ = windows.CloseHandle(snapshot) }()
	entry := windows.ThreadEntry32{}
	entry.Size = uint32(unsafe.Sizeof(entry))
	step := windows.Thread32First
	for range 1_000_000 {
		if err := step(snapshot, &entry); err != nil {
			return os.NewSyscallError("find suspended process thread", err)
		}
		if entry.Size >= uint32(unsafe.Offsetof(entry.OwnerProcessID)+unsafe.Sizeof(entry.OwnerProcessID)) && entry.OwnerProcessID == pid {
			thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
			if err != nil {
				return os.NewSyscallError("OpenThread", err)
			}
			defer func() { _ = windows.CloseHandle(thread) }()
			if count, err := windows.ResumeThread(thread); err != nil {
				return os.NewSyscallError("ResumeThread", err)
			} else if count != 1 {
				return fmt.Errorf("unexpected primary thread suspension count %d", count)
			}
			return nil
		}
		entry.Size = uint32(unsafe.Sizeof(entry))
		step = windows.Thread32Next
	}
	return fmt.Errorf("thread snapshot exceeds one million entries")
}
