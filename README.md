# Ouiligo

A small Go HTTP service with a live train timetable page for **Gare de La Ménitré**, powered by the [SNCF Open Data API](https://numerique.sncf.com/startup/api/).

## Requirements

- Go 1.22+
- Free SNCF API token ([register here](https://numerique.sncf.com/startup/api/))

## Quick start

```bash
export SNCF_API_TOKEN=your-token-here
go run ./cmd/server
```

Open [http://localhost:8080](http://localhost:8080) for the departure board.

The server listens on `:8080` by default. Override with `ADDR` or `PORT`:

```bash
ADDR=:3000 go run ./cmd/server
PORT=3000 go run ./cmd/server
```

## Endpoints

| Method | Path            | Description                                      |
|--------|-----------------|--------------------------------------------------|
| GET    | `/`             | Timetable webpage (La Ménitré)                   |
| GET    | `/healthz`      | Liveness probe                                   |
| GET    | `/api/v1/hello` | Greeting (`?name=` optional)                     |
| GET    | `/api/v1/trains`| Next trains (`direction=departures\|arrivals`) |

Examples:

```bash
curl -s http://localhost:8080/healthz
curl -s 'http://localhost:8080/api/v1/trains?direction=departures'
```

## Development

```bash
go test ./...
go build -o bin/server ./cmd/server
```

## Docker

### Local build

```bash
export SNCF_API_TOKEN=your-token-here
docker compose up --build
```

Or with plain Docker:

```bash
docker build -t ouiligo .
docker run --rm -p 8080:8080 -e SNCF_API_TOKEN="$SNCF_API_TOKEN" ouiligo
```

Override the published host port with `PORT`:

```bash
PORT=3000 docker compose up --build
```

### Image publiée sur GitHub (GHCR)

À chaque push sur `main`, GitHub Actions publie l’image :

```text
ghcr.io/willylambert/ouiligo:latest
```

Lancer depuis le registry (sans rebuild local) :

```bash
export SNCF_API_TOKEN=your-token-here
docker pull ghcr.io/willylambert/ouiligo:latest
docker run --rm -p 8080:8080 -e SNCF_API_TOKEN="$SNCF_API_TOKEN" ghcr.io/willylambert/ouiligo:latest
```

Ou avec Compose :

```bash
export SNCF_API_TOKEN=your-token-here
docker compose up
```

Si le package GHCR est **privé**, authentifie-toi une fois :

```bash
echo "$GITHUB_TOKEN" | docker login ghcr.io -u USERNAME --password-stdin
```

Sur GitHub : **Packages** du repo → package `ouiligo` → *Package settings* → rendre public si tu veux un pull sans login.
