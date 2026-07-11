package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"

	"github.com/loykin/dbstore"
	opensearchadapter "github.com/loykin/dbstore/adapters/opensearch"
	prometheusadapter "github.com/loykin/dbstore/adapters/prometheus"

	corehistory "github.com/loykin/provisr/core/history"
)

const source = "primary"

// Sink sends events to OpenSearch via dbstore, using the official
// OpenSearch Go client. Documents are created with a client-generated ID
// (OccurredAt/name/pid), since there is no dedicated auto-ID create request
// in opensearchapi — see dbstore's own opensearch_repo_test.go for the same
// pattern.
type Sink struct {
	opensearchadapter.Source
	adapter *opensearchadapter.Adapter
	index   string
}

// New opens a connection to OpenSearch at baseURL and returns a Sink that
// writes events to index.
func New(baseURL, index string) (*Sink, error) {
	adapter := opensearchadapter.New()
	adapter.RegisterDriver("opensearch", opensearchadapter.Driver{})
	adapter.SetObserver(prometheusadapter.New("provisr_history_opensearch", nil))
	if err := adapter.Open(source, dbstore.SourceConfig{
		Driver: "opensearch",
		DSN:    strings.TrimRight(baseURL, "/"),
	}); err != nil {
		return nil, fmt.Errorf("opensearch: register pool: %w", err)
	}

	return &Sink{Source: opensearchadapter.NewSource(source, adapter.Executor()), adapter: adapter, index: index}, nil
}

func (s *Sink) Send(ctx context.Context, e corehistory.Event) error {
	return s.Run(ctx, func(ctx context.Context, c *opensearchapi.Client) error {
		body, err := json.Marshal(e)
		if err != nil {
			return err
		}
		id := fmt.Sprintf("%d-%s-%d", e.OccurredAt.UnixNano(), e.Record.Name, e.Record.PID)
		_, err = c.Document.Create(ctx, opensearchapi.DocumentCreateReq{
			Index:      s.index,
			DocumentID: id,
			Body:       bytes.NewReader(body),
		})
		return err
	})
}

// List returns recent history events, newest first. If name is empty,
// events for all processes are returned. limit is capped at 500 (defaults
// to 100). Requires the index to have refreshed since the last write —
// OpenSearch search results are near-real-time, not immediately consistent.
func (s *Sink) List(ctx context.Context, name string, limit int) ([]corehistory.Event, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	query := map[string]any{"match_all": map[string]any{}}
	if name != "" {
		// term (not match) against the auto-generated .keyword sub-field:
		// "match" analyzes both sides, and since the standard analyzer
		// splits on "-", "svc-a" and "svc-b" both tokenize to ["svc", ...]
		// and a match query would OR across tokens, matching either name.
		query = map[string]any{"term": map[string]any{"record.name.keyword": name}}
	}
	body, err := json.Marshal(map[string]any{
		"size":  limit,
		"sort":  []any{map[string]any{"occurred_at": map[string]any{"order": "desc"}}},
		"query": query,
	})
	if err != nil {
		return nil, err
	}

	var events []corehistory.Event
	err = s.Run(ctx, func(ctx context.Context, c *opensearchapi.Client) error {
		resp, err := c.Search(ctx, &opensearchapi.SearchReq{
			Indices: []string{s.index},
			Body:    bytes.NewReader(body),
		})
		if err != nil {
			return err
		}
		events = make([]corehistory.Event, 0, len(resp.Hits.Hits))
		for _, hit := range resp.Hits.Hits {
			var e corehistory.Event
			if err := json.Unmarshal(hit.Source, &e); err != nil {
				return err
			}
			events = append(events, e)
		}
		return nil
	})
	return events, err
}

func (s *Sink) Close() error {
	s.adapter.Close()
	return nil
}

// compile-time check that Sink satisfies corehistory.Sink
var _ corehistory.Sink = (*Sink)(nil)
