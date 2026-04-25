import { useState } from 'react'

export default function ControlPanel() {
  const [bitrateCapture, setBitrateCapture] = useState(8000)
  const [latencyRoute, setLatencyRoute]     = useState(200)
  const [latencyPgm, setLatencyPgm]         = useState(200)
  const [pgmUrl, setPgmUrl]                 = useState('srt://route.vbs.example.com:9000?streamid=publish/pgm')
  const [streaming, setStreaming]           = useState(true)
  const [toast, setToast]                   = useState<string | null>(null)

  const showToast = (msg: string) => {
    setToast(msg)
    setTimeout(() => setToast(null), 2500)
  }

  return (
    <div className="h-full overflow-y-auto p-3 md:p-4 flex flex-col gap-3 md:gap-4">
      <h2 className="text-sm font-bold text-vbs-muted uppercase tracking-widest">系統信令控制台</h2>

      {/* CAPTURE */}
      <div className="glass rounded-xl p-4 flex flex-col gap-4">
        <div className="flex items-center gap-2">
          <span className="w-2 h-2 rounded-full bg-vbs-accent animate-pulse-slow shrink-0" />
          <span className="text-sm font-black text-vbs-accent tracking-widest">CAPTURE 端</span>
        </div>
        <div>
          <div className="flex items-center justify-between mb-1">
            <label className="text-sm font-semibold text-vbs-muted">目標碼率（動態調整）</label>
            <span className="text-sm font-bold text-vbs-text">{bitrateCapture} Kbps</span>
          </div>
          <input id="capture-bitrate" type="range" min={2000} max={20000} step={500}
            value={bitrateCapture} onChange={e => setBitrateCapture(Number(e.target.value))}
            className="w-full h-2 accent-vbs-accent" />
          <div className="flex justify-between text-xs font-medium text-vbs-muted mt-0.5">
            <span>2 Mbps</span><span>20 Mbps</span>
          </div>
        </div>
        {/* 手機友善：按鈕變大，2欄 */}
        <div className="grid grid-cols-2 gap-2">
          <button id="apply-bitrate" onClick={() => showToast(`✓ 碼率已調整為 ${bitrateCapture} Kbps`)}
            className="min-h-[44px] glass border border-vbs-accent/40 text-vbs-accent text-sm font-bold rounded-xl
              hover:bg-vbs-accent/15 hover:border-vbs-accent/70 transition-all active:scale-95">
            套用碼率
          </button>
          <button id="reboot-capture" onClick={() => showToast('⟳ CAPTURE 重啟指令已發送')}
            className="min-h-[44px] glass border border-vbs-pgm/30 text-vbs-pgm text-sm font-bold rounded-xl
              hover:bg-vbs-pgm/10 hover:border-vbs-pgm/60 transition-all active:scale-95">
            遠端重啟
          </button>
        </div>
      </div>

      {/* ROUTE */}
      <div className="glass rounded-xl p-4 flex flex-col gap-4">
        <div className="flex items-center gap-2">
          <span className="w-2 h-2 rounded-full bg-purple-400 animate-pulse-slow shrink-0" />
          <span className="text-sm font-black text-purple-400 tracking-widest">ROUTE 端</span>
        </div>
        <div>
          <div className="flex items-center justify-between mb-1">
            <label className="text-sm font-semibold text-vbs-muted">SRT 緩衝延遲</label>
            <span className="text-sm font-bold text-vbs-text">{latencyRoute} ms</span>
          </div>
          <input id="route-latency" type="range" min={80} max={1000} step={20}
            value={latencyRoute} onChange={e => setLatencyRoute(Number(e.target.value))}
            className="w-full h-2 accent-purple-400" />
          <div className="flex justify-between text-xs font-medium text-vbs-muted mt-0.5">
            <span>80 ms</span><span>1000 ms</span>
          </div>
        </div>
        <button id="apply-latency-route" onClick={() => showToast(`✓ Route 緩衝延遲設為 ${latencyRoute} ms`)}
          className="min-h-[44px] glass border border-purple-400/40 text-purple-400 text-sm font-bold rounded-xl
            hover:bg-purple-400/10 hover:border-purple-400/60 transition-all active:scale-95">
          套用延遲
        </button>
      </div>

      {/* ENGINE */}
      <div className="glass rounded-xl p-4 flex flex-col gap-4">
        <div className="flex items-center gap-2">
          <span className="w-2 h-2 rounded-full bg-amber-400 animate-pulse-slow shrink-0" />
          <span className="text-sm font-black text-amber-400 tracking-widest">ENGINE 端</span>
        </div>
        <div>
          <label className="text-sm font-semibold text-vbs-muted block mb-1">PGM SRT 推流目標</label>
          <input id="pgm-url" type="text" value={pgmUrl} onChange={e => setPgmUrl(e.target.value)}
            className="w-full glass-dark border border-white/10 rounded-xl px-3 py-2.5
              text-sm font-medium text-vbs-text bg-transparent outline-none
              focus:border-amber-400/50 transition-colors" />
        </div>
        <div>
          <div className="flex items-center justify-between mb-1">
            <label className="text-sm font-semibold text-vbs-muted">PGM SRT Latency</label>
            <span className="text-sm font-bold text-vbs-text">{latencyPgm} ms</span>
          </div>
          <input id="pgm-latency" type="range" min={80} max={800} step={20}
            value={latencyPgm} onChange={e => setLatencyPgm(Number(e.target.value))}
            className="w-full h-2 accent-amber-400" />
        </div>
        {/* 手機 3 按鈕 */}
        <div className="grid grid-cols-3 gap-2">
          <button id="toggle-stream"
            onClick={() => { setStreaming(s => !s); showToast(streaming ? '⏹ PGM 推流已停止' : '▶ PGM 推流已啟動') }}
            className={`min-h-[44px] text-sm font-bold rounded-xl border transition-all active:scale-95
              ${streaming
                ? 'glass border-vbs-pgm/50 text-vbs-pgm hover:bg-vbs-pgm/10'
                : 'glass border-vbs-pvw/50 text-vbs-pvw hover:bg-vbs-pvw/10'}`}>
            {streaming ? '⏹ 停止' : '▶ 啟動'}
          </button>
          <button id="apply-pgm-settings" onClick={() => showToast('✓ PGM 參數已套用')}
            className="min-h-[44px] glass border border-amber-400/40 text-amber-400 text-sm font-bold rounded-xl
              hover:bg-amber-400/10 hover:border-amber-400/60 transition-all active:scale-95">
            套用設定
          </button>
          <button id="reset-engine" onClick={() => showToast('⟳ Engine 核心重置中…')}
            className="min-h-[44px] glass border border-vbs-pgm/30 text-vbs-pgm text-sm font-bold rounded-xl
              hover:bg-vbs-pgm/10 hover:border-vbs-pgm/60 transition-all active:scale-95">
            重置核心
          </button>
        </div>
      </div>

      {/* Toast */}
      {toast && (
        <div className="fixed bottom-20 lg:bottom-6 left-1/2 -translate-x-1/2 glass border border-vbs-pvw/30
          px-5 py-2.5 rounded-xl text-[9px\] font-semibold text-vbs-text animate-slide-in shadow-pvw z-50 whitespace-nowrap">
          {toast}
        </div>
      )}
    </div>
  )
}
