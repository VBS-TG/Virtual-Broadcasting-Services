//go:build !linux

package system

import "fmt"

func HostCPUPercent() (float64, error) {
	return 0, fmt.Errorf("HostCPUPercent 僅支援 linux")
}

func HostMemUsedBytes() (uint64, error) {
	return 0, fmt.Errorf("HostMemUsedBytes 僅支援 linux")
}
