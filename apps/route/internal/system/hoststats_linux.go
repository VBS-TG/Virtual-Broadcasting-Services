//go:build linux

package system

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var (
	prevIdle  uint64
	prevTotal uint64
	cpuReady  bool
)

// HostCPUPercent 回傳全系統 CPU 使用率（0–100），以兩次取樣間 /proc/stat 計算。
func HostCPUPercent() (float64, error) {
	idle, total, err := readCPUJiffies()
	if err != nil {
		return 0, err
	}
	if !cpuReady {
		prevIdle, prevTotal = idle, total
		cpuReady = true
		return 0, nil
	}
	idleDelta := float64(idle - prevIdle)
	totalDelta := float64(total - prevTotal)
	prevIdle, prevTotal = idle, total
	if totalDelta <= 0 {
		return 0, nil
	}
	used := 100.0 * (1.0 - idleDelta/totalDelta)
	if used < 0 {
		used = 0
	}
	if used > 100 {
		used = 100
	}
	return round1(used), nil
}

func readCPUJiffies() (idle, total uint64, err error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return 0, 0, fmt.Errorf("/proc/stat: empty")
	}
	fields := strings.Fields(sc.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, fmt.Errorf("/proc/stat: unexpected first line")
	}
	var vals []uint64
	for i := 1; i < len(fields); i++ {
		v, e := strconv.ParseUint(fields[i], 10, 64)
		if e != nil {
			return 0, 0, e
		}
		vals = append(vals, v)
	}
	if len(vals) < 4 {
		return 0, 0, fmt.Errorf("/proc/stat: cpu 欄位不足")
	}
	var sum uint64
	for _, v := range vals {
		sum += v
	}
	// idle + iowait（索引 3、4）
	idle = vals[3]
	if len(vals) > 4 {
		idle += vals[4]
	}
	return idle, sum, nil
}

// HostMemUsedBytes 回傳目前系統已用記憶體（bytes），以 MemTotal−MemAvailable 估算。
func HostMemUsedBytes() (uint64, error) {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	var total, avail uint64
	var haveT, haveA bool
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fs := strings.Fields(line)
			if len(fs) >= 2 {
				total, _ = strconv.ParseUint(fs[1], 10, 64)
				total *= 1024
				haveT = true
			}
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			fs := strings.Fields(line)
			if len(fs) >= 2 {
				avail, _ = strconv.ParseUint(fs[1], 10, 64)
				avail *= 1024
				haveA = true
			}
		}
	}
	if haveT && haveA && total >= avail {
		return total - avail, nil
	}
	return 0, fmt.Errorf("meminfo: MemTotal/MemAvailable 不完整")
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}
