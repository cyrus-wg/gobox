
# 📦 GoBox

[![Go](https://img.shields.io/badge/Go-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/cyrus-wg/gobox)](https://goreportcard.com/report/github.com/cyrus-wg/gobox)

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

## 🛠️ Usage

Import the specific package you need directly:

```go
import "github.com/cyrus-wg/gobox/pkg/<package_name>"
```

Refer to the [`example/`](example/) directory for detailed usage demonstrations.
