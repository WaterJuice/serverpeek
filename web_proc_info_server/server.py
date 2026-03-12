# ----------------------------------------------------------------------------------------
#   server.py
#   ---------
#
#   HTTP server with Server-Sent Events (SSE) for live system monitoring.
#   Serves the dashboard HTML and streams system snapshots to connected clients.
#   A single background thread collects system data; all SSE clients share the
#   same snapshot so resource usage does not increase with more connections.
#   The collector sleeps when no clients are connected and wakes when one arrives.
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

import collections
import json
import pathlib
import threading
import time
from http.server import BaseHTTPRequestHandler
from http.server import HTTPServer
from web_proc_info_server.system_info import get_snapshot
from web_proc_info_server.system_info import initialise

# ----------------------------------------------------------------------------------------
#   Constants
# ----------------------------------------------------------------------------------------

_WEB_DIR = pathlib.Path(__file__).parent / "web"
_UPDATE_INTERVAL = 2.0
_HISTORY_MAX = 60

# ----------------------------------------------------------------------------------------
#   Snapshot Collector
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
class _SnapshotCollector:
    """Collects system snapshots on a background thread.

    All SSE clients read from the same shared snapshot, so the server's
    resource usage is constant regardless of how many clients are connected.
    The collector sleeps when no clients are connected and automatically
    wakes when a new client arrives.  A ring buffer of recent snapshots
    lets new clients receive full graph history immediately.
    """

    def __init__(self) -> None:
        self._lock = threading.Lock()
        self._history: collections.deque[str] = collections.deque(maxlen=_HISTORY_MAX)
        self._latest_json: str = "{}"
        self._update_event = threading.Event()
        self._clients = 0
        self._wake_event = threading.Event()

    # ------------------------------------------------------------------------------------
    def start(self) -> None:
        """Start the background collection thread."""
        thread = threading.Thread(target=self._run, daemon=True)
        thread.start()

    # ------------------------------------------------------------------------------------
    def _run(self) -> None:
        """Continuously collect snapshots while clients are connected."""
        while True:
            # Sleep until at least one client is connected
            self._wake_event.wait()

            snapshot = get_snapshot()
            json_data = json.dumps(snapshot)
            with self._lock:
                self._history.append(json_data)
                self._latest_json = json_data
            # Wake all SSE clients waiting for an update
            self._update_event.set()
            self._update_event.clear()
            time.sleep(_UPDATE_INTERVAL)

    # ------------------------------------------------------------------------------------
    def client_connect(self) -> None:
        """Register a new SSE client — wakes the collector if sleeping."""
        with self._lock:
            self._clients += 1
        self._wake_event.set()

    # ------------------------------------------------------------------------------------
    def client_disconnect(self) -> None:
        """Unregister an SSE client — collector sleeps when count hits zero."""
        with self._lock:
            self._clients = max(0, self._clients - 1)
            if self._clients == 0:
                self._wake_event.clear()

    # ------------------------------------------------------------------------------------
    def get_json(self) -> str:
        """Get the latest snapshot as a pre-serialised JSON string."""
        with self._lock:
            return self._latest_json

    # ------------------------------------------------------------------------------------
    def get_history(self) -> list[str]:
        """Get the full history buffer as a list of JSON strings."""
        with self._lock:
            return list(self._history)

    # ------------------------------------------------------------------------------------
    def wait_for_update(self, timeout: float = 5.0) -> bool:
        """Block until a new snapshot is available. Returns False on timeout."""
        return self._update_event.wait(timeout=timeout)


# Module-level collector instance, set by run_server().
_collector: _SnapshotCollector | None = None

