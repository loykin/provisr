package main

import (
	"os/exec"
	"testing"
)

func TestServeHelpCommand(t *testing.T) {
	// Test that serve command help works after flag simplification
	cmd := exec.Command("go", "run", ".", "serve", "--help")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("serve --help failed: %v out=%s", err, out)
	}
}
