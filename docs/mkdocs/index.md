# serverpeek

A live-updating web dashboard for server monitoring — CPU, memory, disk, processes, Docker containers, and network ports in your browser.

## Why?

Checking server health usually means SSH-ing in and running `htop`, `docker ps`, `ss -tlnp`, and half a dozen other commands. serverpeek gives you all of that in a single, auto-refreshing browser dashboard that you can leave open on a second monitor.

## Features

- **Machine & OS info** — hostname, platform, architecture, CPU model, uptime
- **CPU usage** — overall with colour-coded bar (green → yellow → red), per-core bars on Linux
- **Memory usage** — physical and swap with macOS breakdown (app/wired/compressed), excludes file cache
- **Disk usage** — main disk used/total with free space and a colour-coded bar (green → yellow → red)
- **Load average** — 1, 5, and 15 minute load averages
- **Top processes** — sorted by combined CPU + memory usage, grouped by parent process
- **Smart process names** — identifies scripts behind interpreters (python, node, ruby, perl, java) with runtime tags
- **Docker containers** — shows internal container processes via `docker top`, merged into unified process view
- **Listening ports** — non-localhost network ports with associated process names
- **CPU & memory history graphs** — 2 minute rolling history, shared across all viewers
- **Live updates** — Server-Sent Events push new data every 2 seconds
- **Efficient** — single background collector shared across all viewers, so resource usage stays constant regardless of client count
- **Beautiful dark theme** — clean, modern kiosk-friendly dashboard (fits in one screen, no scrolling)
- **Zero config** — just run it and open the URL
- **Single binary** — statically-linked Go executable, no runtime dependencies

## Quick Start

### Install

```bash
pip install serverpeek
```

Or run directly with uv:

```bash
uvx serverpeek
```

### Run

```bash
serverpeek
```

Then open [http://localhost:8080](http://localhost:8080) in your browser.

### Options

```bash
serverpeek --port 9090        # Custom port
serverpeek --host 127.0.0.1   # Bind to localhost only
serverpeek --help             # Show all options
```

See the [Usage](usage.md) page for full details.

## How It Works

serverpeek starts an HTTP server that serves a single-page dashboard. A background goroutine collects system snapshots every 2 seconds and shares them with all connected clients via Server-Sent Events (SSE). The collector runs continuously, so new clients receive the full 2-minute history buffer and see populated graphs immediately on connect.

System information is gathered using OS-native interfaces (/proc on Linux, sysctl/vm_stat/statfs on macOS) and subprocess calls for Docker and network port information. On macOS, memory usage excludes file cache (reports active + wired + compressed instead).

The dashboard is a self-contained HTML page with embedded CSS and JavaScript — compiled into the binary via `go:embed`. No build tools, no npm, no bundlers. Designed for kiosk use: everything fits in a single non-scrolling screen.
