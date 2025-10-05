package server

import (
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSanitizeBase(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"/", ""},
		{"api", "/api"},
		{"/api", "/api"},
		{"/api/", "/api"},
		{" api ", "/api"},
	}
	for _, c := range cases {
		if got := sanitizeBase(c.in); got != c.want {
			t.Fatalf("sanitizeBase(%q)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestIsSafeName(t *testing.T) {
	valid := []string{"a", "A1._-", "name.1-2_3"}
	invalid := []string{"", "..", "a..b", "a/b", `a\\b`, "hello*", "unicode한글"}
	for _, s := range valid {
		if !isSafeName(s) {
			t.Fatalf("expected valid name %q", s)
		}
	}
	for _, s := range invalid {
		if isSafeName(s) {
			t.Fatalf("expected invalid name %q", s)
		}
	}
}

func TestIsSafeAbsPath(t *testing.T) {
	// empty is allowed
	if !isSafeAbsPath("") {
		t.Fatalf("empty should be allowed")
	}
	abs := getPlatformAbsPath()
	if !isSafeAbsPath(abs) {
		t.Fatalf("abs clean path should be allowed: %s", abs)
	}
	// not absolute
	if isSafeAbsPath("tmp/x") {
		t.Fatalf("relative path should be rejected")
	}
	// with traversal (construct without cleaning)
	sep := string(filepath.Separator)
	bad := sep + "tmp" + sep + ".." + sep + "etc"
	if isSafeAbsPath(bad) {
		t.Fatalf("path with traversal should be rejected: %s", bad)
	}
}

func TestWriteJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", func(c *gin.Context) { writeJSON(c, 201, map[string]any{"a": 1}) })
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest("GET", "/x", nil))
	if rec.Code != 201 {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type: %s", ct)
	}
}
