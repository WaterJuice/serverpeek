// ---------------------------------------------------------------------------------------
//
//	server.go
//	---------
//
//	HTTP server with Server-Sent Events (SSE) for live system monitoring.
//	Serves the dashboard HTML and streams system snapshots to connected clients.
//	A single background goroutine collects system data; all SSE clients share the
//	same snapshot so resource usage does not increase with more connections.
//	The collector sleeps when no clients are connected and wakes when one arrives.
//
//	Endpoints:
//	  GET /              — dashboard HTML page (embedded via go:embed)
//	  GET /api/snapshot  — single JSON snapshot of current system state
//	  GET /api/stream    — SSE stream, pushes data every 2 seconds
//
//	(c) 2026 WaterJuice — Released under the Unlicense; see LICENSE.
//
//	Version History
//	---------------
//	Mar 2026 - Created (Python)
//	Mar 2026 - Rewritten in Go
//
// ---------------------------------------------------------------------------------------
package internal

// ---------------------------------------------------------------------------------------
//
//	Imports
//
// ---------------------------------------------------------------------------------------

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ---------------------------------------------------------------------------------------
//
//	Embedded Static Files
//
// ---------------------------------------------------------------------------------------

//go:embed static/index.html
var indexHTML string

// ---------------------------------------------------------------------------------------
//
//	Constants
//
// ---------------------------------------------------------------------------------------

const (
	updateInterval = 2 * time.Second
	historyMax     = 60
	lingerDuration = 60 * time.Second
)

// ---------------------------------------------------------------------------------------
//
//	Snapshot Collector
//
// ---------------------------------------------------------------------------------------

// snapshotCollector collects system snapshots on a background goroutine.
// All SSE clients read from the same shared snapshot, so the server's
// resource usage is constant regardless of how many clients are connected.
type snapshotCollector struct {
	mu          sync.Mutex
	history     []string
	latestJSON  string
	updateCh    chan struct{} // guarded by mu
	clients     int
	wakeCh      chan struct{}
	lingerSince time.Time
	lingering   bool
	wasSleeping bool
	isTTY       bool
}

// ---------------------------------------------------------------------------------------
// newSnapshotCollector creates a new collector.
func newSnapshotCollector(isTTY bool) *snapshotCollector {
	return &snapshotCollector{
		latestJSON:  "{}",
		updateCh:    make(chan struct{}),
		wakeCh:      make(chan struct{}, 1),
		wasSleeping: true,
		isTTY:       isTTY,
	}
}

// ---------------------------------------------------------------------------------------
// start begins the background collection goroutine.
func (c *snapshotCollector) start() {
	go c.run()
}

// ---------------------------------------------------------------------------------------
// run continuously collects snapshots while clients are connected.
func (c *snapshotCollector) run() {
	for {
		<-c.wakeCh

		snapshot := GetSnapshot()
		jsonData, err := json.Marshal(snapshot)
		if err != nil {
			logInfo(c.isTTY, "Failed to marshal snapshot: %s", err)
			time.Sleep(updateInterval)
			continue
		}
		jsonStr := string(jsonData)

		c.mu.Lock()
		c.history = append(c.history, jsonStr)
		if len(c.history) > historyMax {
			// Copy to a fresh slice so the old backing array can be GC'd
			fresh := make([]string, historyMax)
			copy(fresh, c.history[len(c.history)-historyMax:])
			c.history = fresh
		}
		c.latestJSON = jsonStr

		// Check if we are lingering with no clients
		if c.clients == 0 && c.lingering {
			if time.Since(c.lingerSince) >= lingerDuration {
				c.history = nil
				c.latestJSON = "{}"
				c.lingering = false
				c.wasSleeping = true
				logInfo(c.isTTY, "Linger expired — collector sleeping")
				c.mu.Unlock()
				continue
			}
		}

		// Notify waiting SSE clients (swap channel under lock to avoid race)
		oldCh := c.updateCh
		c.updateCh = make(chan struct{})
		c.mu.Unlock()
		close(oldCh)

		time.Sleep(updateInterval)

		c.mu.Lock()
		shouldWake := c.clients > 0 || c.lingering
		c.mu.Unlock()
		if shouldWake {
			select {
			case c.wakeCh <- struct{}{}:
			default:
			}
		}
	}
}

