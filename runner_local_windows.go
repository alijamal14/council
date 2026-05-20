//go:build windows

package main

import (
	"context"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32                   = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObject        = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObject = kernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")
)

const (
	JobObjectExtendedLimitInformation = 9
	JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE = 0x00002000
	// PROCESS_SET_QUOTA and PROCESS_TERMINATE are needed for AssignProcessToJobObject
	PROCESS_SET_QUOTA = 0x0100
	PROCESS_TERMINATE = 0x0001
)

type JOBOBJECT_BASIC_LIMIT_INFORMATION struct {
	CheckSum                       int64
	LimitFlags                     uint32
	_                              [4]byte // Padding
}

type JOBOBJECT_EXTENDED_LIMIT_INFORMATION struct {
	BasicLimitInformation JOBOBJECT_BASIC_LIMIT_INFORMATION
	_                     [112]byte // Remaining fields and padding
}

// Run executes commands on the local machine (Windows version with Job Objects)
func (r *LocalRunner) Run(ctx context.Context, name string, args []string, stdin io.Reader) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = stdin
	cmd.Env = agentSubprocessEnv() // auth + TERM/Git fixes for non-interactive CLIs

	// Create a job object to manage the process tree
	hJob, _, _ := procCreateJobObject.Call(0, 0)
	if hJob != 0 {
		defer syscall.CloseHandle(syscall.Handle(hJob))
		
		info := JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
		info.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
		
		procSetInformationJobObject.Call(
			hJob,
			JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)),
			uintptr(unsafe.Sizeof(info)),
		)
	}

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Start()
	if err != nil {
		return nil, nil, err
	}

	// Assign the process to the job object
	if hJob != 0 && cmd.Process != nil {
		// OpenProcess is needed to get a handle with proper access for AssignProcessToJobObject
		hProcess, err := syscall.OpenProcess(PROCESS_SET_QUOTA|PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
		if err == nil {
			defer syscall.CloseHandle(hProcess)
			procAssignProcessToJobObject.Call(hJob, uintptr(hProcess))
		}
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		// On Windows, CommandContext should kill the process on cancellation.
		// Job Objects ensure children are killed too when hJob is closed (deferred above).
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		// Wait for the process to actually exit to avoid resource leaks
		<-done
		return []byte(stdout.String()), []byte(stderr.String()), ctx.Err()
	case err := <-done:
		return []byte(stdout.String()), []byte(stderr.String()), err
	}
}
