package system

import (
	"log"
	"os/exec"
)

// ApplyNetTuning 嘗試套用 Route 節點在 .cursorrules 中規定的基礎網路緩衝設定。
// 若執行失敗，僅記錄警告日誌，不中斷主流程，避免在未有權限的環境下造成啟動失敗。
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
			logger.Printf("[route][net] failed to apply sysctl cmd=%v err=%v output=%s", cmdParts, err, string(output))
			continue
		}
		logger.Printf("[route][net] applied sysctl cmd=%v output=%s", cmdParts, string(output))
	}
}

