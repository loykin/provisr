package factory

import (
	"errors"
	"strings"

	"github.com/loykin/provisr/internal/store"
	pg "github.com/loykin/provisr/internal/store/postgres"
	sq "github.com/loykin/provisr/internal/store/sqlite"
)

// NewFromDSN selects a store implementation based on DSN.
// Supported:
//   - sqlite:  "sqlite:///<path>" or bare filepath (treated as sqlite)
//   - postgres: DSN starting with "postgres://" or "postgresql://"
func NewFromDSN(dsn string) (store.Store, error) {
	d := strings.TrimSpace(dsn)
	ld := strings.ToLower(d)
	if ld == "" {
		return nil, errors.New("empty DSN")
	}
	if strings.HasPrefix(ld, "postgres://") || strings.HasPrefix(ld, "postgresql://") {
		return pg.New(d)
	}
	if strings.HasPrefix(ld, "sqlite://") {
		path := strings.TrimPrefix(d, "sqlite://")
		return sq.New(path)
	}
	// default to sqlite path
	return sq.New(d)
}
