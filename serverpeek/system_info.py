# ----------------------------------------------------------------------------------------
#   system_info.py
#   --------------
#
#   System information gathering — CPU, memory, processes, Docker containers,
#   and network connections. Uses psutil for system metrics and subprocess for
#   Docker information.
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

import json
import platform
import shutil
import subprocess
import time
from typing import Any
import psutil

# ----------------------------------------------------------------------------------------
#   Types
# ----------------------------------------------------------------------------------------

type SystemSnapshot = dict[str, Any]

# ----------------------------------------------------------------------------------------
#   Module State
# ----------------------------------------------------------------------------------------

_boot_time: float = psutil.boot_time()

# ----------------------------------------------------------------------------------------
#   Machine Information
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
def _get_cpu_model() -> str:
    """Get the CPU model string.

    Tries multiple sources in order: platform-specific commands, /proc/cpuinfo,
    lscpu, and finally platform.machine() as a last resort.
    """
    for func in (_cpu_from_sysctl, _cpu_from_procinfo, _cpu_from_lscpu):
        result = func()
        if result:
            return result

    # Last resort — at least say "aarch64" or "x86_64"
    return platform.machine() or "Unknown"


# ----------------------------------------------------------------------------------------
def _cpu_from_sysctl() -> str:
    """Try macOS sysctl."""
    if platform.system() != "Darwin":
        return ""
    try:
        result = subprocess.run(
            ["sysctl", "-n", "machdep.cpu.brand_string"],
            capture_output=True,
            text=True,
            timeout=5,
        )
        if result.returncode == 0:
            return result.stdout.strip()
    except (subprocess.TimeoutExpired, FileNotFoundError, OSError):
        pass
    return ""


# ----------------------------------------------------------------------------------------
def _cpu_from_procinfo() -> str:
    """Try /proc/cpuinfo (x86 'model name', ARM 'Hardware')."""
    try:
        with open("/proc/cpuinfo") as f:
            for line in f:
                if line.startswith(("model name", "Hardware")):
                    value = line.split(":", 1)[1].strip()
                    if value:
                        return value
    except (FileNotFoundError, OSError):
        pass
    return ""


# ----------------------------------------------------------------------------------------
def _cpu_from_lscpu() -> str:
    """Try lscpu command."""
    try:
        result = subprocess.run(
            ["lscpu"],
            capture_output=True,
            text=True,
            timeout=5,
        )
        if result.returncode == 0:
            for line in result.stdout.split("\n"):
                if line.startswith("Model name:"):
                    value = line.split(":", 1)[1].strip()
                    if value and value != "-":
                        return value
    except (subprocess.TimeoutExpired, FileNotFoundError, OSError):
        pass
    return ""


