package telemetry

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"vbs/apps/route/internal/config"
)

// WSSClient 以 WebSocket 將遙測送往 Console Hub（Header: X-VBS-Key）。
type WSSClient struct {
	cfg    config.Config
	dialer websocket.Dialer
}

func NewWSSClient(cfg config.Config) *WSSClient {
	d := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	if cfg.TelemetryTLSInsecureSkipVerify {
		d.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // 僅供測試環境明確開啟
	}
	return &WSSClient{cfg: cfg, dialer: d}
}

// SendOne 建立短連線並送出單筆 JSON（Fire-and-forget 模式下的「每次上報重新連線」簡化實作；長連線可後續優化）。
func (c *WSSClient) SendOne(ctx context.Context, payload []byte) error {
	wsURL, err := c.cfg.TelemetryWSSURL()
	if err != nil {
		return err
	}

	header := http.Header{}
	header.Set("X-VBS-Key", c.cfg.APIKey)

	conn, resp, err := c.dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		if resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		return err
	}
	defer conn.Close()

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(5 * time.Second)
	}
	_ = conn.SetWriteDeadline(deadline)
	return conn.WriteMessage(websocket.TextMessage, payload)
}
