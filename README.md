# Checkout API

Xsolla School training project — a simplified checkout API built with Go.

Students build this service incrementally across lectures, starting from a basic HTTP server and evolving it into a production-ready system with persistence, authentication, observability, and more.

## Prerequisites

### Tools

- Go 1.21+
- Docker (for running Postgres locally)
- `golang-migrate` CLI for running migrations:
  ```bash
  brew install golang-migrate
  ```
- `swag` CLI for generating OpenAPI docs:
  ```bash
  go install github.com/swaggo/swag/cmd/swag@latest
  ```
- curl or Postman (for testing endpoints)
- A code editor (VS Code, GoLand, etc.)

### Starting Postgres and applying migrations

```bash
# Start Postgres
docker run -d --name checkout-postgres \
  -e POSTGRES_PASSWORD=postgres \
  -p 5432:5432 postgres:16

# Apply migrations
migrate -path ./migrations \
  -database "postgresql://postgres:postgres@localhost:5432/postgres?sslmode=disable" up
```

### Go dependencies

All dependencies are managed via `go.mod`. Run once after cloning:

```bash
go mod download
```

Key packages used in this lecture:

| Package | Purpose |
|---|---|
| `github.com/jackc/pgx/v5` | PostgreSQL driver |
| `github.com/golang-jwt/jwt/v5` | JWT signing and parsing |
| `golang.org/x/crypto` | bcrypt password hashing |
| `github.com/go-playground/validator/v10` | Struct tag validation |
| `github.com/swaggo/swag` | OpenAPI spec generation from annotations |
| `github.com/swaggo/http-swagger` | Swagger UI handler |

## Quick Start

```bash
go run main.go
```

The server starts on http://localhost:8080.

## API Endpoints

### GET /items

Returns all available items.

```bash
curl http://localhost:8080/items
```

### POST /orders

Creates an order with mock payment processing.

```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"user_id": 1, "items": [{"item_id": 1, "quantity": 2}]}'
```

## Running Tests

```bash
go test -v ./handlers/
```

## Branch Guide

Each lecture has two branches:

| Branch | Purpose |
|--------|---------|
| `week-XX/lecture-XX` | **Starter** — scaffold with TODOs and pre-written tests. Fork from here at the start of class. |
| `week-XX/lecture-XX-final` | **Final** — completed code matching the lecture. Compare your work against this. |

### Available Branches

- `week-01/lecture-01` — Intro to HTTP and JSON APIs (starter)
- `week-01/lecture-01-final` — Intro to HTTP and JSON APIs (completed)
- `week-07/lecture-02` — Clean API Design & DDD (starter) ← **you are here**
- `week-07/lecture-02-final` — Clean API Design & DDD (completed)

## Project Structure (Week 1)

```
checkout-api/
├── main.go                  # HTTP server entry point
├── models/models.go         # Domain models (Item, LineItem, Order)
├── store/store.go           # In-memory data storage
├── handlers/handlers.go     # HTTP route handlers
└── handlers/handlers_test.go # Table-driven handler tests
```

## License

Internal — Xsolla School use only.
