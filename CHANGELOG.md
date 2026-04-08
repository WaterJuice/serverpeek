# serverpeek 1.0.0 Beta 6 - 8 Apr 2026

- Rewritten from Python to Go for single-binary distribution
- Distributed as platform-specific Python wheels via bin2whl (no Python runtime needed)
- Zero external dependencies — Go stdlib only
- System info gathered via OS-native interfaces (/proc, sysctl, vm_stat, ps) instead of psutil
- Cross-compiled for macOS (arm64/amd64), Linux (arm64/amd64), Windows (arm64/amd64)
- All existing features preserved: CPU, memory, processes, Docker, network monitoring
- Web dashboard unchanged — same dark-themed kiosk-friendly UI

# serverpeek 1.0.0 Beta 4 - 15 Mar 2026

- Initial release
- Live-updating web dashboard with CPU, memory, processes, Docker, and network monitoring
- Server-Sent Events for real-time updates
- Beautiful dark-themed responsive dashboard
