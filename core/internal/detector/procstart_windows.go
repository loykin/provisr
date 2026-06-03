//go:build windows

package detector

import (
	"syscall"
	"unsafe"
)

// getProcStartUnix returns the process creation time as Unix seconds on Windows using WinAPI.
// Returns 0 on error.
func getProcStartUnix(pid int) int64 {
	if pid <= 0 {
		return 0
	}
	h, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return 0
	}
	defer syscall.CloseHandle(h)

	var creation, exit, kernel, user syscall.Filetime
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetProcessTimes")
	ret, _, _ := proc.Call(uintptr(h), uintptr(unsafe.Pointer(&creation)), uintptr(unsafe.Pointer(&exit)), uintptr(unsafe.Pointer(&kernel)), uintptr(unsafe.Pointer(&user)))
	if ret == 0 {
		return 0
	}
	const ticksPerSecond = 10000000
	ft := (uint64(creation.HighDateTime) << 32) | uint64(creation.LowDateTime)
	secs := int64(ft / ticksPerSecond)
	const epochDiff = 11644473600
	return secs - epochDiff
}
