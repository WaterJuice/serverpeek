// ---------------------------------------------------------------------------------------
//
//	sysinfo.go
//	----------
//
//	System information gathering — CPU, memory, processes, Docker containers,
//	and network connections. Uses only the Go standard library: /proc on Linux,
//	sysctl/vm_stat/ps on macOS, and subprocess calls for Docker and lsof.
//
//	(c) 2026 WaterJuice — Released under the Unlicense; see LICENSE.
//
//	Version History
//	---------------
//	Mar 2026 - Created (Python with psutil)
//	Mar 2026 - Rewritten in Go (stdlib only)
//
// ---------------------------------------------------------------------------------------
package internal

// ---------------------------------------------------------------------------------------
//
//	Imports
//
// ---------------------------------------------------------------------------------------

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------------------
//
//	Types
//
// ---------------------------------------------------------------------------------------

// Snapshot represents a complete system state at a point in time.
type Snapshot struct {
	Timestamp float64               `json:"timestamp"`
	Machine   MachineInfo           `json:"machine"`
	CPU       CPUInfo               `json:"cpu"`
	Memory    MemoryInfo            `json:"memory"`
	Processes []ProcessInfo         `json:"processes"`
	Docker    []DockerContainerInfo `json:"docker"`
	Network   []NetworkConnection   `json:"network"`
}

// MachineInfo holds static machine and OS information.
type MachineInfo struct {
	Hostname         string  `json:"hostname"`
	Platform         string  `json:"platform"`
	OSVersion        string  `json:"os_version"`
	OSRelease        string  `json:"os_release"`
	Architecture     string  `json:"architecture"`
	CPUModel         string  `json:"cpu_model"`
	CPUCoresPhysical int     `json:"cpu_cores_physical"`
	CPUCoresLogical  int     `json:"cpu_cores_logical"`
	TotalMemoryBytes uint64  `json:"total_memory_bytes"`
	TotalMemoryGB    float64 `json:"total_memory_gb"`
	Uptime           string  `json:"uptime"`
	UptimeSeconds    int64   `json:"uptime_seconds"`
}

// CPUInfo holds current CPU usage data.
type CPUInfo struct {
	Overall     float64     `json:"overall"`
	PerCore     []float64   `json:"per_core"`
	LoadAverage LoadAverage `json:"load_average"`
}

// LoadAverage holds system load averages.
type LoadAverage struct {
	OneMin     float64 `json:"1min"`
	FiveMin    float64 `json:"5min"`
	FifteenMin float64 `json:"15min"`
}

// MemoryInfo holds current memory and swap usage.
type MemoryInfo struct {
	Total       uint64  `json:"total"`
	Available   uint64  `json:"available"`
	Used        uint64  `json:"used"`
	Percent     float64 `json:"percent"`
	Active      uint64  `json:"active"`
	Wired       uint64  `json:"wired"`
	Compressed  uint64  `json:"compressed"`
	SwapTotal   uint64  `json:"swap_total"`
	SwapUsed    uint64  `json:"swap_used"`
	SwapPercent float64 `json:"swap_percent"`
}

