package server

import (
	"path/filepath"
	"strings"
	"testing"
)

// FuzzIsSafeName tests the name validation function with various inputs
func FuzzIsSafeName(f *testing.F) {
	// Seed with various name patterns
	f.Add("valid-name_123")
	f.Add("")
	f.Add("..")
	f.Add("../etc/passwd")
	f.Add("name/with/slash")
	f.Add("name\\with\\backslash")
	f.Add("valid.name")
	f.Add("name_with-special.chars123")
	f.Add("...dotted")
	f.Add("unicode한글name") // Unicode
	f.Add("name\x00null")
	f.Add("name\nnewline")
	f.Add("name\ttab")

	f.Fuzz(func(t *testing.T, name string) {
		if len(name) > 200 {
			t.Skip("name too long")
		}

		// Test isSafeName - should not panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("isSafeName panicked with input %q: %v", name, r)
				}
			}()

			result := isSafeName(name)

			// Basic validation of result consistency
			if name == "" {
				if result {
					t.Error("empty name should not be safe")
				}
			}

			// Names containing ".." should not be safe
			if strings.Contains(name, "..") {
				if result {
					t.Errorf("name with .. should not be safe: %q", name)
				}
			}

			// Names with path separators should not be safe
			if strings.ContainsAny(name, "/\\") {
				if result {
					t.Errorf("name with path separators should not be safe: %q", name)
				}
			}

			// Test consistency - calling multiple times should give same result
			result2 := isSafeName(name)
			if result != result2 {
				t.Errorf("isSafeName inconsistent for %q: %v vs %v", name, result, result2)
			}
		}()
	})
}

// FuzzIsSafeAbsPath tests the absolute path validation function
func FuzzIsSafeAbsPath(f *testing.F) {
	// Seed with path patterns
	f.Add("/safe/absolute/path")
	f.Add("")
	f.Add("/")
	f.Add("relative/path")
	f.Add("/path/../traversal")
	f.Add("/path/./current")
	f.Add("/path//double/slash")
	f.Add("C:\\Windows\\Path") // Windows path
	f.Add("/path/with spaces")
	f.Add("/path\x00null")
	f.Add("/path\nnewline")

	f.Fuzz(func(t *testing.T, path string) {
		if len(path) > 500 {
			t.Skip("path too long")
		}

		// Test isSafeAbsPath - should not panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("isSafeAbsPath panicked with input %q: %v", path, r)
				}
			}()

			result := isSafeAbsPath(path)

			// Basic validation
			if path == "" {
				if !result {
					t.Error("empty path should be safe (allowed)")
				}
			}

			// Non-absolute paths should not be safe (except empty)
			if path != "" && !filepath.IsAbs(path) {
				if result {
					t.Errorf("relative path should not be safe: %q", path)
				}
			}

			// Paths that change when cleaned should not be safe
			if path != "" {
				clean := filepath.Clean(path)
				sep := string(filepath.Separator)
				trimmed := strings.TrimRight(path, sep)
				if trimmed == "" {
					trimmed = path
				}

				pathChanged := !(clean == path || clean == trimmed)
				if pathChanged && result {
					t.Errorf("path that changes when cleaned should not be safe: %q -> %q", path, clean)
				}
			}

			// Test consistency
			result2 := isSafeAbsPath(path)
			if result != result2 {
				t.Errorf("isSafeAbsPath inconsistent for %q: %v vs %v", path, result, result2)
			}
		}()
	})
}

// FuzzSanitizeBase tests base path sanitization
func FuzzSanitizeBase(f *testing.F) {
	// Seed with base path patterns
	f.Add("")
	f.Add("/")
	f.Add("/api")
	f.Add("/api/")
	f.Add("api")
	f.Add("  /api/v1/  ")
	f.Add("//multiple//slashes//")
	f.Add("/path/../traversal")
	f.Add("/path\x00null")

	f.Fuzz(func(t *testing.T, basePath string) {
		if len(basePath) > 200 {
			t.Skip("base path too long")
		}

		// Test sanitizeBase - should not panic
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("sanitizeBase panicked with input %q: %v", basePath, r)
				}
			}()

			result := sanitizeBase(basePath)

			// Validate result properties
			if result != "" {
				// Non-empty results should start with /
				if !strings.HasPrefix(result, "/") {
					t.Errorf("sanitized base should start with /: %q -> %q", basePath, result)
				}

				// Should not end with / (unless it's just "/")
				if result != "/" && strings.HasSuffix(result, "/") {
					t.Errorf("sanitized base should not end with /: %q -> %q", basePath, result)
				}
			}

			// Empty or "/" inputs should result in ""
			trimmed := strings.TrimSpace(basePath)
			if trimmed == "" || trimmed == "/" {
				if result != "" {
					t.Errorf("empty or root base should result in empty: %q -> %q", basePath, result)
				}
			}

			// Test consistency
			result2 := sanitizeBase(basePath)
			if result != result2 {
				t.Errorf("sanitizeBase inconsistent for %q: %q vs %q", basePath, result, result2)
			}
		}()
	})
}

// FuzzCombinedValidation tests the interaction of validation functions
func FuzzCombinedValidation(f *testing.F) {
	// Test combinations of name and path validation
	addPlatformSpecificSeeds(f)
	f.Add("../bad", "")
	f.Add("good", "../bad/path")
	f.Add("", "")

	f.Fuzz(func(t *testing.T, name, workDir string) {
		if len(name) > 100 || len(workDir) > 200 {
			t.Skip("inputs too long")
		}

		// Test that validation functions work together without conflicts
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("combined validation panicked: %v", r)
				}
			}()

			nameOK := isSafeName(name)
			pathOK := isSafeAbsPath(workDir)

			// Basic sanity: if both are safe, they should remain safe when used together
			// (This is mainly testing that the validation logic is consistent)
			if nameOK && pathOK {
				// No specific assertions needed - just ensuring no crashes
				t.Logf("Both name %q and path %q are safe", name, workDir)
			}

			if !nameOK {
				t.Logf("Name %q is not safe", name)
			}

			if !pathOK && workDir != "" {
				t.Logf("Path %q is not safe", workDir)
			}
		}()
	})
}
