package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFileAndGlobalEnv(t *testing.T) {
	dir := t.TempDir()
	dotenv := filepath.Join(dir, ".env")
	if err := os.WriteFile(dotenv, []byte("A=1\n#comment\nB=two\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}
	pairs, err := LoadEnvFile(dotenv)
	if err != nil {
		t.Fatalf("load env file: %v", err)
	}
	// order not guaranteed; validate contents by map
	m := make(map[string]string)
	for _, kv := range pairs {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				m[kv[:i]] = kv[i+1:]
				break
			}
		}
	}
	if m["A"] != "1" || m["B"] != "two" {
		t.Fatalf("unexpected pairs: %+v", m)
	}
}

func TestLoadGlobalEnv_Merge(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "cfg.toml")
	dotenv := filepath.Join(dir, ".env")
	_ = os.Setenv("OS_ONLY", "osv")
	if err := os.WriteFile(dotenv, []byte("FILE_ONLY=fv\nCHAIN=${OS_ONLY}-x\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}
	data := "" +
		"use_os_env = true\n" +
		"env_files = [\"" + dotenv + "\"]\n" +
		"env = [\"TOP=tv\"]\n"
	if err := os.WriteFile(cfgPath, []byte(data), 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	pairs, err := LoadGlobalEnv(cfgPath)
	if err != nil {
		t.Fatalf("LoadGlobalEnv: %v", err)
	}
	m := make(map[string]string)
	for _, kv := range pairs {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				m[kv[:i]] = kv[i+1:]
				break
			}
		}
	}
	// Expect OS_ONLY from OS, FILE_ONLY from file, CHAIN not expanded (expansion happens in Manager env merge), TOP overrides
	if m["OS_ONLY"] != "osv" {
		t.Fatalf("missing OS_ONLY: %v", m["OS_ONLY"])
	}
	if m["FILE_ONLY"] != "fv" {
		t.Fatalf("missing FILE_ONLY: %v", m["FILE_ONLY"])
	}
	if m["TOP"] != "tv" {
		t.Fatalf("missing TOP: %v", m["TOP"])
	}
}
