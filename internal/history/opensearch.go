package history

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OpenSearchSink sends events to OpenSearch via HTTP.
// It constructs URL as: baseURL + "/" + index + "/_doc" and POSTs JSON body.
type OpenSearchSink struct {
	client  *http.Client
	baseURL string
	index   string
}

func NewOpenSearchSink(baseURL, index string) *OpenSearchSink {
	c := &http.Client{Timeout: 5 * time.Second}
	return &OpenSearchSink{client: c, baseURL: strings.TrimRight(baseURL, "/"), index: index}
}

func (s *OpenSearchSink) Send(ctx context.Context, e Event) error {
	u := fmt.Sprintf("%s/%s/_doc", s.baseURL, s.index)
	b, _ := json.Marshal(e)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("opensearch sink status %d", resp.StatusCode)
	}
	return nil
}
