package history

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ClickHouseSink sends events to ClickHouse via the HTTP interface.
// It uses JSONEachRow format: query=INSERT INTO <table> FORMAT JSONEachRow
// and sends a single JSON line per event.
type ClickHouseSink struct {
	client *http.Client
	base   string // base HTTP endpoint, e.g., http://localhost:8123
	table  string
}

func NewClickHouseSink(baseURL, table string) *ClickHouseSink {
	c := &http.Client{Timeout: 5 * time.Second}
	return &ClickHouseSink{client: c, base: strings.TrimRight(baseURL, "/"), table: table}
}

func (s *ClickHouseSink) Send(ctx context.Context, e Event) error {
	// Build URL with query
	u, _ := url.Parse(s.base)
	q := u.Query()
	q.Set("query", fmt.Sprintf("INSERT INTO %s FORMAT JSONEachRow", s.table))
	u.RawQuery = q.Encode()
	line, _ := json.Marshal(e)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(append(line, '\n')))
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("clickhouse sink status %d", resp.StatusCode)
	}
	return nil
}
