package srtla

import (
	"context"
	"log"
	"os/exec"
	"strconv"
	"time"
)

// PipelineConfig 描述 SRTLA Receiver 與 SRT Listener 之間的基本連線參數。
type PipelineConfig struct {
	NodeID        string
	SRTPassphrase string

	SRTLAIngestPort int
	SRTOutputPort   int

	// 未來可擴充更多 SRT 相關參數（latency、rcvbuf 等）。
}

// Pipeline 負責管理對應的外部進程，並提供基本的 watchdog 與指數退避重啟邏輯。
type Pipeline struct {
	cfg    PipelineConfig
	logger *log.Logger
}

func NewPipeline(cfg PipelineConfig, logger *log.Logger) *Pipeline {
	if logger == nil {
		logger = log.Default()
	}
	return &Pipeline{
		cfg:    cfg,
		logger: logger,
	}
}

// Run 會在未取消前持續確保外部 SRTLA → SRT 流程存在。
// 若子進程異常結束，會採用指數退避方式重啟。
func (p *Pipeline) Run(ctx context.Context) {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			p.logger.Printf("[route][srtla] context canceled, stop pipeline")
			return
		default:
		}

		cmd := p.buildCommand(ctx)
		p.logger.Printf("[route][srtla] starting pipeline node_id=%s ingest_port=%d srt_out_port=%d",
			p.cfg.NodeID, p.cfg.SRTLAIngestPort, p.cfg.SRTOutputPort)

		startTime := time.Now()
		if err := cmd.Run(); err != nil {
			p.logger.Printf("[route][srtla] pipeline exited with error err=%v", err)
		} else {
			p.logger.Printf("[route][srtla] pipeline exited normally runtime=%s", time.Since(startTime))
		}

		// 若執行時間非常短，代表可能有配置錯誤或環境問題，仍然遵守退避策略避免瘋狂重啟。
		p.logger.Printf("[route][srtla] restarting after backoff=%s", backoff)

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// buildCommand 產生實際啟動 SRTLA Receiver → SRT Listener 的外部命令。
// 這裡先採用 MVP 版本，假設系統中已安裝對應的二進位檔，未來可再抽成可配置參數。
func (p *Pipeline) buildCommand(ctx context.Context) *exec.Cmd {
	// TODO: 依實際部署環境調整 binary 名稱與參數。
	// 下列僅為範例，代表：
	// - 使用假想的 belabox-srtla-recv 監聽 ingest port
	// - 並將匯聚結果輸出至本地 SRT Listener port

	args := []string{
		"--ingest-port", intToString(p.cfg.SRTLAIngestPort),
		"--srt-output-port", intToString(p.cfg.SRTOutputPort),
	}

	// 若有設定 SRT Passphrase，透過環境或參數傳入外部程式，避免硬編。
	if p.cfg.SRTPassphrase != "" {
		args = append(args, "--srt-passphrase", p.cfg.SRTPassphrase)
	}

	cmd := exec.CommandContext(ctx, "belabox-srtla-recv", args...)
	return cmd
}

func intToString(v int) string {
	return strconv.Itoa(v)
}

