import { useState, useEffect } from 'react'

interface TelemetryPanelProps { compact?: boolean }

function useFluctuate(base: number, spread: number, interval = 1000) {
  const [val, setVal] = useState(base)
  useEffect(() => {
    const t = setInterval(() => setVal(Math.max(0, Math.min(200, base + (Math.random() - 0.5) * spread))), interval)
    return () => clearInterval(t)
  }, [base, spread, interval])
  return val
}

function Gauge({ label, value, max = 100, unit = '%', warn = 70, danger = 85, color }: {
  label: string; value: number; max?: number; unit?: string; warn?: number; danger?: number; color?: string
}) {
  const pct = Math.min(100, (value / max) * 100)
  const c = color ?? (pct >= danger ? '#EF4444' : pct >= warn ? '#F59E0B' : '#10B981')
  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium text-vbs-muted">{label}</span>
        <span className="text-sm font-bold tabular-nums" style={{ color: c }}>
          {value.toFixed(unit === '%' ? 0 : 1)}{unit}
        </span>
      </div>
      <div className="h-1.5 rounded-full bg-white/5 overflow-hidden">
        <div className="h-full rounded-full transition-all duration-500" style={{ width: `${pct}%`, backgroundColor: c }} />
      </div>
    </div>
  )
}

function NodeCard({ title, dotColor, children }: { title: string; dotColor: string; children: React.ReactNode }) {
  return (
    <div className="glass-light rounded-xl p-3 flex flex-col gap-2.5">
      <div className="flex items-center gap-2">
        <span className="w-2 h-2 rounded-full animate-pulse-slow shrink-0" style={{ backgroundColor: dotColor }} />
        <span className="text-sm font-bold tracking-widest" style={{ color: dotColor }}>{title}</span>
      </div>
      {children}
    </div>
  )
}

function InfoRow({ label, value, valueColor }: { label: string; value: string; valueColor?: string }) {
  return (
    <div className="flex items-center justify-between">
      <span className="text-sm font-medium text-vbs-muted">{label}</span>
      <span className={`text-sm font-semibold tabular-nums ${valueColor ?? 'text-vbs-text'}`}>{value}</span>
    </div>
  )
}

function CaptureCard() {
  const cpuTemp  = useFluctuate(58, 8)
  const encFps   = useFluctuate(60, 2)
  const bw0      = useFluctuate(12, 4)
  const bw1      = useFluctuate(8, 3)
  const dropPkts = useFluctuate(2, 3)
  return (
    <NodeCard title="VBS-CAPTURE" dotColor="#1E90FF">
      <Gauge label="CPU 溫度" value={cpuTemp} unit="°C" warn={65} danger={80} max={100} />
      <Gauge label="編碼幀率" value={encFps}  unit="fps" max={60} warn={55} danger={50} />
      <InfoRow label="NIC-0 上行"  value={`${bw0.toFixed(1)} Mbps`} valueColor="text-vbs-accent" />
      <InfoRow label="USB-5G 上行" value={`${bw1.toFixed(1)} Mbps`} valueColor="text-vbs-accent" />
      <InfoRow label="SRTLA 掉包" value={`${Math.round(dropPkts)} pkts`}
        valueColor={dropPkts > 5 ? 'text-vbs-pgm' : 'text-vbs-pvw'} />
      <p className="text-xs font-medium text-vbs-muted border-t border-white/5 pt-1.5">
        RK3588 · hevc_rkmpp · SRTLA · MTU 1400
      </p>
    </NodeCard>
  )
}

function RouteCard() {
  const cpu    = useFluctuate(28, 10)
  const ram    = useFluctuate(45, 8)
  const bwIn   = useFluctuate(22, 5)
  const pktErr = useFluctuate(0.3, 0.3)
  return (
    <NodeCard title="VBS-ROUTE" dotColor="#A78BFA">
      <Gauge label="CPU 使用率" value={cpu} unit="%" />
      <Gauge label="RAM 使用率" value={ram} unit="%" warn={75} danger={90} />
      <InfoRow label="SRTLA 總接收"    value={`${bwIn.toFixed(1)} Mbps`} valueColor="text-purple-400" />
      <InfoRow label="封包排序錯誤率" value={`${pktErr.toFixed(2)}%`}
        valueColor={pktErr > 1 ? 'text-vbs-pgm' : 'text-vbs-pvw'} />
      <InfoRow label="Engine 拉流"    value="CONNECTED" valueColor="text-vbs-pvw" />
      <p className="text-xs font-medium text-vbs-muted border-t border-white/5 pt-1.5">
        ap-northeast-1 · SRT Relay · AES-256
      </p>
    </NodeCard>
  )
}

function EngineCard() {
  const gpuTemp = useFluctuate(62, 10)
  const gpuLoad = useFluctuate(55, 15)
  const vram    = useFluctuate(4200, 500)
  const pgmFps  = useFluctuate(60, 1)
  return (
    <NodeCard title="VBS-ENGINE" dotColor="#F59E0B">
      <Gauge label="GPU 溫度" value={gpuTemp} unit="°C" warn={70} danger={85} max={100} />
      <Gauge label="GPU 負載" value={gpuLoad} unit="%" />
      <InfoRow label="顯存佔用"   value={`${(vram / 1024).toFixed(1)} GB`} valueColor="text-amber-400" />
      <InfoRow label="PGM 幀率"   value={`${pgmFps.toFixed(0)} fps`}       valueColor="text-vbs-pvw" />
      <InfoRow label="推流狀態"   value="SRT → ROUTE ✓"                    valueColor="text-vbs-pvw" />
      {/* 8-ch bar */}
      <div className="flex gap-0.5 items-end h-5 border-t border-white/5 pt-1.5">
        {Array.from({ length: 8 }, (_, i) => <DecoderBar key={i} />)}
      </div>
      <p className="text-xs font-medium text-vbs-muted -mt-1">8ch 解碼幀率</p>
      <p className="text-xs font-medium text-vbs-muted border-t border-white/5 pt-1.5">
        Eyevinn Open Live · WHEP · AES-256
      </p>
    </NodeCard>
  )
}

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
  return (
    <div className={compact ? '' : 'p-3 md:p-4'}>
      {!compact && (
        <h2 className="text-sm font-bold text-vbs-muted uppercase tracking-widest mb-3">遙測匯聚監控</h2>
      )}
      {/*
        手機 (default): 單欄
        平板 (md):      雙欄 (2+1 或 自動)
        桌面 (lg):      三欄
      */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">
        <CaptureCard />
        <RouteCard />
        <EngineCard />
      </div>
    </div>
  )
}
