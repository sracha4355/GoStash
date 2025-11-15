# GoStash

[![Go Reference](https://pkg.go.dev/badge/github.com/sracha4355/GoStash.svg)](https://pkg.go.dev/github.com/sracha4355/GoStash)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/sracha4355/GoStash?label=release)](https://github.com/sracha4355/GoStash/releases)
[![Stars](https://img.shields.io/github/stars/sracha4355/GoStash?style=social)](https://github.com/sracha4355/GoStash)

A small and growing collection of generic, reusable Go components and utilities.

Currently this repository is a placeholder for a growing collection of generic, reusable Go components. Right now there are no published packages in the repo — the project is just getting started.

The intent is to host small, focused packages that other projects can import. Future packages will favor clear, idiomatic APIs and use Go generics where they provide value.

**Table of Contents**
- Overview
- Roadmap / TODOs
- Installation
- Contributing
- License

## Overview

GoStash will become a set of micro-libraries for common tasks and patterns. Examples of potential package categories:

- Generic data structures (stacks, queues, caches)
- Concurrency helpers (worker pools, reusable writer goroutines)
- Small utils (string helpers, validation, small adapters)
- Lightweight integrations (logging adapters, small I/O helpers)

## Roadmap / TODOs

Planned items to add in the near future:

1. **`Reusable writer goroutine`** — A small, well-tested package that provides a reusable writer goroutine pattern with batching, flush semantics, and backpressure handling.
2. **`Generic stack/queue`** implementations
3. **`Small string`** utilities
4. **`Example programs`** demonstrating best practices

**TODO**: Reusable writer goroutine (detailed)

1. **`Goal:`** Provide a drop-in utility that runs a single goroutine responsible for writing to an io.Writer (or similar sink). The goroutine accepts write requests from multiple producers and handles batching, time-based flush, graceful shutdown, and error reporting.
2. **`Why:`** Many applications need a safe, non-blocking way to perform I/O from multiple goroutines without flooding the writer or losing data during restarts/shutdowns.

- **Acceptance criteria:**

    - Supports concurrent `Write` calls from multiple goroutines.
    - Batches items by count or time (whichever comes first).
    - Applies backpressure or returns a defined error when internal buffer is full.
    - Clean shutdown with `Close` that flushes pending items and honors context timeouts.
    - Unit tests covering concurrency, batching, flush-on-close, and error handling.

## Installation (future)

When packages are published, installation will use `go get` and module imports like:

```bash
go get github.com/sracha4355/GoStash@latest
```

## Contributing

Contributions are welcome even at this early stage. If you'd like to implement the reusable writer goroutine, please open an issue to discuss the API first or submit a PR with tests and examples.

## License

This project is licensed under the terms in the `LICENSE` file at the repository root.

