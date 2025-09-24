//go:build windows

package process

import (
	"syscall"
	"unsafe"
)

// Minimal Windows implementation to get process creation time.
// Returns 0 on error.
func getProcStartUnix(pid int) int64 {
	if pid <= 0 {
		return 0
	}
	// OpenProcess with QUERY_INFORMATION
	h, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return 0
	}
	defer syscall.CloseHandle(h)

	var creation, exit, kernel, user syscall.Filetime
	// GetProcessTimes from kernel32
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetProcessTimes")
	ret, _, _ := proc.Call(uintptr(h), uintptr(unsafe.Pointer(&creation)), uintptr(unsafe.Pointer(&exit)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)))
	if ret == 0 {
		return 0
	}
	// FILETIME is in 100-ns intervals since Jan 1, 1601 (UTC)
	// Convert to Unix seconds
	const ticksPerSecond = 10000000
	// Combine High/Low into uint64
	ft := (uint64(creation.HighDateTime) << 32) | uint64(creation.LowDateTime)
	// Convert to seconds since 1601-01-01
	secs := int64(ft / ticksPerSecond)
	// Difference between 1601-01-01 and 1970-01-01 in seconds
	const epochDiff = 11644473600
	return secs - epochDiff
}
