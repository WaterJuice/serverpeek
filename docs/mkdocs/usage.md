# Usage

web-proc-info-server provides a single command that starts the monitoring dashboard.

## Starting the server

```bash
web-proc-info-server
```

This starts the server on `0.0.0.0:8080` by default. Open [http://localhost:8080](http://localhost:8080) in your browser to view the dashboard.

## Options

| Option            | Description                              |
|-------------------|------------------------------------------|
| `--port`, `-p`    | Port to listen on (default: 8080)        |
| `--host`, `-H`    | Host to bind to (default: 0.0.0.0)      |
| `--version`       | Show version and exit                    |
| `--license`       | Show licence information and exit        |
| `--help`          | Show help and exit                       |

### Examples

```bash
# Start on a custom port
web-proc-info-server --port 9090

# Bind to localhost only (not accessible from other machines)
web-proc-info-server --host 127.0.0.1

# Custom port and host
web-proc-info-server -H 127.0.0.1 -p 3000
```

## Dashboard Sections

### Machine & OS

Displays static machine information:

- **Hostname** — machine name
- **OS Release** — kernel/OS version
- **CPU** — processor model
- **Cores** — physical and logical core count
- **Total Memory** — installed RAM
- **Architecture** — CPU architecture (x86_64, arm64, etc.)

### CPU

Shows real-time CPU usage:

- **Overall CPU** — aggregate usage across all cores with a colour-coded bar
- **Per-core bars** — individual usage for each logical core
- **Load average** — 1, 5, and 15 minute system load averages

Bars are colour-coded: green (< 50%), yellow (50–80%), red (> 80%).

### Memory

Shows physical memory and swap usage:

- **Physical Memory** — used and total with percentage bar
- **Breakdown** — app (active), wired, and compressed memory (macOS)
- **Swap** — used and total with percentage bar (only shown if swap is configured)

On macOS, used memory is calculated as active + wired + compressed, which excludes file cache. This gives a more accurate picture of real memory pressure — file cache is freely reclaimable and does not represent the system running out of memory.

### CPU & Memory History

Rolling 2-minute graphs for CPU and memory usage. History is maintained server-side, so new clients see the full graph immediately on connect.

### Top Processes

A unified table of the top processes sorted by combined CPU + memory usage:

| Column  | Description                          |
|---------|--------------------------------------|
| Name    | Process name (with runtime tag)      |
| CPU%    | Current CPU usage percentage         |
| Mem     | Resident memory (absolute)           |
| Source  | User or container name               |

Processes are grouped by parent process — e.g. 10 multiprocessing workers spawned by the same program appear as a single row like "my_app.py (x10)".

For interpreter processes (python, node, ruby, perl, java), the actual script or module name is extracted from the command line, with a coloured runtime tag (e.g. "python 3.14"). Multiprocessing workers resolve the parent program name or the tool name from the binary path (e.g. uv tools).

Docker container internal processes are expanded via `docker top` and merged into this table with a "container" tag. The VM host process is hidden when Docker containers are present.

!!! note
    Docker monitoring requires the Docker CLI to be installed and accessible.

### Listening Ports

Shows non-localhost network ports in LISTEN state:

| Column  | Description                          |
|---------|--------------------------------------|
| Port    | Port number                          |
| Address | Full local address (ip:port)         |
| Process | Process name holding the port        |

Ports bound only to localhost (127.0.0.1, ::1) are filtered out.

## API Endpoints

The server exposes two API endpoints:

| Endpoint         | Description                                              |
|------------------|----------------------------------------------------------|
| `GET /`          | Serves the dashboard HTML page                           |
| `GET /api/snapshot` | Returns a single JSON snapshot of current system state |
| `GET /api/stream`   | Server-Sent Events stream, pushes data every 2 seconds |

### Snapshot format

The `/api/snapshot` endpoint returns JSON with these top-level keys:

```json
{
    "timestamp": 1710000000.0,
    "machine": { ... },
    "cpu": { ... },
    "memory": { ... },
    "processes": [ ... ],
    "docker": [ ... ],
    "network": [ ... ]
}
```

## Running with uv

```bash
# Run directly without installing
uvx web-proc-info-server

# Or with options
uvx web-proc-info-server --port 9090
```

## Running as a module

```bash
python -m web_proc_info_server
python -m web_proc_info_server --port 9090
```
