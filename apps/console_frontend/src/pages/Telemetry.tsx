import { useEffect, useState } from 'react'
import { useTelemetryStore } from '../stores/telemetryStore'
import TelemetryPanel from '../components/TelemetryPanel'
import PageShell from '../components/PageShell'
import { Activity, RefreshCw } from 'lucide-react'
import { postCaptureBitrate, postCaptureReboot, postEnginePGMOutput, postEngineReset, postRouteBuffer } from '../lib/apiClient'
import { useOperationLogStore } from '../stores/operationLogStore'

export default function Telemetry() {
  const { autoRefresh, setAutoRefresh, fetch, refreshInterval, setRefreshInterval, error } =
    useTelemetryStore()
  const data = useTelemetryStore((s) => s.data)
  const [captureBitrate, setCaptureBitrate] = useState(3500)
  const [routeLatency, setRouteLatency] = useState(2000)
  const [routeLossTTL, setRouteLossTTL] = useState(40)
  const [pgmEnabled, setPgmEnabled] = useState(false)
  const [pgmURL, setPgmURL] = useState('')

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

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          <div className="glass rounded-[32px] p-6 border border-white/5 shadow-xl">
            <span className="text-[12px] font-black text-vbs-muted uppercase tracking-widest">Capture 遙測與控制</span>
            <p className="text-[13px] text-white/70 mt-3 leading-relaxed">
              監看 CPU 溫度、編碼 FPS、即時碼率與鏈路品質，並提供動態碼率與重啟控制。
            </p>
            <div className="mt-4 text-[12px] text-vbs-muted space-y-1">
              <div>CPU: {(data?.capture?.cpu_pct ?? 0).toFixed(0)}%</div>
              <div>TEMP: {data?.capture?.temp_c != null ? `${data.capture.temp_c.toFixed(1)}°C` : 'N/A'}</div>
              <div>FPS: {data?.capture?.fps != null ? data.capture.fps.toFixed(0) : 'N/A'}</div>
              <div>吞吐: {(data?.capture?.throughput_mbps ?? 0).toFixed(2)} Mbps</div>
            </div>
            <div className="mt-4 flex items-center gap-2">
              <input type="number" value={captureBitrate} onChange={(e) => setCaptureBitrate(Number(e.target.value) || 0)}
                className="w-28 glass-dark border border-white/10 rounded-lg px-2 py-1 text-white" />
              <button
                onClick={async () => {
                  const res = await postCaptureBitrate(captureBitrate)
                  useOperationLogStore.getState().add('POST /capture/bitrate', JSON.stringify({ bitrate_kbps: captureBitrate }), res.error ? 'error' : 'success', res.error)
                  alert(res.error ? `設定失敗：${res.error}` : 'Capture bitrate 已送出')
                }}
                className="px-3 py-1.5 rounded-lg border border-white/10 bg-white/5 text-white text-[11px] font-black"
              >設定碼率</button>
              <button
                onClick={async () => {
                  const res = await postCaptureReboot('manual-from-telemetry')
                  useOperationLogStore.getState().add('POST /capture/reboot', '{"reason":"manual-from-telemetry"}', res.error ? 'error' : 'success', res.error)
                  alert(res.error ? `重啟失敗：${res.error}` : 'Capture reboot 已送出')
                }}
                className="px-3 py-1.5 rounded-lg border border-vbs-pgm/30 bg-vbs-pgm/10 text-vbs-pgm text-[11px] font-black"
              >重啟</button>
            </div>
          </div>

          <div className="glass rounded-[32px] p-6 border border-white/5 shadow-xl">
            <span className="text-[12px] font-black text-vbs-muted uppercase tracking-widest">Route 遙測與控制</span>
            <p className="text-[13px] text-white/70 mt-3 leading-relaxed">
              監看雲端負載、匯聚吞吐與排序品質，並可調整 SRT 緩衝參數。
            </p>
            <div className="mt-4 text-[12px] text-vbs-muted space-y-1">
              <div>CPU: {(data?.route?.cpu_pct ?? 0).toFixed(0)}%</div>
              <div>MEM: {(data?.route?.mem_pct ?? 0).toFixed(0)}%</div>
              <div>吞吐: {(data?.route?.throughput_mbps ?? 0).toFixed(2)} Mbps</div>
              <div>下游連線: {String(data?.route?.extra?.has_engine_client ?? 'N/A')}</div>
            </div>
            <div className="mt-4 flex items-center gap-2">
              <input type="number" value={routeLatency} onChange={(e) => setRouteLatency(Number(e.target.value) || 0)}
                className="w-24 glass-dark border border-white/10 rounded-lg px-2 py-1 text-white" />
              <input type="number" value={routeLossTTL} onChange={(e) => setRouteLossTTL(Number(e.target.value) || 0)}
                className="w-20 glass-dark border border-white/10 rounded-lg px-2 py-1 text-white" />
              <button
                onClick={async () => {
                  const res = await postRouteBuffer(routeLatency, routeLossTTL)
                  useOperationLogStore.getState().add('POST /pgm/route-buffer', JSON.stringify({ latency_ms: routeLatency, loss_max_ttl: routeLossTTL }), res.error ? 'error' : 'success', res.error)
                  alert(res.error ? `套用失敗：${res.error}` : 'Route buffer 已套用')
                }}
                className="px-3 py-1.5 rounded-lg border border-white/10 bg-white/5 text-white text-[11px] font-black"
              >套用緩衝</button>
            </div>
          </div>

          <div className="glass rounded-[32px] p-6 border border-white/5 shadow-xl">
            <span className="text-[12px] font-black text-vbs-muted uppercase tracking-widest">Engine 遙測與控制</span>
            <p className="text-[13px] text-white/70 mt-3 leading-relaxed">
              監看 GPU/解碼負載與最終輸出狀態，並支援導播環境重置與 PGM 輸出控管。
            </p>
            <div className="mt-4 text-[12px] text-vbs-muted space-y-1">
              <div>CPU: {(data?.engine?.cpu_pct ?? 0).toFixed(0)}%</div>
              <div>TEMP: {data?.engine?.temp_c != null ? `${data.engine.temp_c.toFixed(1)}°C` : 'N/A'}</div>
              <div>FPS: {data?.engine?.fps != null ? data.engine.fps.toFixed(0) : 'N/A'}</div>
              <div>輸出吞吐: {(data?.engine?.throughput_mbps ?? 0).toFixed(2)} Mbps</div>
            </div>
            <div className="mt-4 flex flex-col gap-2">
              <button
                onClick={async () => {
                  const res = await postEngineReset()
                  useOperationLogStore.getState().add('POST /engine/reset', '{}', res.error ? 'error' : 'success', res.error)
                  alert(res.error ? `重置失敗：${res.error}` : 'Engine reset 已送出')
                }}
                className="px-3 py-1.5 rounded-lg border border-vbs-pgm/30 bg-vbs-pgm/10 text-vbs-pgm text-[11px] font-black"
              >重置導播環境</button>
              <div className="flex items-center gap-2">
                <input value={pgmURL} onChange={(e) => setPgmURL(e.target.value)} placeholder="srt://..."
                  className="flex-1 glass-dark border border-white/10 rounded-lg px-2 py-1 text-white" />
                <button
                  onClick={async () => {
                    const res = await postEnginePGMOutput(pgmEnabled, pgmURL)
                    useOperationLogStore.getState().add('POST /engine/pgm/output', JSON.stringify({ enabled: pgmEnabled, url: pgmURL }), res.error ? 'error' : 'success', res.error)
                    alert(res.error ? `設定失敗：${res.error}` : 'PGM output 控制已送出')
                  }}
                  className="px-3 py-1.5 rounded-lg border border-white/10 bg-white/5 text-white text-[11px] font-black"
                >送出</button>
              </div>
              <label className="text-[11px] text-vbs-muted flex items-center gap-2">
                <input type="checkbox" checked={pgmEnabled} onChange={(e) => setPgmEnabled(e.target.checked)} />
                啟用 PGM 輸出
              </label>
            </div>
          </div>
        </div>
      </div>
    </PageShell>
  )
}

