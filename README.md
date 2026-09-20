# web-service

A small Go HTTP service built with [Gin](https://github.com/gin-gonic/gin), with health checks, graceful shutdown, and a sample JSON API.

## Requirements

- Go 1.22+

## Quick start

```bash
go run ./cmd/server
```

The server listens on `:8080` by default. Override with `ADDR` or `PORT`:

```bash
ADDR=:3000 go run ./cmd/server
PORT=3000 go run ./cmd/server
```

## Endpoints

| Method | Path            | Description                          |
|--------|-----------------|--------------------------------------|
| GET    | `/healthz`      | Liveness probe                       |
| GET    | `/api/v1/hello` | Greeting (`?name=` optional)         |

Examples:

```bash
curl -s http://localhost:8080/healthz
curl -s 'http://localhost:8080/api/v1/hello?name=Ada'
```

## Development

```bash
go test ./...
go build -o bin/server ./cmd/server
```

## Docker

```bash
docker build -t web-service .
docker run --rm -p 8080:8080 web-service
```
