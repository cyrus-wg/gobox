
# 📦 GoBox

[![Go](https://img.shields.io/badge/Go-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

---

## 🚀 Overview

**GoBox** provides a growing set of general-purpose utility packages for Go developers. Built with simplicity and performance in mind, packages are designed to work cohesively — some packages build upon shared foundations within GoBox to reduce boilerplate and ensure consistency across your codebase.

Whether you're spinning up a new microservice or maintaining a large-scale application, GoBox offers reliable building blocks that follow Go best practices and idiomatic patterns.

## ✨ Highlights

- **Cohesive Design** — Packages are designed to complement each other. Some build on shared internal foundations, providing a consistent and unified developer experience.
- **Production-Ready** — Designed for real-world use cases with stability and performance as core priorities.
- **Well-Tested** — Packages are thoroughly tested to ensure reliability across environments.
- **Idiomatic Go** — Follows standard Go conventions, making it intuitive and easy to integrate.
- **Minimal External Dependencies** — Keeps your dependency tree clean and your builds fast.
- **Actively Maintained** — New utilities and improvements are continuously being developed.

## 📦 Installation

```bash
go get github.com/cyrus-wg/gobox
```

## 📁 Project Structure

```
gobox/
├── pkg/          # Utility packages (some with internal cross-dependencies)
├── example/      # Usage examples
├── go.mod
├── go.sum
└── LICENSE
```

Explore the `pkg/` directory for available packages and the `example/` directory for practical usage references.

## 📦 Packages

| Package | Purpose |
| --- | --- |
| [`functional`](pkg/functional) | Generic slice / map helpers (Map, Filter, Reduce, GroupBy, Chunk, Unique...) plus lazy `iter.Seq` pipelines (Seq, MapSeq, FilterSeq, TakeSeq) |
| [`retry`](pkg/retry) | Configurable retry with exponential backoff, jitter, retryable-error predicates, and MaxElapsedTime budget |
| [`http_client`](pkg/http_client) | Typed JSON client with generics (`GetJSON[T]`, `PostJSON[T]`) and structured `ResponseError` handling |
| [`http_error`](pkg/http_error) | HTTP status-code errors with configurable error-message generators and sentinels |
| [`http_response`](pkg/http_response) | JSON response writing helpers that reuse typed HTTP errors |
| [`gorm_db`](pkg/gorm_db) | Generic GORM repository — type-safe `Create`/`GetByID`/`List`/`Update`/`Delete`/`Count` with sentinel errors |
| [`redis_db`](pkg/redis_db) | Redis helpers built on go-redis with sentinel errors |
| [`logger`](pkg/logger) | Zap wrapper with atomic global access, child loggers (`With`/`Named`), and runtime level control |
| [`grpc_interceptor`](pkg/grpc_interceptor) | gRPC interceptors: panic recovery, request logging, request validation |
| [`middleware`](pkg/middleware) | HTTP middleware: logging with status/bytes captured, CORS, request-ID, recovery |
| [`graceful_shutdown`](pkg/graceful_shutdown) | Signal-driven graceful server shutdown with health checks |
| [`pick`](pkg/pick) | Generic selection helpers: `If`, `OrDefault`, `Coalesce`, `Deref`, `Ptr`, string parsers |
| [`track`](pkg/track) | Deferred timing helpers (`TrackTime`, `TrackTimeWith`) that log durations with caller location |
| [`pattern`](pkg/pattern) | Miscellaneous pattern utilities |

## 🛠️ Usage

Import the specific package you need directly:

```go
import "github.com/cyrus-wg/gobox/pkg/<package_name>"
```

Requires **Go 1.27+** (uses `iter.Seq`, generics, and `math/rand/v2`).

Refer to the [`example/`](example/) directory for detailed usage demonstrations.
