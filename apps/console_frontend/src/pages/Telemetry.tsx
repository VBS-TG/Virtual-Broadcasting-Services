import { useEffect } from 'react'
import { useTelemetryStore } from '../stores/telemetryStore'
import TelemetryPanel from '../components/TelemetryPanel'
import PageShell from '../components/PageShell'
import { Activity, RefreshCw } from 'lucide-react'

export default function Telemetry() {
  const { autoRefresh, setAutoRefresh, fetch, refreshInterval, setRefreshInterval, error } =
    useTelemetryStore()

  useEffect(() => {
    fetch()
    if (!autoRefresh) return
    const t = setInterval(fetch, refreshInterval)
    return () => clearInterval(t)
  }, [autoRefresh, refreshInterval, fetch])

  return (
    <PageShell
      title="遙測"
      description="System Resource Metrics"
      extra={
        <div className="flex items-center gap-3">
          {error && <span className="text-[12px] text-vbs-pgm font-black uppercase tracking-widest mr-2">{error}</span>}
          
          <select
            value={refreshInterval}
            onChange={(e) => setRefreshInterval(Number(e.target.value))}
            className="glass-dark border border-white/10 rounded-xl px-3 py-1.5 text-[12px] font-black text-vbs-muted bg-transparent outline-none focus:border-vbs-accent/50 transition-all uppercase tracking-widest"
          >
            <option value={500}>0.5s</option>
            <option value={1000}>1s</option>
            <option value={3000}>3s</option>
            <option value={5000}>5s</option>
          </select>

          <button
            onClick={() => setAutoRefresh(!autoRefresh)}
            className={`flex items-center gap-2 px-4 py-1.5 rounded-xl text-[12px] font-black border transition-all uppercase tracking-widest
              ${autoRefresh
                ? 'bg-vbs-pvw/10 border-vbs-pvw/30 text-vbs-pvw'
                : 'glass-dark border-white/10 text-vbs-muted hover:text-white'}`}
          >
            <span className={`w-1.5 h-1.5 rounded-full ${autoRefresh ? 'bg-vbs-pvw animate-pulse' : 'bg-vbs-muted'}`} />
            {autoRefresh ? 'Live' : 'Paused'}
          </button>

          <button
            onClick={fetch}
            className="w-10 h-10 glass-dark border border-white/10 rounded-xl flex items-center justify-center text-vbs-muted hover:text-white transition-all shadow-lg"
            title="立即刷新"
          >
             <RefreshCw className="w-4 h-4" />
          </button>
        </div>
      }
    >
      <div className="flex flex-col gap-6">
        <div className="glass rounded-[40px] p-8 shadow-2xl relative overflow-hidden">
          <div className="flex items-center gap-3 mb-8">
            <Activity className="w-6 h-6 text-vbs-accent" />
            <h2 className="text-2xl font-black text-white uppercase tracking-tighter">節點資源數據摘要</h2>
          </div>
          <TelemetryPanel />
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
           <div className="glass rounded-[32px] p-8 border border-white/5 shadow-xl">
             <span className="text-[12px] font-black text-vbs-muted uppercase tracking-widest">系統健康狀態監控</span>
             <p className="text-[14px] font-bold text-white mt-4 leading-relaxed opacity-60">
               目前所有核心服務節點（Capture, Route, Engine）通訊正常。此數據流由遙測代理自動彙總並推送至 Console 控制面，確保全鏈路資源負載可視化。
             </p>
           </div>
           <div className="glass rounded-[32px] p-8 border border-white/5 shadow-xl">
             <span className="text-[12px] font-black text-vbs-muted uppercase tracking-widest">效能優化建議</span>
             <p className="text-[14px] font-bold text-white mt-4 leading-relaxed opacity-60">
               未偵測到異常資源佔用或網路壅塞。建議在高頻交易期間縮短刷新間隔（0.5s）以獲得更細緻的負載曲線觀察。
             </p>
           </div>
        </div>
      </div>
    </PageShell>
  )
}

