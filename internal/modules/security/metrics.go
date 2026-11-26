package security

import (
	"bufio"
	"context"
	"math"
	"strconv"
	"strings"
	"time"

	"tamestack/internal/i18n"
)

// --- Realtime System Metrics ---

// GetRealtimeSystemMetrics captures an instantaneous snapshot of server performance.
func (s *Service) GetRealtimeSystemMetrics(ctx context.Context, serverID string) (*RealtimeSystemMetrics, error) {
	client, _, err := s.getSSHClient(ctx, serverID)
	if err != nil {
		return nil, err
	}

	metrics := &RealtimeSystemMetrics{
		ServerID:  serverID,
		Timestamp: time.Now().Format("15:04:05"),
	}

	// Read all key system metrics in one fast composite call with clear section separators.
	batchCmd := "echo '===LOADAVG===' && cat /proc/loadavg 2>/dev/null && echo '===CORES===' && (nproc 2>/dev/null || grep -c ^processor /proc/cpuinfo 2>/dev/null || echo 1) && echo '===MEM===' && cat /proc/meminfo 2>/dev/null && echo '===DISK===' && (LC_ALL=C df -k -P / 2>/dev/null || LC_ALL=C df -k / 2>/dev/null) && echo '===UPTIME===' && cat /proc/uptime 2>/dev/null && echo '===END==='"
	out, err := s.execCmd(ctx, client, batchCmd)
	if err != nil {
		return nil, i18n.Errf("common.metricsCollectError", nil, err)
	}

	getSection := func(startTag, endTag string) string {
		startIdx := strings.Index(out, startTag)
		if startIdx == -1 {
			return ""
		}
		startIdx += len(startTag)
		sub := out[startIdx:]
		endIdx := strings.Index(sub, endTag)
		if endIdx == -1 {
			return strings.TrimSpace(sub)
		}
		return strings.TrimSpace(sub[:endIdx])
	}

	// Load average (strip possible SSH banner lines).
	loadSec := getSection("===LOADAVG===", "===")
	scanner := bufio.NewScanner(strings.NewReader(loadSec))
	var lastLine string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lastLine = line
		}
	}
	parts := strings.Fields(lastLine)
	if len(parts) >= 3 {
		metrics.Load1, _ = strconv.ParseFloat(parts[0], 64)
		metrics.Load5, _ = strconv.ParseFloat(parts[1], 64)
		metrics.Load15, _ = strconv.ParseFloat(parts[2], 64)
	}

	coresSec := getSection("===CORES===", "===")
	numCores := 1
	if n, err := strconv.Atoi(strings.TrimSpace(coresSec)); err == nil && n > 0 {
		numCores = n
	}
	if numCores > 0 {
		metrics.CPUPercent = (metrics.Load1 / float64(numCores)) * 100.0
		if metrics.CPUPercent > 100.0 {
			metrics.CPUPercent = 100.0
		}
		metrics.CPUPercent = math.Round(metrics.CPUPercent*10) / 10
	}

	memSec := getSection("===MEM===", "===")
	s.parseMemInfo(memSec, metrics)

	diskSec := getSection("===DISK===", "===")
	s.parseDiskInfo(diskSec, metrics)

	uptimeSec := getSection("===UPTIME===", "===")
	uptimeParts := strings.Fields(uptimeSec)
	if len(uptimeParts) > 0 {
		uSec, _ := strconv.ParseFloat(uptimeParts[0], 64)
		metrics.UptimeSeconds = uint64(uSec)
	}

	return metrics, nil
}

func (s *Service) parseMemInfo(content string, metrics *RealtimeSystemMetrics) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	var memTotalKB, memFreeKB, memAvailableKB, swapTotalKB, swapFreeKB uint64

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			key := strings.TrimSuffix(parts[0], ":")
			val, _ := strconv.ParseUint(parts[1], 10, 64)

			switch key {
			case "MemTotal":
				memTotalKB = val
			case "MemFree":
				memFreeKB = val
			case "MemAvailable":
				memAvailableKB = val
			case "SwapTotal":
				swapTotalKB = val
			case "SwapFree":
				swapFreeKB = val
			}
		}
	}

	metrics.MemoryTotalMB = memTotalKB / 1024
	if memAvailableKB > 0 {
		usedKB := memTotalKB - memAvailableKB
		metrics.MemoryUsedMB = usedKB / 1024
	} else {
		usedKB := memTotalKB - memFreeKB
		metrics.MemoryUsedMB = usedKB / 1024
	}
	if metrics.MemoryTotalMB >= metrics.MemoryUsedMB {
		metrics.MemoryFreeMB = metrics.MemoryTotalMB - metrics.MemoryUsedMB
	} else {
		metrics.MemoryFreeMB = 0
	}

	if metrics.MemoryTotalMB > 0 {
		metrics.MemoryUsedPercent = math.Round((float64(metrics.MemoryUsedMB)/float64(metrics.MemoryTotalMB))*1000) / 10
	}

	metrics.SwapTotalMB = swapTotalKB / 1024
	if swapTotalKB > swapFreeKB {
		metrics.SwapUsedMB = (swapTotalKB - swapFreeKB) / 1024
	}
}

func (s *Service) parseDiskInfo(content string, metrics *RealtimeSystemMetrics) {
	scanner := bufio.NewScanner(strings.NewReader(content))
	var allParts []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		// Skip any header line regardless of locale (e.g. "Filesystem ...", "Файл.система ..."):
		// a valid single-line df entry has a numeric 1024-blocks column at index 1.
		// Preserve 1-field wrapped device lines when we haven't collected fields yet.
		if len(allParts) == 0 && len(parts) >= 2 {
			if _, err := strconv.ParseFloat(parts[1], 64); err != nil {
				continue
			}
		}
		allParts = append(allParts, parts...)
	}

	// Output format of df -k -P /:
	// Filesystem 1024-blocks Used Available Capacity Mounted on
	// allParts[0] = device name (/dev/sda1)
	// allParts[1] = total in KB
	// allParts[2] = used in KB
	// allParts[3] = available in KB
	// allParts[4] = usage percentage
	// allParts[5] = mount point
	if len(allParts) >= 4 {
		totalKB, errTotal := strconv.ParseFloat(allParts[1], 64)
		usedKB, errUsed := strconv.ParseFloat(allParts[2], 64)
		freeKB, errFree := strconv.ParseFloat(allParts[3], 64)
		if errTotal != nil || errUsed != nil || errFree != nil {
			return
		}

		metrics.DiskTotalGB = math.Round((totalKB/(1024*1024))*10) / 10
		metrics.DiskUsedGB = math.Round((usedKB/(1024*1024))*10) / 10
		metrics.DiskFreeGB = math.Round((freeKB/(1024*1024))*10) / 10

		if metrics.DiskTotalGB > 0 {
			metrics.DiskUsedPercent = math.Round((metrics.DiskUsedGB/metrics.DiskTotalGB)*1000) / 10
		}
	}
}
