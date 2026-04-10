// ---------------------------------------------------------------------------------------
//
//	sysinfo_linux.go
//	-----------------
//
//	Linux-specific system information gathering. Reads /proc for CPU times,
//	memory info, process lists, boot time, and load averages.
//
//	(c) 2026 WaterJuice — Released under the Unlicense; see LICENSE.
//
//	Version History
//	---------------
//	Mar 2026 - Created
//
// ---------------------------------------------------------------------------------------
package internal

// ---------------------------------------------------------------------------------------
//
//	Imports
//
// ---------------------------------------------------------------------------------------

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------------------
//
//	Boot Time
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// getBootTime reads the system boot time from /proc/stat.
func getBootTime() time.Time {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return time.Now()
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "btime ") {
			secs, _ := strconv.ParseInt(strings.TrimPrefix(line, "btime "), 10, 64)
			return time.Unix(secs, 0)
		}
	}
	return time.Now()
}

// ---------------------------------------------------------------------------------------
//
//	OS Information
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// getOSInfo returns the OS version string and a user-friendly distro release.
// Prefers /etc/os-release (e.g. "Ubuntu 24.04") and falls back to the kernel
// release from /proc/sys/kernel/osrelease when unavailable.
func getOSInfo() (version string, release string) {
	if data, err := os.ReadFile("/proc/version"); err == nil {
		version = strings.TrimSpace(string(data))
	}

	release = readDistroRelease()
	if release == "" {
		if unameData, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
			release = strings.TrimSpace(string(unameData))
		}
	}
	return
}

// ---------------------------------------------------------------------------------------
// readDistroRelease parses /etc/os-release and returns a short distro label
// like "Ubuntu 24.04". Returns an empty string if the file is missing or
// cannot be parsed into a meaningful name.
func readDistroRelease() string {
	// Try the standard locations in order.
	candidates := []string{"/etc/os-release", "/usr/lib/os-release"}
	var data []byte
	for _, path := range candidates {
		if d, err := os.ReadFile(path); err == nil {
			data = d
			break
		}
	}
	if data == nil {
		return ""
	}

	fields := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		val = strings.Trim(val, `"'`)
		fields[key] = val
	}

	name := fields["NAME"]
	versionID := fields["VERSION_ID"]
	if name != "" && versionID != "" {
		return name + " " + versionID
	}
	if pretty := fields["PRETTY_NAME"]; pretty != "" {
		return pretty
	}
	if name != "" {
		return name
	}
	return ""
}

// ---------------------------------------------------------------------------------------
//
//	CPU Information
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// getCPUModel returns the CPU model string from /proc/cpuinfo or lscpu.
func getCPUModel() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "model name") || strings.HasPrefix(line, "Hardware") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					v := strings.TrimSpace(parts[1])
					if v != "" {
						return v
					}
				}
			}
		}
	}

	out, err := runCommand("lscpu")
	if err == nil {
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "Model name:") {
				v := strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
				if v != "" && v != "-" {
					return v
				}
			}
		}
	}

	return "Unknown"
}

// ---------------------------------------------------------------------------------------
// getPhysicalCores returns the number of physical CPU cores.
func getPhysicalCores() int {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return 0
	}

	physicalIDs := make(map[string]bool)
	coresPerSocket := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "physical id") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				physicalIDs[strings.TrimSpace(parts[1])] = true
			}
		}
		if strings.HasPrefix(line, "cpu cores") && coresPerSocket == 0 {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				coresPerSocket, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
			}
		}
	}

	if len(physicalIDs) > 0 && coresPerSocket > 0 {
		return len(physicalIDs) * coresPerSocket
	}

	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "processor") {
			count++
		}
	}
	return count
}

// ---------------------------------------------------------------------------------------
// readCPUTimes reads per-core CPU times from /proc/stat.
func readCPUTimes() []cpuTime {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return nil
	}

	var times []cpuTime
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 || fields[0] == "cpu" {
			continue
		}

		user, _ := strconv.ParseUint(fields[1], 10, 64)
		nice, _ := strconv.ParseUint(fields[2], 10, 64)
		system, _ := strconv.ParseUint(fields[3], 10, 64)
		idle, _ := strconv.ParseUint(fields[4], 10, 64)

		if len(fields) > 5 {
			iowait, _ := strconv.ParseUint(fields[5], 10, 64)
			idle += iowait
		}

		times = append(times, cpuTime{
			user:   user,
			nice:   nice,
			system: system,
			idle:   idle,
		})
	}
	return times
}

// ---------------------------------------------------------------------------------------
// getLoadAverage reads load averages from /proc/loadavg.
func getLoadAverage() (float64, float64, float64) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0
	}
	l1, _ := strconv.ParseFloat(fields[0], 64)
	l5, _ := strconv.ParseFloat(fields[1], 64)
	l15, _ := strconv.ParseFloat(fields[2], 64)
	return l1, l5, l15
}

// ---------------------------------------------------------------------------------------
//
//	Memory Information
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// getTotalMemory returns total physical memory in bytes.
func getTotalMemory() uint64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			return parseMemInfoValue(line)
		}
	}
	return 0
}

