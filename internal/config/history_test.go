package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadHistoryFromTOML(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "h.toml")
	data := `
[history]
# enable exporting
enabled = true
in_store = false
opensearch_url = "http://localhost:9200"
opensearch_index = "provisr-history"
clickhouse_url = "http://localhost:8123"
clickhouse_table = "default.provisr_history"
`
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	hc, err := LoadHistoryFromTOML(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if hc == nil || !hc.Enabled || hc.InStore == nil || *hc.InStore != false {
		t.Fatalf("unexpected hc: %#v", hc)
	}
	if hc.OpenSearchURL == "" || hc.OpenSearchIndex == "" || hc.ClickHouseURL == "" || hc.ClickHouseTable == "" {
		t.Fatalf("missing fields in hc: %#v", hc)
	}
}
