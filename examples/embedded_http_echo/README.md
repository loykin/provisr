Embedded HTTP API with Echo

This example shows how to embed provisr's process-management HTTP API into an Echo server.

Run it:

```shell
API_BASE=/api go run ./examples/embedded_http_echo/main.go
```

It starts a demo process automatically so you can try the API immediately. Two instances named demo-1 and demo-2 are
started.

Try the API:

- Start additional processes

```shell
curl -s -X POST localhost:8080/api/start \
  -H 'Content-Type: application/json' \
  -d '{"name":"demo-extra","command":"/bin/sh -c \"while true; do echo extra; sleep 5; done\"","instances":1}'
```

- Status by base

```shell
curl -s 'localhost:8080/api/status?base=demo' | jq .
```

- Status by wildcard (new)

```shell
curl -s 'localhost:8080/api/status?wildcard=demo-*' | jq .
```

- Stop by wildcard

```shell
curl -s -X POST 'localhost:8080/api/stop?wildcard=demo-*' | jq .
```

Notes

- Change API_BASE to mount under a different base path.
- Endpoints: POST {base}/start, POST {base}/stop, GET {base}/status.
- Query selectors: name=, base=, or wildcard=. Only one may be provided per request.