// ---------------------------------------------------------------------------------------
// clientConnect registers a new SSE client — wakes the collector if sleeping.
func (c *snapshotCollector) clientConnect() {
	c.mu.Lock()
	c.clients++
	c.lingering = false
	if c.wasSleeping {
		c.history = nil
		c.latestJSON = "{}"
		c.wasSleeping = false
	}
	count := c.clients
	c.mu.Unlock()

	// Wake collector
	select {
	case c.wakeCh <- struct{}{}:
	default:
	}

	logInfo(c.isTTY, "Client connected (%d active)", count)
}

// ---------------------------------------------------------------------------------------
// clientDisconnect unregisters an SSE client — collector lingers when count hits zero.
func (c *snapshotCollector) clientDisconnect() {
	c.mu.Lock()
	c.clients--
	if c.clients < 0 {
		c.clients = 0
	}
	count := c.clients
	if count == 0 {
		c.lingering = true
		c.lingerSince = time.Now()
	}
	c.mu.Unlock()

	if count == 0 {
		logInfo(c.isTTY, "All clients disconnected — lingering for 60s before sleeping")
	} else {
		logInfo(c.isTTY, "Client disconnected (%d active)", count)
	}
}

// ---------------------------------------------------------------------------------------
// getJSON returns the latest snapshot as a pre-serialised JSON string.
func (c *snapshotCollector) getJSON() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.latestJSON
}

// ---------------------------------------------------------------------------------------
// getHistory returns the full history buffer as a list of JSON strings.
func (c *snapshotCollector) getHistory() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]string, len(c.history))
	copy(result, c.history)
	return result
}

// ---------------------------------------------------------------------------------------
// waitForUpdate returns a channel that will be closed when a new snapshot is available.
func (c *snapshotCollector) waitForUpdate() <-chan struct{} {
	c.mu.Lock()
	ch := c.updateCh
	c.mu.Unlock()
	return ch
}

// ---------------------------------------------------------------------------------------
//
//	HTTP Handler
//
// ---------------------------------------------------------------------------------------

// dashboardServer is the HTTP handler for the monitoring dashboard.
type dashboardServer struct {
	collector    *snapshotCollector
	renderedHTML string
}

// ---------------------------------------------------------------------------------------
// ServeHTTP routes requests to the appropriate handler.
func (s *dashboardServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimRight(r.URL.Path, "/")
	if path == "" {
		path = "/"
	}

	switch {
	case r.Method == "GET" && (path == "/" || path == "/index.html"):
		s.handleIndex(w)
	case r.Method == "GET" && path == "/api/snapshot":
		s.handleSnapshot(w)
	case r.Method == "GET" && path == "/api/stream":
		s.handleSSE(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("404 Not Found"))
	}
}

// ---------------------------------------------------------------------------------------
// handleIndex serves the dashboard HTML page.
func (s *dashboardServer) handleIndex(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Write([]byte(s.renderedHTML))
}

// ---------------------------------------------------------------------------------------
// handleSnapshot serves a single JSON snapshot of system state.
func (s *dashboardServer) handleSnapshot(w http.ResponseWriter) {
	data := s.collector.getJSON()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write([]byte(data))
}

// ---------------------------------------------------------------------------------------
// handleSSE streams system snapshots via Server-Sent Events.
// On connect, sends the full snapshot history so graphs are populated immediately.
// Then streams live updates as they arrive.
func (s *dashboardServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	s.collector.clientConnect()
	defer s.collector.clientDisconnect()

	// Send history backlog so new clients get full graphs
	history := s.collector.getHistory()
	if len(history) > 0 {
		historyPayload := "[" + strings.Join(history, ",") + "]"
		fmt.Fprintf(w, "event: history\ndata: %s\n\n", historyPayload)
		flusher.Flush()
	}

	// Stream live updates
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.collector.waitForUpdate():
			data := s.collector.getJSON()
			_, err := fmt.Fprintf(w, "data: %s\n\n", data)
			if err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// ---------------------------------------------------------------------------------------
//
//	Server Entry Point
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// startServer creates and starts the monitoring HTTP server.
func startServer(addr string, version string, isTTY bool) {
	// Initialise CPU tracking
	Initialise()

	// Brief pause to let initial CPU data settle
	time.Sleep(500 * time.Millisecond)

	// Start shared snapshot collector
	collector := newSnapshotCollector(isTTY)
	collector.start()

	srv := &dashboardServer{
		collector:    collector,
		renderedHTML: strings.Replace(indexHTML, "{{VERSION}}", html.EscapeString(version), 1),
	}

	httpServer := &http.Server{
		Addr:    addr,
		Handler: srv,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		logInfo(isTTY, "Shutting down...")
		httpServer.Close()
	}()

	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "Server error: %s\n", err)
		os.Exit(1)
	}
}
