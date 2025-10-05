//go:build !windows

package server

import "path/filepath"

// getPlatformAbsPath returns a valid absolute path for Unix systems
func getPlatformAbsPath() string {
	return filepath.Join(string(filepath.Separator), "tmp", "x")
}
