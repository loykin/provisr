package server

import (
	"encoding/json"
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

func writeJSON(c *gin.Context, code int, v any) {
	c.Header("Content-Type", "application/json")
	c.Status(code)
	_ = json.NewEncoder(c.Writer).Encode(v)
}