// ProcessInfo holds information about a process or process group.
type ProcessInfo struct {
	Name          string  `json:"name"`
	Tag           string  `json:"tag,omitempty"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	MemoryBytes   uint64  `json:"memory_bytes"`
	User          string  `json:"user"`
	Count         int     `json:"count"`
}

// DockerContainerInfo holds information about a running Docker container.
type DockerContainerInfo struct {
	ID         string              `json:"id"`
	Name       string              `json:"name"`
	Image      string              `json:"image"`
	Status     string              `json:"status"`
	Ports      string              `json:"ports"`
	CPU        string              `json:"cpu"`
	Memory     string              `json:"memory"`
	MemPercent string              `json:"mem_percent"`
	NetIO      string              `json:"net_io"`
	Processes  []DockerProcessInfo `json:"processes"`
}

// DockerProcessInfo holds a process inside a Docker container.
type DockerProcessInfo struct {
	PID  string `json:"pid"`
	Name string `json:"name"`
}

// NetworkConnection holds an open network port.
type NetworkConnection struct {
	LocalAddress string `json:"local_address"`
	Port         int    `json:"port"`
	PID          int    `json:"pid"`
	Process      string `json:"process"`
	Status       string `json:"status"`
}

// ---------------------------------------------------------------------------------------
//
//	Module State
//
// ---------------------------------------------------------------------------------------

var (
	bootTime     time.Time
	bootTimeOnce sync.Once

	// Previous CPU times for delta calculation
	prevCPUTimes     []cpuTime
	prevCPUTimesLock sync.Mutex

	// Cached static machine info (computed once at startup)
	cachedMachineInfo     MachineInfo
	cachedMachineInfoOnce sync.Once

	// Cached total memory (never changes while running)
	cachedTotalMem     uint64
	cachedTotalMemOnce sync.Once

	// Cached docker availability
	dockerAvailable     bool
	dockerAvailableOnce sync.Once
)

type cpuTime struct {
	user   uint64
	nice   uint64
	system uint64
	idle   uint64
}

// ---------------------------------------------------------------------------------------
//
//	Initialisation
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// Initialise sets up CPU tracking state. Must be called once before GetSnapshot.
func Initialise() {
	bootTimeOnce.Do(func() {
		bootTime = getBootTime()
	})

	// Prime CPU times so the first real reading has a delta.
	// readCPUTimes on macOS reads prevCPUTimes internally, so we must not
	// hold prevCPUTimesLock here to avoid deadlock.
	times := readCPUTimes()
	prevCPUTimesLock.Lock()
	prevCPUTimes = times
	prevCPUTimesLock.Unlock()
}

// ---------------------------------------------------------------------------------------
//
//	Snapshot Collection
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// GetSnapshot collects a full system snapshot for the dashboard.
func GetSnapshot() Snapshot {
	return Snapshot{
		Timestamp: float64(time.Now().UnixMilli()) / 1000.0,
		Machine:   getMachineInfo(),
		CPU:       getCPUInfo(),
		Memory:    getMemoryInfoPlatform(),
		Processes: getTopProcesses(20),
		Docker:    getDockerContainers(),
		Network:   getNetworkConnections(),
	}
}

// ---------------------------------------------------------------------------------------
//
//	Machine Information
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// getMachineInfo returns machine info, computing static fields only once.
func getMachineInfo() MachineInfo {
	cachedMachineInfoOnce.Do(func() {
		hostname, _ := os.Hostname()
		hostname = stripDomain(hostname)

		info := MachineInfo{
			Hostname:         hostname,
			Platform:         runtime.GOOS,
			Architecture:     runtime.GOARCH,
			CPUModel:         getCPUModel(),
			CPUCoresLogical:  runtime.NumCPU(),
			CPUCoresPhysical: getPhysicalCores(),
			TotalMemoryBytes: getCachedTotalMemory(),
			TotalMemoryGB:    math.Round(float64(getCachedTotalMemory())/(1024*1024*1024)*10) / 10,
		}

		info.OSVersion, info.OSRelease = getOSInfo()

		// Normalise platform name to match Python's platform.system()
		switch info.Platform {
		case "darwin":
			info.Platform = "Darwin"
		case "linux":
			info.Platform = "Linux"
		case "windows":
			info.Platform = "Windows"
		}

		// Normalise architecture to match Python's platform.uname().machine
		switch info.Architecture {
		case "amd64":
			info.Architecture = "x86_64"
		case "arm64":
			if runtime.GOOS != "darwin" {
				info.Architecture = "aarch64"
			}
		}

		cachedMachineInfo = info
	})

	// Only uptime is dynamic — copy the cached struct and update it
	info := cachedMachineInfo
	uptimeSeconds := int64(time.Since(bootTime).Seconds())
	info.Uptime = formatUptime(uptimeSeconds)
	info.UptimeSeconds = uptimeSeconds
	return info
}

// ---------------------------------------------------------------------------------------
// formatUptime formats seconds into a human-readable uptime string.
func formatUptime(seconds int64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	parts = append(parts, fmt.Sprintf("%dm", minutes))
	return strings.Join(parts, " ")
}

// ---------------------------------------------------------------------------------------
// getCachedTotalMemory returns total memory, reading it only once.
func getCachedTotalMemory() uint64 {
	cachedTotalMemOnce.Do(func() {
		cachedTotalMem = getTotalMemory()
	})
	return cachedTotalMem
}

// ---------------------------------------------------------------------------------------
//
//	CPU Information
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// getCPUInfo returns current CPU usage — overall and per-core.
func getCPUInfo() CPUInfo {
	current := readCPUTimes()

	prevCPUTimesLock.Lock()
	prev := prevCPUTimes
	prevCPUTimes = current
	prevCPUTimesLock.Unlock()

	perCore := make([]float64, len(current))
	var totalUsed, totalAll float64

	for i := range current {
		if i >= len(prev) {
			break
		}
		dUser := current[i].user - prev[i].user
		dNice := current[i].nice - prev[i].nice
		dSystem := current[i].system - prev[i].system
		dIdle := current[i].idle - prev[i].idle

		total := float64(dUser + dNice + dSystem + dIdle)
		used := float64(dUser + dNice + dSystem)

		if total > 0 {
			perCore[i] = math.Round(used/total*1000) / 10
		}
		totalUsed += used
		totalAll += total
	}

	overall := 0.0
	if totalAll > 0 {
		overall = math.Round(totalUsed/totalAll*1000) / 10
	}

	load1, load5, load15 := getLoadAverage()

	return CPUInfo{
		Overall: overall,
		PerCore: perCore,
		LoadAverage: LoadAverage{
			OneMin:     load1,
			FiveMin:    load5,
			FifteenMin: load15,
		},
	}
}

// ---------------------------------------------------------------------------------------
//
//	Process Information
//
// ---------------------------------------------------------------------------------------

// rawProcess holds raw process data before grouping.
type rawProcess struct {
	pid         int
	ppid        int
	name        string
	cmdline     string
	cpuPercent  float64
	memPercent  float64
	memoryBytes uint64
	user        string
}

// ---------------------------------------------------------------------------------------
// getTopProcesses returns the top processes grouped and sorted by resource usage.
func getTopProcesses(limit int) []ProcessInfo {
	raw := readProcessList()

	// Derive friendly names and tags
	type namedProc struct {
		rawProcess
		displayName string
		tag         string
	}

	named := make([]namedProc, 0, len(raw))
	for _, p := range raw {
		if p.cpuPercent <= 0 && p.memPercent <= 0.1 {
			continue
		}
		dname, tag := friendlyName(p.name, p.cmdline, p.ppid)
		named = append(named, namedProc{
			rawProcess:  p,
			displayName: dname,
			tag:         tag,
		})
	}

	// Group by (ppid, displayName, tag)
	type groupKey struct {
		ppid int
		name string
		tag  string
	}
	groups := make(map[groupKey][]namedProc)
	for _, p := range named {
		key := groupKey{p.ppid, p.displayName, p.tag}
		groups[key] = append(groups[key], p)
	}

	procs := make([]ProcessInfo, 0, len(groups))
	for key, members := range groups {
		var totalCPU, totalMemPct float64
		var totalRSS uint64
		for _, m := range members {
			totalCPU += m.cpuPercent
			totalMemPct += m.memPercent
			totalRSS += m.memoryBytes
		}
		count := len(members)
		displayName := key.name
		if count > 1 {
			displayName = fmt.Sprintf("%s (x%d)", key.name, count)
		}
		procs = append(procs, ProcessInfo{
			Name:          displayName,
			Tag:           key.tag,
			CPUPercent:    roundTo1(totalCPU),
			MemoryPercent: roundTo1(totalMemPct),
			MemoryBytes:   totalRSS,
			User:          members[0].user,
			Count:         count,
		})
	}

	// Sort by combined resource usage
	cores := float64(runtime.NumCPU())
	if cores < 1 {
		cores = 1
	}
	sort.Slice(procs, func(i, j int) bool {
		scoreI := procs[i].CPUPercent/cores + procs[i].MemoryPercent
		scoreJ := procs[j].CPUPercent/cores + procs[j].MemoryPercent
		return scoreI > scoreJ
	})

	if len(procs) > limit {
		procs = procs[:limit]
	}
	return procs
}

// ---------------------------------------------------------------------------------------
//
//	Friendly Process Names
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// friendlyName derives a friendly display name and runtime tag for a process.
func friendlyName(name string, cmdline string, ppid int) (string, string) {
	lower := strings.ToLower(name)

	interpreterLabels := map[string]string{
		"python": "python",
		"node":   "node",
		"ruby":   "ruby",
		"perl":   "perl",
	}

	label := ""
	for prefix, lbl := range interpreterLabels {
		if strings.HasPrefix(lower, prefix) {
			version := extractVersion(lower, prefix)
			if version != "" {
				label = lbl + " " + version
			} else {
				label = lbl
			}
			break
		}
	}
	if lower == "java" {
		label = "java"
	}

	if label == "" || cmdline == "" {
		return name, ""
	}

	// Parse cmdline into args
	args := splitCmdline(cmdline)
	if len(args) < 2 {
		return name, ""
	}

	// Walk args looking for the first meaningful argument
	skipNext := false
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if skipNext {
			skipNext = false
			continue
		}
		if strings.HasPrefix(arg, "-") {
			if arg == "-m" && i+1 < len(args) {
				parts := strings.Split(args[i+1], ".")
				return parts[len(parts)-1], label
			}
			if arg == "-c" && i+1 < len(args) {
				code := args[i+1]
				if strings.Contains(code, "spawn_main") || strings.Contains(code, "forkserver") {
					friendly := nameFromBinaryPath(args[0])
					if friendly == "" {
						friendly = "worker"
					}
					return friendly, label
				}
				fn := nameFromBinaryPath(args[0])
				if fn != "" {
					return fn, label
				}
				return name, label
			}
			if arg == "-c" {
				fn := nameFromBinaryPath(args[0])
				if fn != "" {
					return fn, label
				}
				return name, label
			}
			if len(arg) == 2 && arg[1] >= 'A' && arg[1] <= 'z' {
				skipNext = true
			}
			continue
		}
		if arg == "" {
			continue
		}
		if lower == "java" {
			if strings.HasSuffix(arg, ".jar") {
				parts := strings.Split(arg, "/")
				return parts[len(parts)-1], label
			}
			if strings.Contains(arg, ".") && !strings.HasPrefix(arg, "/") {
				parts := strings.Split(arg, ".")
				return parts[len(parts)-1], label
			}
			continue
		}
		// Script filename
		parts := strings.Split(arg, "/")
		return parts[len(parts)-1], label
	}

	return name, ""
}

// ---------------------------------------------------------------------------------------
// extractVersion extracts version suffix from a binary name like "python3.14" → "3.14".
func extractVersion(lower string, prefix string) string {
	suffix := lower[len(prefix):]
	if suffix != "" && suffix[0] >= '0' && suffix[0] <= '9' && strings.Contains(suffix, ".") {
		return suffix
	}
	return ""
}

// ---------------------------------------------------------------------------------------
// nameFromBinaryPath extracts a meaningful name from an interpreter binary path.
func nameFromBinaryPath(binary string) string {
	parts := strings.Split(strings.ReplaceAll(binary, "\\", "/"), "/")

	// uv tools pattern: .../uv/tools/<name>/bin/python
	toolsIdx := -1
	for i, p := range parts {
		if p == "tools" {
			toolsIdx = i
		}
	}
	if toolsIdx >= 0 {
		for i := toolsIdx + 1; i < len(parts); i++ {
			if parts[i] == "bin" && i == toolsIdx+2 {
				return parts[toolsIdx+1]
			}
		}
	}

	// virtualenv pattern: .../<name>/bin/python
	for i, p := range parts {
		if p == "bin" && i > 0 {
			candidate := parts[i-1]
			skip := map[string]bool{"venv": true, ".venv": true, "env": true, ".env": true, "bin": true}
			if !skip[candidate] {
				return candidate
			}
		}
	}

	return ""
}

// ---------------------------------------------------------------------------------------
// splitCmdline splits a command line string into arguments. On Linux /proc provides
// null-separated args; on macOS ps provides space-separated output.
func splitCmdline(cmdline string) []string {
	if strings.Contains(cmdline, "\x00") {
		parts := strings.Split(cmdline, "\x00")
		var result []string
		for _, p := range parts {
			if p != "" {
				result = append(result, p)
			}
		}
		return result
	}
	return strings.Fields(cmdline)
}

// ---------------------------------------------------------------------------------------
//
//	Docker Information
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// getDockerContainers returns running Docker containers with stats and internal processes.
func getDockerContainers() []DockerContainerInfo {
	dockerAvailableOnce.Do(func() {
		_, err := exec.LookPath("docker")
		dockerAvailable = err == nil
	})
	if !dockerAvailable {
		return nil
	}

	// Get container list
	out, err := runCommand("docker", "ps", "--format",
		`{"id":"{{.ID}}","name":"{{.Names}}","image":"{{.Image}}","status":"{{.Status}}","ports":"{{.Ports}}"}`)
	if err != nil {
		return nil
	}

	var containers []DockerContainerInfo
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var c DockerContainerInfo
		if json.Unmarshal([]byte(line), &c) == nil {
			containers = append(containers, c)
		}
	}

	if len(containers) == 0 {
		return nil
	}

	// Get stats
	ids := make([]string, len(containers))
	for i, c := range containers {
		ids[i] = c.ID
	}
	statsArgs := append([]string{"stats", "--no-stream", "--format",
		`{"id":"{{.ID}}","cpu":"{{.CPUPerc}}","memory":"{{.MemUsage}}","mem_percent":"{{.MemPerc}}","net_io":"{{.NetIO}}"}`},
		ids...)
	statsOut, err := runCommand("docker", statsArgs...)
	statsMap := make(map[string]map[string]string)
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(statsOut), "\n") {
			if line == "" {
				continue
			}
			var stat map[string]string
			if json.Unmarshal([]byte(line), &stat) == nil {
				statsMap[stat["id"]] = stat
			}
		}
	}

	// Merge stats and get internal processes
	for i := range containers {
		stat := statsMap[containers[i].ID]
		if stat != nil {
			containers[i].CPU = stat["cpu"]
			containers[i].Memory = stat["memory"]
			containers[i].MemPercent = stat["mem_percent"]
			containers[i].NetIO = stat["net_io"]
		} else {
			containers[i].CPU = "0%"
			containers[i].Memory = "N/A"
			containers[i].MemPercent = "0%"
			containers[i].NetIO = "N/A"
		}
		containers[i].Processes = getContainerProcesses(containers[i].ID)
	}

	return containers
}

// ---------------------------------------------------------------------------------------
// getContainerProcesses returns the processes running inside a Docker container.
func getContainerProcesses(containerID string) []DockerProcessInfo {
	out, err := runCommand("docker", "top", containerID, "-o", "pid,comm")
	if err != nil {
		return nil
	}

	var procs []DockerProcessInfo
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines[1:] {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			procs = append(procs, DockerProcessInfo{
				PID:  parts[0],
				Name: strings.Join(parts[1:], " "),
			})
		}
	}
	return procs
}

// ---------------------------------------------------------------------------------------
//
//	Network Information
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// getNetworkConnections returns listening network connections with process info.
func getNetworkConnections() []NetworkConnection {
	out, err := runCommand("lsof", "-iTCP", "-sTCP:LISTEN", "-nP", "-F", "pcn")
	if err != nil {
		return nil
	}

	var connections []NetworkConnection
	seen := make(map[int]bool)

	pid := 0
	procName := ""
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		fieldType := line[0]
		value := line[1:]
		switch fieldType {
		case 'p':
			pid, _ = strconv.Atoi(value)
			procName = ""
		case 'c':
			procName = value
		case 'n':
			if !strings.Contains(value, ":") {
				continue
			}
			lastColon := strings.LastIndex(value, ":")
			hostPart := value[:lastColon]
			portStr := value[lastColon+1:]
			port, err := strconv.Atoi(portStr)
			if err != nil {
				continue
			}

			// Skip localhost-only bindings
			if hostPart == "127.0.0.1" || hostPart == "[::1]" || hostPart == "localhost" {
				continue
			}

			if seen[port] {
				continue
			}
			seen[port] = true

			connections = append(connections, NetworkConnection{
				LocalAddress: value,
				Port:         port,
				PID:          pid,
				Process:      procName,
				Status:       "LISTEN",
			})
		}
	}

	sort.Slice(connections, func(i, j int) bool {
		return connections[i].Port < connections[j].Port
	})
	return connections
}

// ---------------------------------------------------------------------------------------
//
//	Helpers
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// runCommand executes a command with a 10-second timeout and returns its stdout output.
func runCommand(name string, args ...string) (string, error) {
	ctx, cancel := contextWithTimeout(10 * time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	return string(out), err
}

// ---------------------------------------------------------------------------------------
// roundTo1 rounds a float to 1 decimal place.
func roundTo1(f float64) float64 {
	return math.Round(f*10) / 10
}

// ---------------------------------------------------------------------------------------
// contextWithTimeout creates a context with the given timeout.
func contextWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

// ---------------------------------------------------------------------------------------
// stripDomain removes the domain suffix from a hostname (e.g. "host.local" → "host").
func stripDomain(hostname string) string {
	if dot := strings.IndexByte(hostname, '.'); dot >= 0 {
		return hostname[:dot]
	}
	return hostname
}
