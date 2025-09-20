package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/loykin/provisr/internal/history"
)

// Sink sends events to OpenSearch via HTTP.
// It constructs URL as: baseURL + "/" + index + "/_doc" and POSTs JSON body.
type Sink struct {
	client  *http.Client
	baseURL string
	index   string
}

func New(baseURL, index string) *Sink {
	c := &http.Client{Timeout: 5 * time.Second}
	return &Sink{client: c, baseURL: strings.TrimRight(baseURL, "/"), index: index}
}

func (s *Sink) Send(ctx context.Context, e history.Event) error {
	u := fmt.Sprintf("%s/%s/_doc", s.baseURL, s.index)
	b, _ := json.Marshal(e)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("opensearch sink status %d", resp.StatusCode)
	}
	return nil
}