// ---------------------------------------------------------------------------------------
// getMemoryInfoPlatform returns memory usage on Linux by parsing /proc/meminfo.
func getMemoryInfoPlatform() MemoryInfo {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return MemoryInfo{}
	}

	values := make(map[string]uint64)
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		values[key] = parseMemInfoValue(line)
	}

	total := values["MemTotal"]
	available := values["MemAvailable"]
	used := total - available
	pct := 0.0
	if total > 0 {
		pct = float64(used) / float64(total) * 100
	}

	swapTotal := values["SwapTotal"]
	swapFree := values["SwapFree"]
	swapUsed := swapTotal - swapFree
	swapPct := 0.0
	if swapTotal > 0 {
		swapPct = float64(swapUsed) / float64(swapTotal) * 100
	}

	return MemoryInfo{
		Total:       total,
		Available:   available,
		Used:        used,
		Percent:     roundTo1(pct),
		SwapTotal:   swapTotal,
		SwapUsed:    swapUsed,
		SwapPercent: roundTo1(swapPct),
	}
}

// ---------------------------------------------------------------------------------------
// parseMemInfoValue parses a /proc/meminfo line and returns the value in bytes.
func parseMemInfoValue(line string) uint64 {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return 0
	}
	valStr := parts[1]
	val, _ := strconv.ParseUint(valStr, 10, 64)
	if len(parts) >= 3 && strings.ToLower(parts[2]) == "kb" {
		val *= 1024
	}
	return val
}

// ---------------------------------------------------------------------------------------
//
//	Process Information
//
// ---------------------------------------------------------------------------------------

// UID-to-username cache, rebuilt once per snapshot cycle
var uidCache map[string]string

// ---------------------------------------------------------------------------------------
// buildUIDCache reads /etc/passwd once and builds a UID-to-username map.
func buildUIDCache() {
	uidCache = make(map[string]string)
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) >= 3 {
			uidCache[fields[2]] = fields[0]
		}
	}
}

// ---------------------------------------------------------------------------------------
// lookupUser resolves a numeric UID to a username using the cached map.
func lookupUser(uid string) string {
	if uid == "" {
		return ""
	}
	if name, ok := uidCache[uid]; ok {
		return name
	}
	return uid
}

// ---------------------------------------------------------------------------------------
// readProcessList reads processes from /proc on Linux.
func readProcessList() []rawProcess {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}

	// Build UID cache once for this snapshot
	buildUIDCache()

	totalMem := getCachedTotalMemory()
	var procs []rawProcess

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		procDir := filepath.Join("/proc", entry.Name())

		statData, err := os.ReadFile(filepath.Join(procDir, "stat"))
		if err != nil {
			continue
		}

		cmdlineData, _ := os.ReadFile(filepath.Join(procDir, "cmdline"))
		cmdline := string(cmdlineData)

		statusData, _ := os.ReadFile(filepath.Join(procDir, "status"))

		// Parse /proc/[pid]/stat — find comm between parens (can contain spaces)
		statStr := string(statData)
		openParen := strings.IndexByte(statStr, '(')
		closeParen := strings.LastIndexByte(statStr, ')')
		if openParen < 0 || closeParen < 0 {
			continue
		}
		name := statStr[openParen+1 : closeParen]
		rest := strings.Fields(statStr[closeParen+2:])
		if len(rest) < 2 {
			continue
		}

		ppid, _ := strconv.Atoi(rest[1])

		// Parse RSS and UID from status
		var rss uint64
		var uid string
		for _, line := range strings.Split(string(statusData), "\n") {
			if strings.HasPrefix(line, "VmRSS:") {
				rss = parseMemInfoValue(line)
			}
			if strings.HasPrefix(line, "Uid:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					uid = fields[1]
				}
			}
		}

		memPct := 0.0
		if totalMem > 0 {
			memPct = float64(rss) / float64(totalMem) * 100
		}

		procName := name
		if cmdline != "" {
			args := splitCmdline(cmdline)
			if len(args) > 0 {
				base := filepath.Base(args[0])
				if base != "" {
					procName = base
				}
			}
		}

		user := lookupUser(uid)

		procs = append(procs, rawProcess{
			pid:         pid,
			ppid:        ppid,
			name:        procName,
			cmdline:     cmdline,
			cpuPercent:  0,
			memPercent:  roundTo1(memPct),
			memoryBytes: rss,
			user:        user,
		})
	}

	// Get CPU percentages from ps (single subprocess call for all processes)
	cpuFromPS(procs)

	return procs
}

// ---------------------------------------------------------------------------------------
// cpuFromPS enriches process CPU percentages using a single ps call.
func cpuFromPS(procs []rawProcess) {
	out, err := runCommand("ps", "-eo", "pid,%cpu", "--no-headers")
	if err != nil {
		return
	}

	cpuMap := make(map[int]float64)
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		cpu, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			continue
		}
		cpuMap[pid] = cpu
	}

	for i := range procs {
		if cpu, ok := cpuMap[procs[i].pid]; ok {
			procs[i].cpuPercent = cpu
		}
	}
}
