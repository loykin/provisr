module github.com/loykin/provisr/examples/store_basic

go 1.25.1

replace github.com/loykin/provisr => ../..

require github.com/loykin/provisr v0.0.0-00010101000000-000000000000

require (
	github.com/lib/pq v1.10.9 // indirect
	github.com/mattn/go-sqlite3 v1.14.32 // indirect
)
