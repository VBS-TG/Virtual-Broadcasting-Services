import { useState, useEffect } from 'react'
import { useTelemetryStore } from '../stores/telemetryStore'
import type { NodeTelemetry } from '../types'

interface TelemetryPanelProps { compact?: boolean }

function Gauge({ label, value, max = 100, unit = '%', warn = 70, danger = 85 }: {
  label: string; value: number; max?: number; unit?: string; warn?: number; danger?: number
}) {
  const pct = Math.min(100, (value / max) * 100)
  const c = pct >= danger ? '#EF4444' : pct >= warn ? '#F59E0B' : '#10B981'
  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-center justify-between">
        <span className="text-[15px] font-medium text-vbs-muted">{label}</span>
        <span className="text-[15px] font-bold tabular-nums" style={{ color: c }}>
          {value.toFixed(unit === '%' ? 0 : 1)}{unit}
        </span>
      </div>
      <div className="h-1.5 rounded-full bg-white/5 overflow-hidden">
        <div className="h-full rounded-full transition-all duration-500" style={{ width: `${pct}%`, backgroundColor: c }} />
      </div>
    </div>
  )
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-[15px] font-medium text-vbs-muted">{label}</span>
      <span className="text-[15px] font-semibold tabular-nums text-vbs-text">{value}</span>
    </div>
  )
}

function NodeCard({ title, dotColor, node }: {
  title: string; dotColor: string; node: NodeTelemetry | null
}) {
  if (!node) {
    return (
      <div className="glass-light rounded-xl p-3 flex flex-col gap-2.5">
        <div className="flex items-center gap-2">
          <span className="w-2 h-2 rounded-full bg-vbs-muted shrink-0" />
          <span className="text-[15px] font-bold tracking-widest text-vbs-muted">{title}</span>
        </div>
        <p className="text-[15px] text-vbs-muted">離線 / 無資料</p>
      </div>
    )
  }
  return (
    <div className="glass-light rounded-xl p-3 flex flex-col gap-2.5">
      <div className="flex items-center gap-2">
        <span className="w-2 h-2 rounded-full animate-pulse-slow shrink-0" style={{ backgroundColor: dotColor }} />
        <span className="text-[15px] font-bold tracking-widest" style={{ color: dotColor }}>{title}</span>
        <span className={`ml-auto text-[15px] font-black px-1.5 py-0.5 rounded-md
          ${node.online ? 'bg-vbs-pvw/20 text-vbs-pvw' : 'bg-vbs-pgm/20 text-vbs-pgm'}`}>
          {node.online ? 'ONLINE' : 'OFFLINE'}
        </span>
      </div>
      <Gauge label="CPU" value={node.cpu_pct} unit="%" />
      <Gauge label="MEM" value={node.mem_pct} unit="%" warn={75} danger={90} />
      <InfoRow label="吞吐量" value={`${node.throughput_mbps.toFixed(1)} Mbps`} />
      {node.fps !== undefined && <InfoRow label="FPS" value={`${node.fps.toFixed(0)} fps`} />}
      {node.temp_c !== undefined && (
        <Gauge label="溫度" value={node.temp_c} unit="°C" max={100} warn={65} danger={80} />
      )}
      {node.extra && Object.entries(node.extra).map(([k, v]) => (
        <InfoRow key={k} label={k} value={String(v)} />
      ))}
    </div>
  )
}

// 視覺化裝飾：8ch 活動指示條
function DecoderBar() {
  const [h, setH] = useState(60 + Math.random() * 40)
  useEffect(() => {
    const t = setInterval(() => setH(50 + Math.random() * 50), 800 + Math.random() * 400)
    return () => clearInterval(t)
  }, [])
  return (
    <div className="flex-1 bg-white/5 rounded-sm overflow-hidden relative" style={{ height: '100%' }}>
      <div className="absolute bottom-0 w-full rounded-sm transition-all duration-300"
        style={{ height: `${h}%`, backgroundColor: h > 90 ? '#EF4444' : h > 70 ? '#F59E0B' : '#10B981' }} />
    </div>
  )
}

export default function TelemetryPanel({ compact }: TelemetryPanelProps) {
  const { data } = useTelemetryStore()

  return (
    <div className={compact ? '' : 'p-3 md:p-4'}>
      {!compact && (
        <h2 className="text-[15px] font-bold text-vbs-muted uppercase tracking-widest mb-3">遙測匯聚監控</h2>
      )}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
        <NodeCard title="VBS-CAPTURE" dotColor="#1E90FF" node={data?.capture ?? null} />
        <NodeCard title="VBS-ROUTE"   dotColor="#A78BFA" node={data?.route   ?? null} />
        <NodeCard title="VBS-ENGINE"  dotColor="#F59E0B" node={data?.engine  ?? null} />
      </div>
      {/* 8ch 解碼器視覺化（ENGINE 裝飾，保留原有效果） */}
      {!compact && (
        <div className="mt-3 glass rounded-xl p-3">
          <p className="text-[15px] font-bold text-vbs-muted uppercase tracking-widest mb-2">8ch 解碼幀率</p>
          <div className="flex gap-0.5 items-end h-6">
            {Array.from({ length: 8 }, (_, i) => <DecoderBar key={i} />)}
          </div>
        </div>
      )}
    </div>
  )
}
