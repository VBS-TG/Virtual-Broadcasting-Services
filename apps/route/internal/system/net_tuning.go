package system

import (
	"log"
	"os"
	"os/exec"
	"strings"
)

// ApplyNetTuning 嘗試套用 Route 節點在 .cursorrules 中規定的基礎網路緩衝設定。
// 若執行失敗，僅記錄警告日誌，不中斷主流程，避免在未有權限的環境下造成啟動失敗。
// 可選：設定 VBS_ROUTE_MTU_IFACE 為介面名（如 eth0），並搭配 VBS_ROUTE_MTU（預設 1400）以符合行動網路 MTU 建議。
func ApplyNetTuning(logger *log.Logger) {
	if logger == nil {
		logger = log.Default()
	}

	commands := [][]string{
		{"sysctl", "-w", "net.core.rmem_max=16777216"},
		{"sysctl", "-w", "net.core.wmem_max=16777216"},
	}

	for _, cmdParts := range commands {
		if len(cmdParts) == 0 {
			continue
		}
		cmd := exec.Command(cmdParts[0], cmdParts[1:]...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Printf("[route][net] sysctl 套用失敗 cmd=%v err=%v output=%s", cmdParts, err, string(output))
			continue
		}
		logger.Printf("[route][net] 已套用 sysctl cmd=%v output=%s", cmdParts, string(output))
	}

	iface := strings.TrimSpace(os.Getenv("VBS_ROUTE_MTU_IFACE"))
	if iface == "" {
		return
	}
	mtu := strings.TrimSpace(os.Getenv("VBS_ROUTE_MTU"))
	if mtu == "" {
		mtu = "1400"
	}
	cmd := exec.Command("ip", "link", "set", "dev", iface, "mtu", mtu)
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Printf("[route][net] MTU 設定失敗 iface=%s mtu=%s err=%v out=%s", iface, mtu, err, string(out))
		return
	}
	logger.Printf("[route][net] 已設定 MTU iface=%s mtu=%s", iface, mtu)
}

