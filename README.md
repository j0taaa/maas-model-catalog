# MaaS Model Catalog

Small JSON web service for Huawei Cloud MaaS model pricing, limits, and API URLs.

## Endpoints

- `GET /models` returns the model catalog.
- `GET /health` returns `{ "status": "ok" }`.

## Development

```sh
go test ./...
go run .
```

The server listens on `PORT`, defaulting to `3000`.

## Docker

```sh
docker compose up -d --build
```
