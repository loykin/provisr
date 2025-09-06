package main

import (
	"os/exec"
	"testing"
)

func TestServeNonBlockingFlagStartsServer(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "serve", "--api-listen", ":0", "--non-blocking")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("serve non-blocking failed: %v out=%s", err, out)
	}
}
