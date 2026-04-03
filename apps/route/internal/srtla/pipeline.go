package srtla

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PipelineConfig 描述單次 SRTLA → SRT 管線快照（可熱更新）。
type PipelineConfig struct {
	NodeID        string
	SRTPassphrase string

	SRTLAIngestPort int
	SRTOutputPort   int
	InternalSRTPort int
	LossMaxTTL      int
	LatencyMs       int
}

// Pipeline 管理外部進程與 watchdog（子進程結束、外部重啟信號皆會觸發整條管線重建）。
type Pipeline struct {
	getConfig func() PipelineConfig
	logger    *log.Logger
	mu        sync.RWMutex
	stats     Stats
}

// Stats 為 Route pipeline 的即時狀態摘要，供遙測使用。
type Stats struct {
	BytesLost     uint64
	BytesSent     uint64
	BytesReceived uint64
	LastUpdate    time.Time
}

// NewPipeline 建立管線；getConfig 每次啟動子進程前呼叫，以套用熱更新參數。
func NewPipeline(getConfig func() PipelineConfig, logger *log.Logger) *Pipeline {
	if logger == nil {
		logger = log.Default()
	}
	return &Pipeline{
		getConfig: getConfig,
		logger:    logger,
	}
}

func (p *Pipeline) Snapshot() Stats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stats
}

// Run 持續維持 SRTLA → SRT 流程。externalRestart 非 nil 時，收到信號會強制重啟管線（停滯自癒或控制面更新）。
func (p *Pipeline) Run(ctx context.Context, externalRestart <-chan struct{}) {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			p.logger.Printf("[route][srtla] context canceled，停止管線")
			return
		default:
		}

		cfg := p.getConfig()
		p.logger.Printf(
			"[route][srtla] 啟動管線 node_id=%s ingest_port=%d internal_srt_port=%d srt_out_port=%d lossmaxttl=%d latency_ms=%d output_encryption=%t",
			cfg.NodeID, cfg.SRTLAIngestPort, cfg.InternalSRTPort, cfg.SRTOutputPort, cfg.LossMaxTTL, cfg.LatencyMs, cfg.SRTPassphrase != "",
		)

		innerCtx, cancel := context.WithCancel(ctx)
		ltCmd := p.buildSrtLiveTransmitCommand(innerCtx, cfg)
		recCmd := p.buildSrtlaRecCommand(innerCtx, cfg)
		p.attachCommandLogs("srt-live-transmit", ltCmd)
		p.attachCommandLogs("srtla_rec", recCmd)

		ltStarted := false
		recStarted := false

		if err := ltCmd.Start(); err != nil {
			p.logger.Printf("[route][srtla] 無法啟動 srt-live-transmit err=%v", err)
		} else {
			ltStarted = true
			p.logger.Printf("[route][srtla] 已啟動 srt-live-transmit pid=%d", ltCmd.Process.Pid)
		}

		time.Sleep(500 * time.Millisecond)

		if err := recCmd.Start(); err != nil {
			p.logger.Printf("[route][srtla] 無法啟動 srtla_rec err=%v", err)
		} else {
			recStarted = true
			p.logger.Printf("[route][srtla] 已啟動 srtla_rec pid=%d", recCmd.Process.Pid)
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

		reason := "child_exit"
		select {
		case <-ctx.Done():
			reason = "shutdown"
		case <-externalRestart:
			reason = "external_restart"
			p.logger.Printf("[route][srtla] 收到外部重啟信號，將重建管線")
		case err := <-errCh:
			p.logger.Printf("[route][srtla] 子進程結束 err=%v，將重建管線", err)
		}

		if recStarted && recCmd.Process != nil {
			_ = recCmd.Process.Kill()
		}
		if ltStarted && ltCmd.Process != nil {
			_ = ltCmd.Process.Kill()
		}
		cancel()
		wg.Wait()

		if reason == "shutdown" {
			return
		}

		if reason == "external_restart" {
			backoff = time.Second
			continue
		}

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

func (p *Pipeline) buildSrtLiveTransmitCommand(ctx context.Context, cfg PipelineConfig) *exec.Cmd {
	// 內部迴路：本機 127.0.0.1，不經公網；對 Engine 的 listener 強制 AES-256（passphrase 由環境注入）。
	inURL := fmt.Sprintf(
		"srt://127.0.0.1:%d?mode=listener&lossmaxttl=%d&latency=%d",
		cfg.InternalSRTPort,
		cfg.LossMaxTTL,
		cfg.LatencyMs,
	)

	outURL := fmt.Sprintf(
		"srt://0.0.0.0:%d?mode=listener&latency=%d",
		cfg.SRTOutputPort,
		cfg.LatencyMs,
	)
	if cfg.SRTPassphrase != "" {
		outURL = fmt.Sprintf(
			"srt://0.0.0.0:%d?mode=listener&latency=%d&passphrase=%s&pbkeylen=32&enforcedencryption=1",
			cfg.SRTOutputPort,
			cfg.LatencyMs,
			cfg.SRTPassphrase,
		)
	}

	return exec.CommandContext(ctx, "srt-live-transmit", "-st:yes", inURL, outURL)
}

func (p *Pipeline) buildSrtlaRecCommand(ctx context.Context, cfg PipelineConfig) *exec.Cmd {
	args := []string{
		intToString(cfg.SRTLAIngestPort),
		"127.0.0.1",
		intToString(cfg.InternalSRTPort),
	}
	return exec.CommandContext(ctx, "srtla_rec", args...)
}

func (p *Pipeline) attachCommandLogs(name string, cmd *exec.Cmd) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		p.logger.Printf("[route][srtla] 無法綁定 stdout（%s） err=%v", name, err)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		p.logger.Printf("[route][srtla] 無法綁定 stderr（%s） err=%v", name, err)
		return
	}

	go p.streamLogs(name, "stdout", stdout)
	go p.streamLogs(name, "stderr", stderr)
}

func (p *Pipeline) streamLogs(procName, stream string, r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := sanitizeSensitive(scanner.Text())
		p.logger.Printf("[route][srtla][%s][%s] %s", procName, stream, line)
		if procName == "srt-live-transmit" {
			p.tryParseSrtStats(line)
		}
	}
	if err := scanner.Err(); err != nil {
		p.logger.Printf("[route][srtla][%s][%s] 讀取錯誤 err=%v", procName, stream, err)
	}
}

var passphrasePattern = regexp.MustCompile(`(?i)(passphrase=)[^&\s]+`)

func sanitizeSensitive(s string) string {
	return passphrasePattern.ReplaceAllString(s, "${1}***")
}

var srtStatsPattern = regexp.MustCompile(`(?i)(\d+)\s+bytes\s+lost,\s+(\d+)\s+bytes\s+sent,\s+(\d+)\s+bytes\s+received`)

func (p *Pipeline) tryParseSrtStats(line string) {
	m := srtStatsPattern.FindStringSubmatch(strings.ToLower(line))
	if len(m) != 4 {
		return
	}
	lost, err1 := strconv.ParseUint(m[1], 10, 64)
	sent, err2 := strconv.ParseUint(m[2], 10, 64)
	recv, err3 := strconv.ParseUint(m[3], 10, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return
	}
	p.mu.Lock()
	p.stats = Stats{
		BytesLost:     lost,
		BytesSent:     sent,
		BytesReceived: recv,
		LastUpdate:    time.Now(),
	}
	p.mu.Unlock()
}
