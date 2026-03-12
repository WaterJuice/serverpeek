# CLAUDE.md

This file provides guidance for AI agents working on this project.

## Project Overview

web-proc-info-server is a live-updating web dashboard for server monitoring. It displays CPU usage (per core), memory usage, machine and OS information, high-CPU processes, Docker containers, and open network ports. The server uses Server-Sent Events (SSE) for real-time updates to a single-page web dashboard.

## Language and Spelling

Use **Australian English** throughout:
- colour (not color)
- initialise (not initialize)
- sanitise (not sanitize)
- organisation (not organization)

## Code Style

### Python Files

Every Python file should have:
1. A file header block with description and version history
2. Section headers separating major sections (Imports, Constants, Functions, etc.)
3. Horizontal separators (92 chars of `-`) above each function definition

Example structure:
```python
# ----------------------------------------------------------------------------------------
#   filename.py
#   -----------
#
#   Brief description of what this module does.
#
#   (c) 2026 WaterJuice — Released under the Unlicense; see LICENSE.
#
#   Version History
#   ---------------
#   Mar 2026 - Created
# ----------------------------------------------------------------------------------------

# ----------------------------------------------------------------------------------------
#   Imports
# ----------------------------------------------------------------------------------------

import sys

# ----------------------------------------------------------------------------------------
#   Functions
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
def my_function() -> None:
    """Docstring here."""
    pass
```

### General

- Python 3.12+ (do **not** use `from __future__ import annotations`)
- Use type hints throughout
- Prefer pathlib.Path over os.path
- Single-line imports, no blank lines between import groups (configured in pyproject.toml)
- Run `make format` to auto-fix import ordering
- Single external dependency: psutil (for system information gathering)
- CLI uses argbuilder.py (custom argparse wrapper)

## Common Commands

```bash
make help       # Show all available targets
make check      # Run ruff + pyright
make format     # Auto-fix and format code
make build      # Build wheel + docs into output/
make docs       # Build HTML documentation into html/
make clean      # Remove build artefacts
make dev        # Just create dev (.venv) setup
```

## Project Structure

```
web_proc_info_server/
├── __init__.py       # Package init, exports __version__
├── __main__.py       # Entry point for python -m web_proc_info_server
├── version.py        # Version string handling
├── argbuilder.py     # Custom argparse wrapper (from cal-publish-python)
├── cli.py            # CLI argument parsing and server launch
├── server.py         # HTTP server with SSE streaming
├── system_info.py    # System information gathering (CPU, memory, processes, docker, network)
└── web/
    └── index.html    # Single-page dashboard (embedded CSS/JS)
```

## Architecture

### Web Server
- Uses stdlib `http.server` for HTTP serving (threaded)
- Single-page HTML dashboard served at `/` with `Cache-Control: no-cache`
- SSE endpoint at `/api/stream` streams shared snapshots to all clients
- JSON snapshot endpoint at `/api/snapshot` for one-off queries
- Single `_SnapshotCollector` background thread gathers data every 2 seconds
- Collector sleeps when no SSE clients are connected, wakes on first connect
- New clients receive full 2-minute history buffer (60 snapshots) on connect
- Resource usage is constant regardless of number of connected clients

### System Information
- CPU usage per core via psutil
- CPU model detection with fallbacks: sysctl (macOS), /proc/cpuinfo, lscpu, platform.machine()
- Memory and swap usage via psutil; on macOS reports active+wired+compressed (excludes file cache)
- Compressed memory via `vm_stat` (macOS only)
- Machine info (hostname, platform, architecture, CPU model, uptime) via platform + psutil
- Top processes sorted by combined CPU + memory usage (normalised per core)
- Processes grouped by parent PID + name (e.g. 10 stress workers → "stress (x10)")
- Friendly names for interpreter processes: extracts script/module from cmdline for python, node, ruby, perl, java
- Multiprocessing worker detection: resolves parent process or binary path (e.g. uv tools name)
- Docker containers via `docker ps`, `docker stats`, and `docker top` for internal processes
- Docker internal processes expanded and merged into unified process view
- Network connections via `lsof` (not psutil, which requires root on macOS); filters out localhost-only bindings

### Dashboard
- Dark-themed single-page app with embedded CSS and JavaScript
- Kiosk-friendly: 100vh viewport-filling CSS Grid layout, no scrolling
- Auto-connects to SSE for real-time updates with history replay
- CPU and memory history graphs (Canvas API, 2-minute rolling window)
- Colour-coded CPU and memory bars (green → yellow → red)
- Runtime tags on processes (e.g. "python 3.14", "container")
- Unified process table merging host processes and Docker container internals

## Testing Changes

After making changes:
1. Run `make check` to verify linting and types pass
2. Run `make build` to verify the full build works
3. Test with `uv run web-proc-info-server` to verify the server works

## Versioning

- Version is derived from git tags via uv-dynamic-versioning
- Create a tag like `1.0.0` before running `make build` for a release (no `v` prefix)
- The build generates `_version.py` at build time, which is not committed
- If no tags exist, version falls back to "dev"

## Commits

When committing:
- Use clear, descriptive commit messages
- Include `Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>` in commits made with AI assistance
- **Never rewrite git history** unless explicitly asked to

## Licence

Released under the [Unlicense](https://unlicense.org/) — public domain.
