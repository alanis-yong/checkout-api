# Checkout API

Xsolla School training project — a simplified checkout API built with Go.

Students build this service incrementally across lectures, starting from a basic HTTP server and evolving it into a production-ready system with persistence, authentication, observability, and more.

## Prerequisites

- Go 1.21+
- curl or Postman (for testing endpoints)
- A code editor (VS Code, GoLand, etc.)

## Quick Start

```bash
go run main.go
```

The server starts on http://localhost:8080.

## Branch Guide

Each lecture has two branches:

| Branch | Purpose |
|--------|---------|
| `week-XX/lecture-XX` | **Starter** — empty scaffold with TODOs. Fork from here at the start of class. |
| `week-XX/lecture-XX-final` | **Final** — completed code from the lecture. Compare your work against this. |

### Available Branches

- `week-01/lecture-01` — Intro to HTTP and JSON APIs (starter)
- `week-01/lecture-01-final` — Intro to HTTP and JSON APIs (completed)

## Project Structure (Week 1)

```
checkout-api/
├── main.go              # HTTP server entry point
├── models/models.go     # Domain models (Item, Cart, Order)
├── store/memory.go      # In-memory data storage
├── handlers/handlers.go # HTTP route handlers
└── docs/                # Documentation and ADRs
```

## License

Internal — Xsolla School use only.
