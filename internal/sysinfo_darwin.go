// ---------------------------------------------------------------------------------------
//
//	sysinfo_darwin.go
//	------------------
//
//	macOS-specific system information gathering. Uses sysctl, vm_stat, and ps
//	to collect CPU, memory, and process information without external dependencies.
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
	"encoding/binary"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ---------------------------------------------------------------------------------------
//
//	Boot Time
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// getBootTime returns the system boot time using sysctl kern.boottime.
// kern.boottime returns a struct timeval where tv_sec is a 32-bit or 64-bit
// integer depending on the platform, stored in little-endian binary form.
func getBootTime() time.Time {
	raw, err := syscall.Sysctl("kern.boottime")
	if err != nil {
		return time.Now()
	}

	// Convert string to bytes for safe binary parsing
	b := []byte(raw)
	if len(b) < 4 {
		return time.Now()
	}

	// On macOS, struct timeval uses a 4-byte tv_sec on 32-bit and 8-byte on 64-bit.
	// In practice on modern macOS (all 64-bit), we read 4 bytes because the kernel
	// returns a __darwin_time_t which is `long` but the sysctl interface truncates.
	// Read as uint32 first; sufficient until 2106.
	sec := binary.LittleEndian.Uint32(b[:4])
	return time.Unix(int64(sec), 0)
}

// ---------------------------------------------------------------------------------------
//
//	OS Information
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// getOSInfo returns the macOS version and product release.
func getOSInfo() (version string, release string) {
	version, _ = syscall.Sysctl("kern.version")
	version = strings.TrimSpace(version)
	release, _ = syscall.Sysctl("kern.osproductversion")
	release = strings.TrimSpace(release)
	if release == "" {
		// Fallback to kernel release if product version unavailable
		release, _ = syscall.Sysctl("kern.osrelease")
		release = strings.TrimSpace(release)
	}
	return
}

// ---------------------------------------------------------------------------------------
//
//	CPU Information
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// getCPUModel returns the CPU model string using sysctl.
func getCPUModel() string {
	model, err := syscall.Sysctl("machdep.cpu.brand_string")
	if err == nil && model != "" {
		return strings.TrimSpace(model)
	}
	return "Unknown"
}

// ---------------------------------------------------------------------------------------
// getPhysicalCores returns the number of physical CPU cores.
func getPhysicalCores() int {
	n, err := syscall.SysctlUint32("hw.physicalcpu")
	if err == nil {
		return int(n)
	}
	return 0
}

// ---------------------------------------------------------------------------------------
// readCPUTimes reads aggregate CPU usage on macOS using `top -l 1 -n 0 -s 0`.
// Per-core CPU counters require the Mach host_processor_info API which is only
// accessible via cgo — not available in a pure Go build. Returns a single-element
// slice; the dashboard hides per-core bars and shows only the overall bar.
func readCPUTimes() []cpuTime {
	out, err := runCommand("top", "-l", "1", "-n", "0", "-s", "0")
	if err != nil {
		return []cpuTime{{}}
	}

	var userPct, sysPct, idlePct float64
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "CPU usage:") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if strings.HasSuffix(f, "user,") || strings.HasSuffix(f, "user") {
					if i > 0 {
						userPct, _ = strconv.ParseFloat(strings.TrimSuffix(strings.TrimSuffix(fields[i-1], "%"), ","), 64)
					}
				}
				if strings.HasSuffix(f, "sys,") || strings.HasSuffix(f, "sys") {
					if i > 0 {
						sysPct, _ = strconv.ParseFloat(strings.TrimSuffix(strings.TrimSuffix(fields[i-1], "%"), ","), 64)
					}
				}
				if strings.HasSuffix(f, "idle") {
					if i > 0 {
						idlePct, _ = strconv.ParseFloat(strings.TrimSuffix(strings.TrimSuffix(fields[i-1], "%"), ","), 64)
					}
				}
			}
			break
		}
	}

	// Accumulate pseudo-ticks so delta-based calculation in getCPUInfo works
	prevCPUTimesLock.Lock()
	var base cpuTime
	if len(prevCPUTimes) > 0 {
		base = prevCPUTimes[0]
	}
	prevCPUTimesLock.Unlock()

	return []cpuTime{{
		user:   base.user + uint64(userPct*100),
		system: base.system + uint64(sysPct*100),
		idle:   base.idle + uint64(idlePct*100),
	}}
}

// ---------------------------------------------------------------------------------------
// getLoadAverage returns system load averages on macOS.
// Uses `sysctl -n vm.loadavg` which returns "{ 1.23 4.56 7.89 }".
func getLoadAverage() (float64, float64, float64) {
	out, err := runCommand("sysctl", "-n", "vm.loadavg")
	if err != nil {
		return 0, 0, 0
	}
	s := strings.Trim(strings.TrimSpace(out), "{ }")
	fields := strings.Fields(s)
	if len(fields) < 3 {
		return 0, 0, 0
	}
	l1, _ := strconv.ParseFloat(fields[0], 64)
	l5, _ := strconv.ParseFloat(fields[1], 64)
	l15, _ := strconv.ParseFloat(fields[2], 64)
	return roundTo1(l1), roundTo1(l5), roundTo1(l15)
}

// ---------------------------------------------------------------------------------------
//
//	Memory Information
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// getTotalMemory returns total physical memory in bytes.
// hw.memsize is a 64-bit value; SysctlUint32 would truncate on >4GB machines,
// so we always use the subprocess approach.
func getTotalMemory() uint64 {
	out, err := runCommand("sysctl", "-n", "hw.memsize")
	if err != nil {
		return 0
	}
	val, _ := strconv.ParseUint(strings.TrimSpace(out), 10, 64)
	return val
}

