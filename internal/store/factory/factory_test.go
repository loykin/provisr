package factory

import "testing"

func TestFactoryDSNSelection(t *testing.T) {
	// Empty DSN -> error
	if _, err := NewFromDSN(""); err == nil {
		t.Fatalf("expected error for empty DSN")
	}
	// postgres scheme -> postgres driver object (Close immediately; no connect performed by sql.Open)
	pg, err := NewFromDSN("postgres://user@localhost/db")
	if err != nil || pg == nil {
		t.Fatalf("postgres dsn: err=%v obj=%T", err, pg)
	}
	_ = pg.Close()
	// sqlite scheme
	s1, err := NewFromDSN("sqlite://:memory:")
	if err != nil || s1 == nil {
		t.Fatalf("sqlite scheme: err=%v obj=%T", err, s1)
	}
	_ = s1.Close()
	// bare path defaults to sqlite
	s2, err := NewFromDSN(":memory:")
	if err != nil || s2 == nil {
		t.Fatalf("bare sqlite: err=%v obj=%T", err, s2)
	}
	_ = s2.Close()
}
