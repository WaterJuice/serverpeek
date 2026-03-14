# serverpeek

A live-updating web dashboard for server monitoring — CPU, memory, processes, Docker containers, and network ports in your browser.

## Why?

Checking server health usually means SSH-ing in and running `htop`, `docker ps`, `ss -tlnp`, and half a dozen other commands. serverpeek gives you all of that in a single, auto-refreshing browser dashboard that you can leave open on a second monitor.

## Features

- **Machine & OS info** — hostname, platform, architecture, CPU model, uptime
- **CPU usage** — overall and per-core with colour-coded bars (green → yellow → red)
- **Memory usage** — physical and swap with macOS breakdown (app/wired/compressed), excludes file cache
- **Load average** — 1, 5, and 15 minute load averages
- **Top processes** — sorted by combined CPU + memory usage, grouped by parent process
- **Smart process names** — identifies scripts behind interpreters (python, node, ruby, perl, java) with runtime tags
- **Docker containers** — shows internal container processes via `docker top`, merged into unified process view
- **Listening ports** — non-localhost network ports with associated process names
- **CPU & memory history graphs** — 2 minute rolling history, shared across all viewers
- **Live updates** — Server-Sent Events push new data every 2 seconds
- **Efficient** — single background collector thread, sleeps when no clients are connected
- **Beautiful dark theme** — clean, modern kiosk-friendly dashboard (fits in one screen, no scrolling)
- **Zero config** — just run it and open the URL

## Requirements

- Python 3.12+
- psutil (installed automatically)
- Docker CLI (optional, for container monitoring)

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

## How It Works

serverpeek starts an HTTP server that serves a single-page dashboard. A single background thread collects system snapshots every 2 seconds and shares them with all connected clients via Server-Sent Events (SSE). When no clients are connected, the collector sleeps. New clients receive the full 2-minute history buffer so graphs are populated immediately.

System information is gathered using psutil (CPU, memory, processes) and Docker CLI subprocess calls (containers, internal processes). On macOS, memory usage excludes file cache (reports active + wired + compressed instead).

The dashboard is a self-contained HTML page with embedded CSS and JavaScript — no build tools, no npm, no bundlers. Designed for kiosk use: everything fits in a single non-scrolling screen.

## Licence

Released under the [Unlicense](https://unlicense.org/) — public domain.
