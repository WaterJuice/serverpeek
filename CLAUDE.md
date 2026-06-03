# CLAUDE.md

This file provides guidance for AI agents working on this project.

## Project Overview

serverpeek is a live-updating web dashboard for server monitoring. It displays CPU usage, memory usage, main disk space usage, machine and OS information, high-CPU processes, Docker containers, and open network ports. The server uses Server-Sent Events (SSE) for real-time updates to a single-page web dashboard.

The project is written in Go and distributed as platform-specific Python wheels via PyPI (using bin2whl). This means users install it with `pip install serverpeek` or `uvx serverpeek`, but the binary is a statically-linked Go executable — no Python runtime required at execution time.

## Language and Spelling

Use **Australian English** throughout:
- colour (not color)
- initialise (not initialize)
- sanitise (not sanitize)
- organisation (not organization)

## Code Style

### Go Files

Every Go file should have:
1. A file header block with description and version history
2. Section headers separating major sections (Imports, Constants, Functions, etc.)
3. Horizontal separators (87 dashes after `// `, 90 chars total) above each function definition

Example structure:
```go
// ---------------------------------------------------------------------------------------
//
//	filename.go
//	-----------
//
//	Brief description of what this module does.
//
//	(c) 2026 WaterJuice — Released under the Unlicense; see LICENSE.
//
//	Version History
//	---------------
//	Mar 2026 - Created
//
// ---------------------------------------------------------------------------------------
package internal

// ---------------------------------------------------------------------------------------
//
//	Imports
//
// ---------------------------------------------------------------------------------------

import (
	"fmt"
)

// ---------------------------------------------------------------------------------------
//
//	Functions
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// MyFunction does something.
func MyFunction() {
}
```

### General

- Go 1.25+
- Zero external dependencies — stdlib only
- Use `gofmt` for formatting, `go vet` for linting
- Run `make format` to auto-fix formatting
- Run `make check` to verify formatting and lint
- CLI uses manual argument parsing (no flag package, no external CLI libs)
- TTY-aware ANSI colours for terminal output

## Common Commands

```bash
make help       # Show all available targets
make check      # Run gofmt check + go vet
make format     # Auto-format Go source with gofmt
make go-build   # Cross-compile for all 6 platforms
make build      # Full build: check, go-build, docs, platform wheels
make docs       # Build HTML documentation into html/
make clean      # Remove build artefacts
make dev        # Set up .venv + a go-run launcher (runs from source, no rebuild)
make run ARGS="--port 3000"  # Run from source via `go run` with arguments
```

## Project Structure

```
main.go                 # Entry point, calls internal.Run(Version)
go.mod                  # Go module definition
internal/
├── cli.go              # CLI argument parsing, help text, colour helpers
├── server.go           # HTTP server with embedded web UI and SSE streaming
├── sysinfo.go          # System info types and platform-independent logic
├── sysinfo_darwin.go   # macOS-specific system info (sysctl, vm_stat, ps)
├── sysinfo_linux.go    # Linux-specific system info (/proc, ps)
├── sysinfo_windows.go  # Windows stubs (minimal)
└── static/
    └── index.html      # Web UI (single-page app with embedded CSS/JS)
wheel.json              # bin2whl configuration for platform wheels
pyproject.toml          # Minimal — just for uv dev dependencies
Makefile                # Build orchestration
```

## Architecture

### Web Server
- HTTP server built on `net/http` — zero dependencies
- Web UI embedded at compile time using `//go:embed`
- Single-page HTML dashboard served at `/` with `Cache-Control: no-cache`
- SSE endpoint at `/api/stream` streams shared snapshots to all clients
- JSON snapshot endpoint at `/api/snapshot` for one-off queries
- Single background goroutine gathers data every 2 seconds
- Collector sleeps when no SSE clients are connected, wakes on first connect
- New clients receive full 2-minute history buffer (60 snapshots) on connect
- Resource usage is constant regardless of number of connected clients

### System Information
- CPU usage per core via /proc/stat (Linux); aggregate-only on macOS (per-core requires Mach API / cgo)
- CPU model detection with fallbacks: sysctl (macOS), /proc/cpuinfo, lscpu
- Memory and swap usage via /proc/meminfo (Linux) or vm_stat (macOS)
- On macOS, reports active+wired+compressed (excludes file cache)
- Main disk usage via statfs on the root mount (`/`); reports the whole-disk figure and ignores virtual filesystems, overlays, and external drives
- Machine info (hostname, platform, architecture, CPU model, uptime) via sysctl/proc
- Top processes via ps, sorted by combined CPU + memory usage (normalised per core)
- Processes grouped by parent PID + name (e.g. 10 stress workers → "stress (x10)")
- Friendly names for interpreter processes: extracts script/module from cmdline for python, node, ruby, perl, java
- Docker containers via `docker ps`, `docker stats`, and `docker top` for internal processes
- Docker internal processes expanded and merged into unified process view
- Network connections via `lsof` (filters out localhost-only bindings)

### Dashboard
- Dark-themed single-page app with embedded CSS and JavaScript
- Kiosk-friendly: 100vh viewport-filling CSS Grid layout, no scrolling
- Auto-connects to SSE for real-time updates with history replay
- CPU and memory history graphs (Canvas API, 2-minute rolling window)
- Colour-coded CPU, memory, swap, and disk bars (green → yellow → red)
- Runtime tags on processes (e.g. "python 3.14", "container")
- Unified process table merging host processes and Docker container internals

## Key Design Decisions

1. **Go binary, PyPI distribution** — statically-linked Go binary wrapped in platform wheels via bin2whl
2. **Zero dependencies** — Go stdlib only
3. **net/http server** — idiomatic Go HTTP server with SSE
4. **Single HTML file** — web UI is one self-contained file with embedded CSS/JS
5. **go:embed** — HTML compiled into the binary, no external files needed at runtime
6. **Manual CLI parsing** — no flag package, consistent with tls-switch project conventions
7. **Platform build tags** — sysinfo_darwin.go, sysinfo_linux.go, sysinfo_windows.go

## Build & Distribution

- Cross-compiled for 6 platforms: macOS (arm64/amd64), Linux (arm64/amd64), Windows (arm64/amd64)
- All binaries are statically linked (`CGO_ENABLED=0`)
- Version injected at build time via `-ldflags -X main.Version=...`
- Platform wheels built using `bin2whl` from `wheel.json` config
- Published to PyPI via `cal-publish-python`

## Testing Changes

After making changes:
1. Run `make check` to verify formatting and vet pass
2. Run `make go-build` to verify cross-compilation works
3. Test with `make run` to verify the server works

## Versioning

- Version is derived from git tags via `git describe --tags --always`
- Create a tag like `1.0.0` before running `make build` for a release (no `v` prefix)
- Version is injected at build time via `-ldflags`
- Falls back to "dev" if no tags exist

## Commits

When committing:
- Use clear, descriptive commit messages
- Include `Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>` in commits made with AI assistance
- **Never rewrite git history** unless explicitly asked to

## Licence

Released under the [Unlicense](https://unlicense.org/) — public domain.