# ----------------------------------------------------------------------------------------
#   Request Handler
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
class _DashboardHandler(BaseHTTPRequestHandler):
    """HTTP request handler for the monitoring dashboard."""

    # ------------------------------------------------------------------------------------
    def do_GET(self) -> None:
        """Handle GET requests."""
        if self.path == "/" or self.path == "/index.html":
            self._serve_html()
        elif self.path == "/api/snapshot":
            self._serve_snapshot()
        elif self.path == "/api/stream":
            self._serve_sse()
        else:
            self.send_error(404)

    # ------------------------------------------------------------------------------------
    def _serve_html(self) -> None:
        """Serve the dashboard HTML page."""
        html_path = _WEB_DIR / "index.html"
        if not html_path.exists():
            self.send_error(500, "Dashboard HTML not found")
            return

        content = html_path.read_bytes()
        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(content)))
        self.send_header("Cache-Control", "no-cache, no-store, must-revalidate")
        self.end_headers()
        self.wfile.write(content)

    # ------------------------------------------------------------------------------------
    def _serve_snapshot(self) -> None:
        """Serve a single JSON snapshot of system state."""
        assert _collector is not None
        data = _collector.get_json().encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.send_header("Access-Control-Allow-Origin", "*")
        self.end_headers()
        self.wfile.write(data)

    # ------------------------------------------------------------------------------------
    def _serve_sse(self) -> None:
        """Stream system snapshots via Server-Sent Events.

        On connect, sends the full snapshot history so graphs are populated
        immediately.  Then streams live updates as they arrive.
        """
        assert _collector is not None
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.send_header("Connection", "keep-alive")
        self.send_header("Access-Control-Allow-Origin", "*")
        self.end_headers()

        _collector.client_connect()
        try:
            # Send history backlog so new clients get full graphs
            history = _collector.get_history()
            if history:
                history_payload = "[" + ",".join(history) + "]"
                self.wfile.write(
                    f"event: history\ndata: {history_payload}\n\n".encode()
                )
                self.wfile.flush()

            # Stream live updates
            while True:
                _collector.wait_for_update()
                data = _collector.get_json()
                message = f"data: {data}\n\n"
                self.wfile.write(message.encode("utf-8"))
                self.wfile.flush()
        except (BrokenPipeError, ConnectionResetError, OSError):
            pass
        finally:
            _collector.client_disconnect()

    # ------------------------------------------------------------------------------------
    def log_message(self, format: str, *args: object) -> None:  # noqa: A002
        """Suppress noisy request logging — only log errors."""
        # Suppress SSE and normal request logging entirely
        if len(args) >= 1 and isinstance(args[0], str):
            request_line: str = args[0]
            if "/api/stream" in request_line or "/api/snapshot" in request_line:
                return
        super().log_message(format, *args)

    # ------------------------------------------------------------------------------------
    def log_error(self, format: str, *args: object) -> None:  # noqa: A002
        """Suppress connection reset errors."""
        msg = format % args if args else format
        if "ConnectionResetError" in msg or "BrokenPipeError" in msg:
            return
        super().log_error(format, *args)


# ----------------------------------------------------------------------------------------
#   Server
# ----------------------------------------------------------------------------------------


# ----------------------------------------------------------------------------------------
class _ThreadedHTTPServer(HTTPServer):
    """HTTP server that handles each request in a separate thread."""

    daemon_threads = True
    allow_reuse_address = True

    # ------------------------------------------------------------------------------------
    def process_request(self, request: object, client_address: tuple[str, int]) -> None:
        """Start a new thread to process each request."""
        thread = threading.Thread(
            target=self.process_request_thread,
            args=(request, client_address),
            daemon=True,
        )
        thread.start()

    # ------------------------------------------------------------------------------------
    def process_request_thread(
        self,
        request: object,
        client_address: tuple[str, int],
    ) -> None:
        """Process a request in a worker thread."""
        try:
            self.finish_request(request, client_address)  # pyright: ignore[reportArgumentType]
        except (BrokenPipeError, ConnectionResetError, ConnectionAbortedError, OSError):
            pass
        except Exception:
            self.handle_error(request, client_address)  # pyright: ignore[reportArgumentType]
        finally:
            self.shutdown_request(request)  # pyright: ignore[reportArgumentType]


# ----------------------------------------------------------------------------------------
def run_server(*, host: str = "0.0.0.0", port: int = 8080) -> None:
    """Start the monitoring web server."""
    global _collector  # noqa: PLW0603

    # Initialise CPU tracking (first call always returns 0)
    initialise()

    # Brief pause to let psutil collect initial CPU data
    time.sleep(0.5)

    # Start shared snapshot collector (sleeps until first client connects)
    _collector = _SnapshotCollector()
    _collector.start()

    server = _ThreadedHTTPServer((host, port), _DashboardHandler)
    server.serve_forever()