// ---------------------------------------------------------------------------------------
// getMemoryInfoPlatform returns memory usage on macOS using vm_stat and sysctl.
func getMemoryInfoPlatform() MemoryInfo {
	total := getCachedTotalMemory()

	out, err := runCommand("vm_stat")
	if err != nil {
		return MemoryInfo{Total: total}
	}

	pageSize := uint64(16384) // Default on Apple Silicon
	values := make(map[string]uint64)

	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "Mach Virtual Memory Statistics") {
			parts := strings.Split(line, "page size of ")
			if len(parts) == 2 {
				ps, _ := strconv.ParseUint(strings.TrimRight(parts[1], " bytes)"), 10, 64)
				if ps > 0 {
					pageSize = ps
				}
			}
			continue
		}
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		key := strings.TrimSpace(parts[0])
		valStr := strings.TrimSpace(strings.TrimRight(parts[1], "."))
		val, _ := strconv.ParseUint(valStr, 10, 64)
		values[key] = val * pageSize
	}

	active := values["Pages active"]
	wired := values["Pages wired down"]
	compressed := values["Pages occupied by compressor"]
	free := values["Pages free"]
	speculative := values["Pages speculative"]
	inactive := values["Pages inactive"]

	// On macOS, used = active + wired + compressed (excludes file cache)
	used := active + wired + compressed
	available := free + inactive + speculative
	if available > total {
		available = total
	}

	pct := 0.0
	if total > 0 {
		pct = float64(used) / float64(total) * 100
	}

	// Get swap info (sysctl returns binary struct, so use the command instead)
	var swapTotal, swapUsed uint64
	var swapPct float64
	swapOut, err := runCommand("sysctl", "-n", "vm.swapusage")
	if err == nil {
		swapTotal, swapUsed = parseSwapUsage(swapOut)
		if swapTotal > 0 {
			swapPct = float64(swapUsed) / float64(swapTotal) * 100
		}
	}

	return MemoryInfo{
		Total:       total,
		Available:   available,
		Used:        used,
		Percent:     roundTo1(pct),
		Active:      active,
		Wired:       wired,
		Compressed:  compressed,
		SwapTotal:   swapTotal,
		SwapUsed:    swapUsed,
		SwapPercent: roundTo1(swapPct),
	}
}

// ---------------------------------------------------------------------------------------
// parseSwapUsage parses macOS sysctl vm.swapusage output.
// Format: "total = 2048.00M  used = 123.45M  free = 1924.55M  ..."
func parseSwapUsage(s string) (total uint64, used uint64) {
	fields := strings.Fields(s)
	for i, f := range fields {
		if f == "=" && i > 0 && i+1 < len(fields) {
			label := fields[i-1]
			valStr := fields[i+1]
			if strings.HasSuffix(valStr, "M") {
				val, err := strconv.ParseFloat(strings.TrimSuffix(valStr, "M"), 64)
				if err != nil {
					continue
				}
				bytes := uint64(val * 1024 * 1024)
				switch label {
				case "total":
					total = bytes
				case "used":
					used = bytes
				}
			}
		}
	}
	return
}

// ---------------------------------------------------------------------------------------
//
//	Process Information
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// readProcessList reads processes on macOS using two ps calls: one for process
// info with comm (executable name, no path/args), and one for full command lines
// (for friendly name extraction). Two calls avoids the N+1 per-process problem
// while correctly handling spaces in binary paths (e.g. "Visual Studio Code").
func readProcessList() []rawProcess {
	out, err := runCommand("ps", "-axo", "pid,ppid,pcpu,pmem,rss,user,comm")
	if err != nil {
		return nil
	}

	// Build pid→cmdline map from a second ps call
	cmdlines := make(map[int]string)
	cmdOut, err := runCommand("ps", "-axo", "pid,command")
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(cmdOut), "\n")[1:] {
			trimmed := strings.TrimSpace(line)
			if spaceIdx := strings.IndexByte(trimmed, ' '); spaceIdx > 0 {
				pid, err := strconv.Atoi(trimmed[:spaceIdx])
				if err == nil {
					cmdlines[pid] = strings.TrimSpace(trimmed[spaceIdx+1:])
				}
			}
		}
	}

	var procs []rawProcess
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return nil
	}

	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) < 7 {
			continue
		}

		pid, _ := strconv.Atoi(fields[0])
		ppid, _ := strconv.Atoi(fields[1])
		cpuPct, _ := strconv.ParseFloat(fields[2], 64)
		memPct, _ := strconv.ParseFloat(fields[3], 64)
		rssKB, _ := strconv.ParseUint(fields[4], 10, 64)
		user := fields[5]
		// comm is the last column (may be a full path) — join remaining
		// fields to handle paths with spaces, then take the basename
		fullPath := strings.Join(fields[6:], " ")
		name := fullPath
		if idx := strings.LastIndex(fullPath, "/"); idx >= 0 {
			name = fullPath[idx+1:]
		}

		procs = append(procs, rawProcess{
			pid:         pid,
			ppid:        ppid,
			name:        name,
			cmdline:     cmdlines[pid],
			cpuPercent:  cpuPct,
			memPercent:  roundTo1(memPct),
			memoryBytes: rssKB * 1024,
			user:        user,
		})
	}

	return procs
}
