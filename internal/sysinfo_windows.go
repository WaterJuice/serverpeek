// ---------------------------------------------------------------------------------------
//
//	sysinfo_windows.go
//	-------------------
//
//	Windows-specific system information gathering. Provides minimal stubs
//	so the binary compiles on Windows — full monitoring requires Linux or macOS.
//
//	(c) 2026 WaterJuice — Released under the Unlicense; see LICENSE.
//
//	Version History
//	---------------
//	Mar 2026 - Created
//	Jun 2026 - Added getDiskInfo stub
//
// ---------------------------------------------------------------------------------------
package internal

// ---------------------------------------------------------------------------------------
//
//	Imports
//
// ---------------------------------------------------------------------------------------

import (
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
// getBootTime returns the system boot time on Windows using wmic.
func getBootTime() time.Time {
	out, err := runCommand("wmic", "os", "get", "lastbootuptime")
	if err != nil {
		return time.Now()
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return time.Now()
	}
	s := strings.TrimSpace(lines[1])
	if len(s) < 14 {
		return time.Now()
	}
	t, err := time.Parse("20060102150405", s[:14])
	if err != nil {
		return time.Now()
	}
	return t
}

// ---------------------------------------------------------------------------------------
//
//	OS Information
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// getOSInfo returns the Windows version and release.
func getOSInfo() (version string, release string) {
	out, err := runCommand("cmd", "/c", "ver")
	if err != nil {
		return "Windows", ""
	}
	version = strings.TrimSpace(out)
	return
}

// ---------------------------------------------------------------------------------------
//
//	CPU Information
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// getCPUModel returns the CPU model string on Windows.
func getCPUModel() string {
	out, err := runCommand("wmic", "cpu", "get", "name")
	if err != nil {
		return "Unknown"
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return "Unknown"
	}
	return strings.TrimSpace(lines[1])
}

// ---------------------------------------------------------------------------------------
// getPhysicalCores returns the number of physical CPU cores.
func getPhysicalCores() int {
	out, err := runCommand("wmic", "cpu", "get", "NumberOfCores")
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(lines[1]))
	return n
}

// ---------------------------------------------------------------------------------------
// readCPUTimes returns stub CPU times on Windows.
func readCPUTimes() []cpuTime {
	numCPU := getPhysicalCores()
	if numCPU <= 0 {
		numCPU = 1
	}
	return make([]cpuTime, numCPU)
}

// ---------------------------------------------------------------------------------------
// getLoadAverage returns zero load averages on Windows (not available).
func getLoadAverage() (float64, float64, float64) {
	return 0, 0, 0
}

// ---------------------------------------------------------------------------------------
//
//	Memory Information
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// getTotalMemory returns total physical memory in bytes.
func getTotalMemory() uint64 {
	out, err := runCommand("wmic", "computersystem", "get", "TotalPhysicalMemory")
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return 0
	}
	val, _ := strconv.ParseUint(strings.TrimSpace(lines[1]), 10, 64)
	return val
}

// ---------------------------------------------------------------------------------------
// getMemoryInfoPlatform returns memory usage on Windows.
func getMemoryInfoPlatform() MemoryInfo {
	total := getCachedTotalMemory()
	return MemoryInfo{
		Total:   total,
		Used:    0,
		Percent: 0,
	}
}

// ---------------------------------------------------------------------------------------
//
//	Disk Information
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// getDiskInfo returns an empty DiskInfo on Windows (not implemented).
func getDiskInfo() DiskInfo {
	return DiskInfo{Mountpoint: "C:"}
}

// ---------------------------------------------------------------------------------------
//
//	Process Information
//
// ---------------------------------------------------------------------------------------

// ---------------------------------------------------------------------------------------
// readProcessList returns an empty process list on Windows.
func readProcessList() []rawProcess {
	return nil
}
