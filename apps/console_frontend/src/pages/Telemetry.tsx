import { useEffect } from 'react'
import { useTelemetryStore } from '../stores/telemetryStore'
import TelemetryPanel from '../components/TelemetryPanel'

export default function Telemetry() {
  const { autoRefresh, setAutoRefresh, fetch, refreshInterval, setRefreshInterval, error } =
    useTelemetryStore()

  // Auto Refresh 定時器
  useEffect(() => {
    fetch()
    if (!autoRefresh) return
    const t = setInterval(fetch, refreshInterval)
    return () => clearInterval(t)
  }, [autoRefresh, refreshInterval, fetch])

  return (
    <div className="h-full overflow-y-auto p-3 md:p-4 flex flex-col gap-3">
      {/* 頁頭 + 控制列 */}
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="text-[15px] font-black text-vbs-muted uppercase tracking-widest">遙測監控</h2>
        <div className="flex items-center gap-3">
          {error && <span className="text-[15px] text-vbs-pgm font-semibold"> {error}</span>}

          {/* 刷新間隔 */}
          <select
            id="telemetry-interval"
            value={refreshInterval}
            onChange={(e) => setRefreshInterval(Number(e.target.value))}
            className="glass-dark border border-white/10 rounded-lg px-2 py-1.5 text-[15px] text-vbs-text
              bg-transparent outline-none focus:border-vbs-accent/50 transition-colors"
          >
            <option value={500}>0.5s</option>
            <option value={1000}>1s</option>
            <option value={3000}>3s</option>
            <option value={5000}>5s</option>
          </select>

          {/* Auto Refresh Toggle */}
          <button
            id="telemetry-refresh-toggle"
            onClick={() => setAutoRefresh(!autoRefresh)}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-[15px] font-bold border transition-all
              ${autoRefresh
                ? 'bg-vbs-pvw/20 border-vbs-pvw/50 text-vbs-pvw'
                : 'glass-dark border-white/10 text-vbs-muted hover:text-vbs-text'}`}
          >
            <span className={`w-1.5 h-1.5 rounded-full ${autoRefresh ? 'bg-vbs-pvw animate-pulse-slow' : 'bg-vbs-muted'}`} />
            {autoRefresh ? '自動刷新中' : '已暫停'}
          </button>

          {/* 手動刷新 */}
          <button
            id="telemetry-manual-refresh"
            onClick={fetch}
            className="glass-dark border border-white/10 rounded-lg px-3 py-1.5 text-[15px] text-vbs-muted
              hover:text-vbs-text transition-colors font-semibold"
          >
             立即更新
          </button>
        </div>
      </div>

      <TelemetryPanel />
    </div>
  )
}
