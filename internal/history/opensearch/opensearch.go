package opensearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

type Options struct {
	Migrate bool
}

// New opens a connection to OpenSearch at baseURL and returns a Sink that
// writes events to index.
func New(baseURL, index string) (*Sink, error) {
	return NewWithOptions(baseURL, index, Options{})
}

func NewWithOptions(baseURL, index string, options Options) (*Sink, error) {
	adapter := opensearchadapter.New()
	adapter.RegisterDriver("opensearch", opensearchadapter.Driver{})
	adapter.SetObserver(prometheusadapter.New("provisr_history_opensearch", nil))
	if err := adapter.Open(source, dbstore.SourceConfig{
		Driver: "opensearch",
		DSN:    strings.TrimRight(baseURL, "/"),
	}); err != nil {
		return nil, fmt.Errorf("opensearch: register pool: %w", err)
	}

	sink := &Sink{Source: opensearchadapter.NewSource(source, adapter.Executor()), adapter: adapter, index: index}
	if options.Migrate {
		err := sink.Run(context.Background(), func(ctx context.Context, c *opensearchapi.Client) error {
			resp, err := c.Indices.Exists(ctx, opensearchapi.IndicesExistsReq{Indices: []string{index}})
			if err == nil && resp.StatusCode == 200 {
				return nil
			}
			body := strings.NewReader(`{"mappings":{"properties":{"occurred_at":{"type":"date"},"record":{"properties":{"name":{"type":"keyword"},"pid":{"type":"integer"},"last_status":{"type":"keyword"}}}}}}`)
			_, err = c.Indices.Create(ctx, opensearchapi.IndicesCreateReq{Index: index, Body: body})
			return err
		})
		if err != nil {
			adapter.Close()
			return nil, fmt.Errorf("opensearch: migrate index: %w", err)
		}
	}
	return sink, nil
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
func (s *Sink) List(ctx context.Context, name string, limit, offset int) ([]corehistory.Entry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	query := map[string]any{"match_all": map[string]any{}}
	if name != "" {
		query = map[string]any{"wildcard": map[string]any{"record.name": map[string]any{
			"value": "*" + escapeWildcard(name) + "*", "case_insensitive": true,
		}}}
	}
	body, err := json.Marshal(map[string]any{
		"from":  offset,
		"size":  limit,
		"sort":  []any{map[string]any{"occurred_at": map[string]any{"order": "desc"}}},
		"query": query,
	})
	if err != nil {
		return nil, err
	}

	var entries []corehistory.Entry
	err = s.Run(ctx, func(ctx context.Context, c *opensearchapi.Client) error {
		resp, err := c.Search(ctx, &opensearchapi.SearchReq{
			Indices: []string{s.index},
			Body:    bytes.NewReader(body),
		})
		if err != nil {
			return err
		}
		entries = make([]corehistory.Entry, 0, len(resp.Hits.Hits))
		for _, hit := range resp.Hits.Hits {
			var e corehistory.Event
			if err := json.Unmarshal(hit.Source, &e); err != nil {
				return err
			}
			entries = append(entries, corehistory.Entry{
				Timestamp: e.OccurredAt,
				PID:       e.Record.PID,
				Name:      e.Record.Name,
				Status:    e.Record.LastStatus,
			})
		}
		return nil
	})
	return entries, err
}

func (s *Sink) Count(ctx context.Context, name string) (int, error) {
	query := map[string]any{"match_all": map[string]any{}}
	if name != "" {
		query = map[string]any{"wildcard": map[string]any{"record.name": map[string]any{"value": "*" + escapeWildcard(name) + "*", "case_insensitive": true}}}
	}
	body, err := json.Marshal(map[string]any{"size": 0, "query": query})
	if err != nil {
		return 0, err
	}
	var total int
	err = s.Run(ctx, func(ctx context.Context, c *opensearchapi.Client) error {
		resp, err := c.Search(ctx, &opensearchapi.SearchReq{Indices: []string{s.index}, Body: bytes.NewReader(body)})
		if err == nil {
			total = resp.Hits.Total.Value
		}
		return err
	})
	return total, err
}

func escapeWildcard(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `*`, `\*`)
	return strings.ReplaceAll(value, `?`, `\?`)
}

func (s *Sink) PruneBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	body, err := json.Marshal(map[string]any{"query": map[string]any{"range": map[string]any{"occurred_at": map[string]any{"lt": cutoff.UTC().Format(time.RFC3339Nano)}}}})
	if err != nil {
		return 0, err
	}
	var deleted int64
	err = s.Run(ctx, func(ctx context.Context, c *opensearchapi.Client) error {
		resp, err := c.Document.DeleteByQuery(ctx, opensearchapi.DocumentDeleteByQueryReq{
			Indices: []string{s.index}, Body: bytes.NewReader(body),
		})
		if err == nil {
			deleted = int64(resp.Deleted)
		}
		return err
	})
	return deleted, err
}

func (s *Sink) Close() error {
	s.adapter.Close()
	return nil
}

// compile-time check that Sink satisfies corehistory.Sink
var _ corehistory.Sink = (*Sink)(nil)
var _ corehistory.Reader = (*Sink)(nil)
var _ corehistory.Pruner = (*Sink)(nil)
