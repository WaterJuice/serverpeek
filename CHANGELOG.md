# serverpeek 1.0.0 - 14 Apr 2026

- Initial release
- Live-updating web dashboard for server monitoring via Server-Sent Events
- Dark-themed, kiosk-friendly single-page UI — no scrolling, fills the viewport
- Monitors CPU usage (overall and per-core), memory, swap, and system uptime
- Top processes sorted by combined CPU + memory usage, grouped by parent
- Friendly process names for interpreters (Python, Node, Ruby, Perl, Java)
- Docker container monitoring with stats and internal process expansion
- Open network port detection via lsof (excludes localhost-only bindings)
- Written in Go with zero external dependencies — stdlib only
- Distributed as platform-specific Python wheels via bin2whl (`pip install serverpeek`)
- Cross-compiled for macOS (arm64/amd64), Linux (arm64/amd64), Windows (arm64/amd64)
- Statically linked single binary — no Python runtime needed at execution time
- Linux distro detection via /etc/os-release (e.g. "Ubuntu 24.04")
- macOS version display using kern.osproductversion (e.g. "26.4")
