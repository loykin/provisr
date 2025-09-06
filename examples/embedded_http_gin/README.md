Embedded HTTP API with Gin

This example shows how to embed provisr's process-management HTTP API into your own Gin server.

Run it:

```shell
API_BASE=/api go run ./examples/embedded_http_gin/main.go
```

It starts a demo process automatically so you can try the API right away. Two instances named demo-1 and demo-2 are
started running an endless shell loop.

Try the API:

- Start additional processes

```shell
curl -s -X POST localhost:8080/api/start \
  -H 'Content-Type: application/json' \
  -d '{"name":"demo-extra","command":"/bin/sh -c \"while true; do echo extra; sleep 5; done\"","instances":1}'
```

- Status by exact name

```shell
curl -s 'localhost:8080/api/status?name=demo-1' | jq .
```

- Status by base (all instances of base name)

```shell
curl -s 'localhost:8080/api/status?base=demo' | jq .
```

- Status by wildcard (new)

```shell
curl -s 'localhost:8080/api/status?wildcard=demo-*' | jq .

curl -s 'localhost:8080/api/status?wildcard=*extra*' | jq .
```

- Stop by exact name

```shell
curl -s -X POST 'localhost:8080/api/stop?name=demo-1' | jq .
```

- Stop by base

```shell
curl -s -X POST 'localhost:8080/api/stop?base=demo' | jq .
```

- Stop by wildcard (new)

```shell
curl -s -X POST 'localhost:8080/api/stop?wildcard=demo-*' | jq .
```

Notes

- You can change the base path by setting API_BASE (default /api).
- The API uses JSON for both requests and responses; time values accept Go duration strings (e.g., 2s, 500ms).
