package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/loykin/provisr"
)

// Example demonstrating how to enable the persistent store (sqlite or postgres)
// when embedding provisr. You can run it as-is; by default it uses a local
// SQLite database file at ./provisr_store.db. To use PostgreSQL, set STORE_DSN.
//
//	STORE_DSN=postgres://user:pass@localhost:5432/dbname?sslmode=disable go run .
//
// Or for SQLite in-memory (ephemeral):
//
//	STORE_DSN=sqlite://:memory: go run .
func main() {
	dsn := os.Getenv("STORE_DSN")
	if dsn == "" {
		// Default to a local SQLite file for persistence across runs
		dsn = "sqlite://./provisr_store.db"
	}

	mgr := provisr.New()

	// Start a short-lived demo process
	spec := provisr.Spec{
		Name:          "store-demo",
		Command:       "sh -c 'echo running; sleep 0.3'",
		StartDuration: 0,
		AutoRestart:   false,
	}
	if err := mgr.Start(spec); err != nil {
		panic(err)
	}

	// Observe status shortly after start
	time.Sleep(100 * time.Millisecond)
	st, _ := mgr.Status("store-demo")
	b, _ := json.MarshalIndent(st, "", "  ")
	fmt.Println("Status after start:")
	fmt.Println(string(b))

	// Wait for the process to finish naturally, then reconcile and show final state
	time.Sleep(500 * time.Millisecond)

	st2, _ := mgr.Status("store-demo")
	b2, _ := json.MarshalIndent(st2, "", "  ")
	fmt.Println("Status after exit + reconcile:")
	fmt.Println(string(b2))

	fmt.Println()
	fmt.Println("Store DSN:", dsn)
	fmt.Println("Records are persisted via the configured store; rerun this example to accumulate history.")
}
