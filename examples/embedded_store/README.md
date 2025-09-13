# Embedded store example

This example shows how to enable the persistent store (SQLite or PostgreSQL) when embedding provisr in your own Go program.

It uses the public facade only (no internal packages), via:

- Manager.SetStoreFromDSN(dsn)
- Manager.StartReconciler / ReconcileOnce for periodic state sync and HA

## Run (SQLite, local file)

By default the example uses a local SQLite database file at `./provisr_store.db`.

```
cd examples/embedded_store
go run .
```

You should see JSON status output, and the store file will be created in the current directory. Run it multiple times to accumulate history in the store.

## Run (SQLite, in-memory)

```
STORE_DSN=sqlite://:memory: go run .
```

This does not persist between runs; useful for quick checks.

## Run (PostgreSQL)

Provide a PostgreSQL DSN (pgx stdlib compatible):

```
STORE_DSN="postgres://user:pass@localhost:5432/dbname?sslmode=disable" go run .
```

Tip: If you don't have a local Postgres instance, you can quickly start one via Docker:

```
docker run --rm -e POSTGRES_PASSWORD=test -e POSTGRES_USER=test -e POSTGRES_DB=testdb -p 5432:5432 postgres:16-alpine
# In another terminal:
STORE_DSN="postgres://test:test@localhost:5432/testdb?sslmode=disable" go run .
```

## What it does

- Configures the manager with a persistent store from DSN.
- Starts the background reconciler (2s interval) which:
  - Upserts the observed state to the store.
  - Marks lost processes as stopped in the store.
  - Enforces AutoRestart where configured.
- Starts a short-lived process, prints status, then shows the final status after exit and a reconcile.

This demonstrates how to wire the store and the reconciler in your embedded usage to achieve durable state and high availability handling.
