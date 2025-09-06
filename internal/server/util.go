package server

import (
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func sanitizeBase(bp string) string {
	bp = strings.TrimSpace(bp)
	if bp == "" || bp == "/" {
		return ""
	}
	if !strings.HasPrefix(bp, "/") {
		bp = "/" + bp
	}
	bp = strings.TrimRight(bp, "/")
	return bp
}

// isSafeName validates process names to avoid path traversal when used in filenames.
// Allowed characters: A-Z a-z 0-9 . _ - and no consecutive dots forming "..".
func isSafeName(s string) bool {
	if s == "" {
		return false
	}
	if strings.Contains(s, "..") {
		return false
	}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	// disallow path separators just in case (platform independent)
	if strings.ContainsAny(s, "/\\") {
		return false
	}
	return true
}

// isSafeAbsPath ensures the provided path is absolute and does not contain traversal.
// It must be already cleaned (no ".." segments). This reduces risk of uncontrolled
// user input being used in filesystem paths.
func isSafeAbsPath(p string) bool {
	if p == "" {
		return true
	}
	if !filepath.IsAbs(p) {
		return false
	}
	clean := filepath.Clean(p)
	sep := string(filepath.Separator)
	trimmed := strings.TrimRight(p, sep)
	if trimmed == "" {
		trimmed = p // keep root like "/" on Unix
	}
	// Reject if cleaning changes more than just trailing separators
	if !(clean == p || clean == trimmed) {
		return false
	}
	return true
}

func writeJSON(c *gin.Context, code int, v any) {
	c.Header("Content-Type", "application/json")
	c.Status(code)
	_ = json.NewEncoder(c.Writer).Encode(v)
}
