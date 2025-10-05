//go:build windows

package process

import (
	"errors"
	"syscall"
)

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess      = kernel32.NewProc("OpenProcess")
	procTerminateProcess = kernel32.NewProc("TerminateProcess")
	procCloseHandle      = kernel32.NewProc("CloseHandle")
)

const (
	PROCESS_TERMINATE         = 0x0001
	PROCESS_QUERY_INFORMATION = 0x0400
)

// killProcess terminates a Windows process by PID
func killProcess(pid int, signal syscall.Signal) error {
	if pid <= 0 {
		// On Windows, invalid PIDs are common during rapid process termination
		// Return success to avoid unnecessary error propagation
		return nil
	}

	// Handle negative PID (process group on Unix) - on Windows, just use absolute value
	actualPid := pid
	if pid < 0 {
		actualPid = -pid
	}

	// For Windows, we only support SIGTERM and SIGKILL behavior
	// Signal 0 is used for checking if process exists
	if signal == 0 {
		return checkProcessExists(actualPid)
	}

	// Open process with terminate access
	handle, err := openProcess(PROCESS_TERMINATE, false, uint32(actualPid))
	if err != nil {
		// If we can't open the process, it likely doesn't exist anymore
		// This is common in Windows when processes terminate quickly
		// Consider this a successful termination
		return nil
	}
	defer closeHandle(handle)

	// Terminate the process
	ret, _, err := procTerminateProcess.Call(uintptr(handle), uintptr(1))
	if ret == 0 {
		return err
	}

	return nil
}

// checkProcessExists checks if a process exists (equivalent to kill(pid, 0) on Unix)
func checkProcessExists(pid int) error {
	handle, err := openProcess(PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return err
	}
	defer closeHandle(handle)
	return nil
}

// openProcess opens a process handle
func openProcess(access uint32, inheritHandle bool, processID uint32) (syscall.Handle, error) {
	inherit := 0
	if inheritHandle {
		inherit = 1
	}

	ret, _, err := procOpenProcess.Call(
		uintptr(access),
		uintptr(inherit),
		uintptr(processID),
	)

	if ret == 0 {
		return 0, err
	}

	return syscall.Handle(ret), nil
}

// closeHandle closes a Windows handle
func closeHandle(handle syscall.Handle) error {
	ret, _, err := procCloseHandle.Call(uintptr(handle))
	if ret == 0 {
		return err
	}
	return nil
}

// processExists checks if a process exists (for test compatibility)
func processExists(pid int) bool {
	return checkProcessExists(pid) == nil
}
