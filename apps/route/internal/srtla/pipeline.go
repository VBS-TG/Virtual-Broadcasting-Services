package srtla

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

// PipelineConfig 描述 SRTLA Receiver 與 SRT Listener 之間的基本連線參數。
type PipelineConfig struct {
	NodeID        string
	SRTPassphrase string

	SRTLAIngestPort int
	SRTOutputPort   int

	InternalSRTPort int
	LossMaxTTL      int
	LatencyMs       int
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

		p.logger.Printf(
			"[route][srtla] starting pipeline node_id=%s ingest_port=%d internal_srt_port=%d srt_out_port=%d lossmaxttl=%d latency_ms=%d",
			p.cfg.NodeID, p.cfg.SRTLAIngestPort, p.cfg.InternalSRTPort, p.cfg.SRTOutputPort, p.cfg.LossMaxTTL, p.cfg.LatencyMs,
		)

		// MVP：srt-live-transmit 先建立內部 SRT listener，再由 srtla_rec 將 SRTLA 輸入轉送到內部 SRT。
		ltCmd := p.buildSrtLiveTransmitCommand(ctx)
		recCmd := p.buildSrtlaRecCommand(ctx)

		ltStarted := false
		recStarted := false

		if err := ltCmd.Start(); err != nil {
			p.logger.Printf("[route][srtla] failed to start srt-live-transmit err=%v", err)
		} else {
			ltStarted = true
			p.logger.Printf("[route][srtla] started srt-live-transmit pid=%d", ltCmd.Process.Pid)
		}

		// 給內部 listener 一點時間讓其準備就緒。
		time.Sleep(500 * time.Millisecond)

		if err := recCmd.Start(); err != nil {
			p.logger.Printf("[route][srtla] failed to start srtla_rec err=%v", err)
		} else {
			recStarted = true
			p.logger.Printf("[route][srtla] started srtla_rec pid=%d", recCmd.Process.Pid)
		}

		errCh := make(chan error, 2)
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			errCh <- ltCmd.Wait()
		}()
		go func() {
			defer wg.Done()
			errCh <- recCmd.Wait()
		}()

		var runErr error
		select {
		case <-ctx.Done():
			p.logger.Printf("[route][srtla] context canceled, stopping child processes")
		case runErr = <-errCh:
			p.logger.Printf("[route][srtla] child process exited err=%v, restarting whole pipeline", runErr)
		}

		// 任一子進程結束即重啟整條 pipeline。
		if recStarted && recCmd.Process != nil {
			_ = recCmd.Process.Kill()
		}
		if ltStarted && ltCmd.Process != nil {
			_ = ltCmd.Process.Kill()
		}

		// 等待 Wait goroutines 結束，避免資源殘留。
		wg.Wait()

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

func intToString(v int) string {
	return strconv.Itoa(v)
}

func (p *Pipeline) buildSrtLiveTransmitCommand(ctx context.Context) *exec.Cmd {
	// 依照 BELABOX/srtla README 的建議：
	// - srt-live-transmit 使用 internal listener 端口接收 srtla_rec 的輸出
	// - 再把穩定流提供給 Engine 的 srt://0.0.0.0:<SRT_OUTPUT_PORT>?mode=listener
	//
	// 備註：
	// README 明示其基本設定不含 encryption/auth；因此 MVP 先把加密套用在「對 Engine 的輸出端」。
	// 你若後續確認 srtla_rec 也能協同 SRT encryption，可再擴充到輸入端。

	// internal input：不帶 passphrase（MVP 對照其 README 預設行為）
	inURL := fmt.Sprintf(
		"srt://127.0.0.1:%d?mode=listener&lossmaxttl=%d&latency=%d",
		p.cfg.InternalSRTPort,
		p.cfg.LossMaxTTL,
		p.cfg.LatencyMs,
	)

	// output listener：套用 AES-256 passphrase（對應 .cursorrules 要求）
	outURL := fmt.Sprintf(
		"srt://0.0.0.0:%d?mode=listener&passphrase=%s&pbkeylen=32&latency=%d",
		p.cfg.SRTOutputPort,
		// 注意：避免對 passphrase 再做額外 URL encoding，避免造成 receiver/listener 端解密密鑰不一致。
		// .cursorrules 規定 passphrase 需一致性，因此這裡直接使用原始字串。
		p.cfg.SRTPassphrase,
		p.cfg.LatencyMs,
	)

	// -st:yes 表示使用順序時間戳校正（保持與官方示例一致）
	return exec.CommandContext(ctx, "srt-live-transmit", "-st:yes", inURL, outURL)
}

func (p *Pipeline) buildSrtlaRecCommand(ctx context.Context) *exec.Cmd {
	// 依照 BELABOX/srtla README：
	// path/to/srtla/srtla_rec <srtla_listen_port> <receiver_srt_ip> <receiver_srt_output_port>
	//
	// 其中 <receiver_srt_output_port> 對應 internal listener 給 srt-live-transmit 消費。
	args := []string{
		intToString(p.cfg.SRTLAIngestPort),
		"127.0.0.1",
		intToString(p.cfg.InternalSRTPort),
	}
	return exec.CommandContext(ctx, "srtla_rec", args...)
}

