# serverpeek 1.3.0 Beta 1 - 18 Jun 2026

- Faster, steadier updates on busy servers — snapshot collectors (CPU, memory, disk, processes, Docker, network) now run concurrently, so a cycle costs as much as the slowest single source rather than the sum of them all
- Docker `stats` and per-container `top` calls now run in parallel instead of one-at-a-time
- Update interval is now measured from the start of collection, so the period stays close to 2 seconds instead of drifting out to 2 seconds plus however long collection took

# serverpeek 1.2.0 - 3 Jun 2026

- Added a Disk card to the dashboard — shows used/total, free space, and a colour-coded usage bar for the main disk
- Reports the whole-disk figure via statfs on the root mount; deliberately ignores virtual filesystems, overlays, and external drives

# serverpeek 1.1.1 - 30 May 2026

Moved to new GitHub location: https://github.com/WaterJuice/serverpeek

# serverpeek 1.1.0 - 21 Apr 2026

- Collector now runs continuously instead of sleeping when no clients are connected, so new connections immediately see a populated history graph

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
