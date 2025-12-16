
# GoStash

[![Go Reference](https://pkg.go.dev/badge/github.com/sracha4355/GoStash.svg)](https://pkg.go.dev/github.com/sracha4355/GoStash)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/sracha4355/GoStash?label=release)](https://github.com/sracha4355/GoStash/releases)
[![Stars](https://img.shields.io/github/stars/sracha4355/GoStash?style=social)](https://github.com/sracha4355/GoStash)


**GoStash** is a growing library of generic, reusable Go components and utilities for building robust, maintainable applications. It aims to provide well-tested, idiomatic micro-libraries for common tasks, patterns, and concurrency.

---

## ✨ Features

- Generic data structures (stacks, queues, caches)
- Concurrency helpers (worker pools, reusable writer goroutines)
- Utility packages (string helpers, validation, adapters)
- Lightweight integrations (logging, I/O helpers)

## 🚀 Getting Started

> **Note:** GoStash is in early development. APIs and packages may change.

Install (when packages are published):

```bash
go get github.com/sracha4355/GoStash@latest
```

Import a package:

```go
import "github.com/sracha4355/GoStash/pkgname"
```

## 🛠 Usage Example

Example: Using a generic stack (future API)

```go
import "github.com/sracha4355/GoStash/stack"

stack := stack.New[int]()
stack.Push(42)
val, _ := stack.Pop()
fmt.Println(val) // 42
```

## 🗺 Roadmap

Planned features and packages:

1. **`Reusable writer goroutine`** — A well-tested package for concurrent, batched writing with flush semantics and backpressure.
2. **`Generic stack/queue`** implementations
3. **`Small string`** utilities
4. **`Example programs`** demonstrating best practices

**Reusable writer goroutine (details):**

- **Goal:** Provide a drop-in utility for writing to an `io.Writer` from multiple producers, with batching, flush, graceful shutdown, and error reporting.
- **Why:** Many apps need safe, non-blocking I/O from multiple goroutines without flooding the writer or losing data during shutdowns.
- **Acceptance criteria:**
    - Supports concurrent `Write` calls
    - Batches by count or time
    - Applies backpressure or returns error when buffer is full
    - Clean shutdown with flush
    - Unit tests for concurrency, batching, flush-on-close, error handling

## 🤝 Contributing

Contributions are welcome! If you'd like to help, open an issue to discuss your idea or submit a PR with tests and examples. See the `LICENSE` for terms.

## 📄 License

This project is licensed under the MIT License. See the `LICENSE` file for details.

