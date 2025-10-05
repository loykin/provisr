//go:build windows

package server

import "path/filepath"

// getPlatformAbsPath returns a valid absolute path for Windows systems
func getPlatformAbsPath() string {
	return filepath.Join("C:\\", "tmp", "x")
}
