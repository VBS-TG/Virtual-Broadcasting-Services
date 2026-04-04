package system

import (
	"fmt"
	"net"
)

// CheckUDPPortAvailable 檢查指定 UDP 埠是否可綁定。
// 若不可綁定，通常代表已有其他程序占用，需先清理殘留程序。
func CheckUDPPortAvailable(port int) error {
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	ln, err := net.ListenPacket("udp", addr)
	if err != nil {
		return err
	}
	_ = ln.Close()
	return nil
}