# ----------------------------------------------------------------------------------------
def _format_uptime(seconds: float) -> str:
    """Format uptime seconds into a human-readable string."""
    days = int(seconds // 86400)
    hours = int((seconds % 86400) // 3600)
    minutes = int((seconds % 3600) // 60)

    parts: list[str] = []
    if days > 0:
        parts.append(f"{days}d")
    if hours > 0:
        parts.append(f"{hours}h")
    parts.append(f"{minutes}m")
    return " ".join(parts)


# ----------------------------------------------------------------------------------------
def get_machine_info() -> dict[str, Any]:
    """Gather static machine and OS information."""
    uname = platform.uname()
    mem = psutil.virtual_memory()

    uptime_seconds = time.time() - _boot_time

    return {
        "hostname": uname.node,
        "platform": platform.system(),
        "os_version": uname.version,
        "os_release": uname.release,
        "architecture": uname.machine,
        "cpu_model": _get_cpu_model(),
        "cpu_cores_physical": psutil.cpu_count(logical=False) or 0,
        "cpu_cores_logical": psutil.cpu_count(logical=True) or 0,
        "total_memory_bytes": mem.total,
        "total_memory_gb": round(mem.total / (1024**3), 1),
        "uptime": _format_uptime(uptime_seconds),
        "uptime_seconds": round(uptime_seconds),
    }


# ----------------------------------------------------------------------------------------
#   CPU Information
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
def get_cpu_info() -> dict[str, Any]:
    """Get current CPU usage — overall and per-core."""
    per_core = psutil.cpu_percent(interval=None, percpu=True)
    overall = psutil.cpu_percent(interval=None)

    # Get load averages
    try:
        load_avg = list(psutil.getloadavg())
    except (AttributeError, OSError):
        load_avg = [0.0, 0.0, 0.0]

    return {
        "overall": overall,
        "per_core": per_core,
        "load_average": {
            "1min": round(load_avg[0], 2),
            "5min": round(load_avg[1], 2),
            "15min": round(load_avg[2], 2),
        },
    }


# ----------------------------------------------------------------------------------------
#   Memory Information
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
def _get_compressed_bytes() -> int:
    """Get compressed memory size from vm_stat (macOS only)."""
    if platform.system() != "Darwin":
        return 0
    try:
        result = subprocess.run(
            ["vm_stat"],
            capture_output=True,
            text=True,
            timeout=5,
        )
        if result.returncode != 0:
            return 0

        page_size = 16384  # Default on Apple Silicon
        for line in result.stdout.split("\n"):
            if line.startswith("Mach Virtual Memory Statistics"):
                # Extract page size from header: "...page size of NNNN bytes)"
                parts = line.split("page size of ")
                if len(parts) == 2:
                    page_size = int(parts[1].rstrip(" bytes)"))
            if "Pages occupied by compressor" in line:
                count = int(line.split(":")[1].strip().rstrip("."))
                return count * page_size
    except (subprocess.TimeoutExpired, FileNotFoundError, OSError, ValueError):
        pass
    return 0


# ----------------------------------------------------------------------------------------
def get_memory_info() -> dict[str, Any]:
    """Get current memory and swap usage."""
    mem = psutil.virtual_memory()
    swap = psutil.swap_memory()

    # macOS-specific breakdown
    active = getattr(mem, "active", 0)
    wired = getattr(mem, "wired", 0)
    compressed = _get_compressed_bytes()

    # On macOS, psutil's "used" includes file cache which is freely
    # reclaimable — not real memory pressure.  Use active + wired +
    # compressed instead.  On Linux, psutil already excludes
    # buffers/cache from "used" so we use the default values.
    if platform.system() == "Darwin" and (active or wired or compressed):
        real_used = active + wired + compressed
        real_percent = round((real_used / mem.total) * 100, 1) if mem.total else 0.0
    else:
        real_used = mem.total - mem.available
        real_percent = mem.percent

    return {
        "total": mem.total,
        "available": mem.available,
        "used": real_used,
        "percent": real_percent,
        "active": active,
        "wired": wired,
        "compressed": compressed,
        "swap_total": swap.total,
        "swap_used": swap.used,
        "swap_percent": swap.percent,
    }


# ----------------------------------------------------------------------------------------
#   Process Information
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
def _extract_version(binary_lower: str, prefix: str) -> str:
    """Extract version suffix from a binary name like 'python3.14' → '3.14'.

    Only returns a version if it contains a dot (i.e. major.minor), so
    bare major versions like '3' from 'python3' are not shown.
    """
    suffix = binary_lower[len(prefix) :]
    if suffix and suffix[0].isdigit() and "." in suffix:
        return suffix
    return ""


# ----------------------------------------------------------------------------------------
def _name_from_binary_path(binary: str) -> str:
    """Extract a meaningful name from the interpreter binary path.

    Recognises patterns like:
      /Users/.../.local/share/uv/tools/pushfill/bin/python → "pushfill"
      /path/to/venvs/myapp/bin/python → "myapp"
    """
    parts = binary.replace("\\", "/").split("/")
    # Look for uv tools pattern: .../uv/tools/<name>/bin/python
    if "tools" in parts and "bin" in parts:
        tools_idx = len(parts) - 1 - parts[::-1].index("tools")
        bin_idx = parts.index("bin", tools_idx)
        if bin_idx == tools_idx + 2:
            return parts[tools_idx + 1]
    # Look for virtualenv pattern: .../<name>/bin/python
    if "bin" in parts:
        bin_idx = parts.index("bin")
        if bin_idx > 0:
            candidate = parts[bin_idx - 1]
            # Skip generic venv names
            if candidate not in ("venv", ".venv", "env", ".env", "bin"):
                return candidate
    return ""


# ----------------------------------------------------------------------------------------
def _get_parent_friendly_name(ppid: int) -> str:
    """Look up the parent process and extract a friendly script/module name."""
    try:
        parent = psutil.Process(ppid)
        parent_name = parent.name()
        parent_cmdline = parent.cmdline()
        friendly, _ = _friendly_name(parent_name, parent_cmdline, 0)
        return friendly
    except (psutil.NoSuchProcess, psutil.AccessDenied, psutil.ZombieProcess):
        return ""


# ----------------------------------------------------------------------------------------
def _friendly_name(
    name: str, cmdline: list[str] | None, ppid: int = 0
) -> tuple[str, str]:
    """Derive a friendly display name and runtime tag for a process.

    Returns (display_name, tag) where tag is e.g. "python", "node", or ""
    for non-interpreter processes.
    """
    # Map process name prefixes to short labels
    interpreter_labels = {
        "python": "python",
        "node": "node",
        "ruby": "ruby",
        "perl": "perl",
    }
    lower = name.lower()

    label = ""
    for prefix, lbl in interpreter_labels.items():
        if lower.startswith(prefix):
            # Extract version from process name or cmdline[0] binary path.
            # Process name is often "python3" but cmdline[0] may be
            # "/usr/bin/python3.14" which has the full version.
            version = _extract_version(lower, prefix)
            if not version and cmdline:
                binary = cmdline[0].rsplit("/", 1)[-1].lower()
                version = _extract_version(binary, prefix)
            label = f"{lbl} {version}" if version else lbl
            break
    is_java = lower == "java"
    if is_java:
        label = "java"

    if not label or not cmdline or len(cmdline) < 2:
        return name, ""

    # Walk args looking for the first meaningful argument
    skip_next = False
    for i, arg in enumerate(cmdline[1:], 1):
        if skip_next:
            skip_next = False
            continue
        # Skip flags
        if arg.startswith("-"):
            # python -m module_name → use module_name
            if arg == "-m" and i + 1 < len(cmdline):
                return cmdline[i + 1].rsplit(".", 1)[-1], label
            # python -c "code" → check for multiprocessing workers
            if arg == "-c" and i + 1 < len(cmdline):
                code = cmdline[i + 1]
                if "spawn_main" in code or "forkserver" in code:
                    # Try: parent process name, then binary path, then generic
                    friendly = (
                        _get_parent_friendly_name(ppid)
                        or _name_from_binary_path(cmdline[0])
                        or "worker"
                    )
                    return friendly, label
                return _name_from_binary_path(cmdline[0]) or name, label
            if arg == "-c":
                return _name_from_binary_path(cmdline[0]) or name, label
            # Flags that consume the next arg (e.g. -W, -X, -Q)
            if len(arg) == 2 and arg[1].isalpha():
                skip_next = True
            continue
        # Skip empty args
        if not arg:
            continue
        # For Java, look for the main class (last dotted component)
        if is_java:
            if arg.endswith(".jar"):
                return arg.rsplit("/", 1)[-1], label
            # Main class like com.example.App → App
            if "." in arg and not arg.startswith("/"):
                return arg.rsplit(".", 1)[-1], label
            continue
        # For script interpreters, use the script filename
        return arg.rsplit("/", 1)[-1], label

    return name, ""


# ----------------------------------------------------------------------------------------
def get_top_processes(limit: int = 20) -> list[dict[str, Any]]:
    """Get top processes grouped by parent+name and sorted by resource usage.

    Processes spawned by the same parent with the same name are consolidated
    into a single entry (e.g. 10 stress workers become "stress (x10)").  Two
    different Python applications each spawning workers will remain separate
    because they have different parent PIDs.
    """
    # Collect raw process data including ppid for grouping
    raw: list[dict[str, Any]] = []

    for proc in psutil.process_iter(
        [
            "pid",
            "ppid",
            "name",
            "cmdline",
            "cpu_percent",
            "memory_percent",
            "memory_info",
            "username",
        ]
    ):
        try:
            info = proc.info  # pyright: ignore[reportAttributeAccessIssue]
            cpu = info.get("cpu_percent") or 0.0
            mem = info.get("memory_percent") or 0.0
            mem_info = info.get("memory_info")
            rss = mem_info.rss if mem_info else 0
            if cpu > 0 or mem > 0.1:
                proc_name, proc_tag = _friendly_name(
                    info.get("name", ""),
                    info.get("cmdline"),
                    info.get("ppid", 0),
                )
                raw.append(
                    {
                        "pid": info.get("pid", 0),
                        "ppid": info.get("ppid", 0),
                        "name": proc_name,
                        "tag": proc_tag,
                        "cpu_percent": float(cpu),
                        "memory_percent": float(mem),
                        "memory_bytes": rss,
                        "user": info.get("username", ""),
                    }
                )
        except (psutil.NoSuchProcess, psutil.AccessDenied, psutil.ZombieProcess):
            continue

    # Group by (parent_pid, name, tag) — siblings with the same name are
    # consolidated.  Each unique parent spawning the same binary gets its
    # own group, so two separate Python apps stay separate.
    groups: dict[tuple[int, str, str], list[dict[str, Any]]] = {}
    for p in raw:
        key = (p["ppid"], p["name"], p["tag"])
        groups.setdefault(key, []).append(p)

    procs: list[dict[str, Any]] = []
    for (_, name, tag), members in groups.items():
        total_cpu = sum(m["cpu_percent"] for m in members)
        total_mem_pct = sum(m["memory_percent"] for m in members)
        total_rss = sum(m["memory_bytes"] for m in members)
        count = len(members)
        display_name = f"{name} (x{count})" if count > 1 else name
        procs.append(
            {
                "name": display_name,
                "tag": tag,
                "cpu_percent": round(total_cpu, 1),
                "memory_percent": round(total_mem_pct, 1),
                "memory_bytes": total_rss,
                "user": members[0]["user"],
                "count": count,
            }
        )

    # Sort by combined resource usage: CPU weight + memory weight.
    # CPU is 0-N*100, memory is 0-100, so we normalise CPU to per-core.
    cores = psutil.cpu_count(logical=True) or 1
    procs.sort(
        key=lambda p: (p["cpu_percent"] / cores) + p["memory_percent"],
        reverse=True,
    )
    return procs[:limit]


# ----------------------------------------------------------------------------------------
#   Docker Information
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
def _docker_available() -> bool:
    """Check if Docker CLI is available."""
    return shutil.which("docker") is not None


# ----------------------------------------------------------------------------------------
def _get_container_processes(container_id: str) -> list[dict[str, str]]:
    """Get the processes running inside a Docker container via docker top."""
    try:
        result = subprocess.run(
            ["docker", "top", container_id, "-o", "pid,comm"],
            capture_output=True,
            text=True,
            timeout=5,
        )
        if result.returncode != 0:
            return []

        procs: list[dict[str, str]] = []
        lines = result.stdout.strip().split("\n")
        # Skip header line
        for line in lines[1:]:
            parts = line.split(None, 1)
            if len(parts) >= 2:
                procs.append({"pid": parts[0], "name": parts[1]})
        return procs
    except (subprocess.TimeoutExpired, FileNotFoundError, OSError):
        return []


# ----------------------------------------------------------------------------------------
def get_docker_containers() -> list[dict[str, Any]]:
    """Get running Docker containers with stats."""
    if not _docker_available():
        return []

    try:
        # Get container list
        result = subprocess.run(
            [
                "docker",
                "ps",
                "--format",
                '{"id":"{{.ID}}","name":"{{.Names}}","image":"{{.Image}}",'
                '"status":"{{.Status}}","ports":"{{.Ports}}"}',
            ],
            capture_output=True,
            text=True,
            timeout=5,
        )
        if result.returncode != 0:
            return []

        containers: list[dict[str, Any]] = []
        for line in result.stdout.strip().split("\n"):
            if not line:
                continue
            try:
                containers.append(json.loads(line))
            except json.JSONDecodeError:
                continue

        if not containers:
            return []

        # Get stats for running containers
        ids = [c["id"] for c in containers]
        stats_result = subprocess.run(
            [
                "docker",
                "stats",
                "--no-stream",
                "--format",
                '{"id":"{{.ID}}","cpu":"{{.CPUPerc}}","memory":"{{.MemUsage}}",'
                '"mem_percent":"{{.MemPerc}}","net_io":"{{.NetIO}}"}',
                *ids,
            ],
            capture_output=True,
            text=True,
            timeout=10,
        )

        stats_map: dict[str, dict[str, Any]] = {}
        if stats_result.returncode == 0:
            for line in stats_result.stdout.strip().split("\n"):
                if not line:
                    continue
                try:
                    stat = json.loads(line)
                    stats_map[stat["id"]] = stat
                except json.JSONDecodeError:
                    continue

        # Merge stats and internal processes into containers
        for container in containers:
            stat = stats_map.get(container["id"], {})
            container["cpu"] = stat.get("cpu", "0%")
            container["memory"] = stat.get("memory", "N/A")
            container["mem_percent"] = stat.get("mem_percent", "0%")
            container["net_io"] = stat.get("net_io", "N/A")
            container["processes"] = _get_container_processes(container["id"])

        return containers

    except (subprocess.TimeoutExpired, FileNotFoundError, OSError):
        return []


# ----------------------------------------------------------------------------------------
#   Network Information
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
def get_network_connections() -> list[dict[str, Any]]:
    """Get listening network connections with associated process info."""
    # psutil.net_connections() requires root on macOS, so we fall back to
    # lsof which works without elevated privileges.
    connections: list[dict[str, Any]] = []
    seen: set[int] = set()

    try:
        result = subprocess.run(
            ["lsof", "-iTCP", "-sTCP:LISTEN", "-nP", "-F", "pcn"],
            capture_output=True,
            text=True,
            timeout=10,
        )
        if result.returncode != 0:
            return []

        pid = 0
        proc_name = ""
        for line in result.stdout.strip().split("\n"):
            if not line:
                continue
            field_type = line[0]
            value = line[1:]
            if field_type == "p":
                pid = int(value)
                proc_name = ""
            elif field_type == "c":
                proc_name = value
            elif field_type == "n":
                # value is like "host:port" or "*:port"
                if ":" not in value:
                    continue
                host_part, _, port_str = value.rpartition(":")
                try:
                    port = int(port_str)
                except ValueError:
                    continue

                # Skip localhost-only bindings
                if host_part in ("127.0.0.1", "[::1]", "localhost"):
                    continue

                # Deduplicate by port
                if port in seen:
                    continue
                seen.add(port)

                connections.append(
                    {
                        "local_address": value,
                        "port": port,
                        "pid": pid,
                        "process": proc_name,
                        "status": "LISTEN",
                    }
                )
    except (subprocess.TimeoutExpired, FileNotFoundError, OSError):
        pass

    connections.sort(key=lambda c: c["port"])
    return connections


# ----------------------------------------------------------------------------------------
#   Full Snapshot
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
def get_snapshot() -> SystemSnapshot:
    """Collect a full system snapshot for the dashboard."""
    return {
        "timestamp": time.time(),
        "machine": get_machine_info(),
        "cpu": get_cpu_info(),
        "memory": get_memory_info(),
        "processes": get_top_processes(),
        "docker": get_docker_containers(),
        "network": get_network_connections(),
    }


# ----------------------------------------------------------------------------------------
#   Initialisation
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
def initialise() -> None:
    """Initialise CPU percent tracking (first call always returns 0)."""
    psutil.cpu_percent(interval=None, percpu=True)
