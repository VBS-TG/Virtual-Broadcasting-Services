import { useEffect, useRef, useState } from 'react'

interface MultiviewerProps {
  pgm: number
  compact?: boolean
}

const CELLS = [
  { id: 1, label: 'CAM-1',  color: '#0a1628' },
  { id: 2, label: 'CAM-2',  color: '#0d1a2e' },
  { id: 3, label: 'CAM-3',  color: '#1a0d28' },
  { id: 4, label: 'CAM-4',  color: '#1e1408' },
  { id: 5, label: 'HDMI-5', color: '#0a1520' },
  { id: 6, label: 'FILE-6', color: '#0a1e14' },
  { id: 7, label: 'GFX-7',  color: '#151510' },
  { id: 8, label: 'BLK-8',  color: '#080808' },
]

export default function Multiviewer({ pgm, compact }: MultiviewerProps) {
  return (
    <div className={`glass rounded-xl flex flex-col gap-2 ${compact ? 'p-3' : 'p-4'}`}>
      <div className="flex items-center justify-between shrink-0">
        <span className="text-sm font-semibold text-vbs-muted uppercase tracking-widest">Multiviewer</span>
        <span className="text-sm text-vbs-muted hidden sm:inline">WHEP · WebRTC</span>
      </div>

      {/*
        手機 (default): 2×4 (2欄)
        平板 (sm):      4×2 (4欄)
      */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-1.5 md:gap-2">
        {CELLS.map(cell => (
          <ViewCell key={cell.id} cell={cell} isPgm={pgm === cell.id} />
        ))}
      </div>
    </div>
  )
}

function ViewCell({ cell, isPgm }: { cell: typeof CELLS[0]; isPgm: boolean }) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const [fps, setFps] = useState(60)
  const [bitrate, setBitrate] = useState(Math.floor(Math.random() * 3000 + 4000))

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const ctx = canvas.getContext('2d')!
    let frame = 0
    let raf: number
    const draw = () => {
      const w = canvas.width, h = canvas.height
      const grad = ctx.createLinearGradient(0, 0, w, h)
      grad.addColorStop(0, cell.color)
      grad.addColorStop(1, '#050508')
      ctx.fillStyle = grad
      ctx.fillRect(0, 0, w, h)
      ctx.fillStyle = 'rgba(0,0,0,0.12)'
      for (let y = 0; y < h; y += 4) ctx.fillRect(0, y, w, 1)
      if (cell.id !== 8) {
        for (let i = 0; i < 3; i++) {
          const x = (Math.sin(frame * 0.02 + i * 2.1) * 0.3 + 0.5) * w
          const y = (Math.cos(frame * 0.015 + i * 1.7) * 0.3 + 0.5) * h
          ctx.beginPath()
          ctx.arc(x, y, 5 + i * 3, 0, Math.PI * 2)
          ctx.fillStyle = 'rgba(255,255,255,0.035)'
          ctx.fill()
        }
      }
      frame++
      raf = requestAnimationFrame(draw)
    }
    draw()
    return () => cancelAnimationFrame(raf)
  }, [cell])

  useEffect(() => {
    if (cell.id === 8) return
    const t = setInterval(() => {
      setFps(58 + Math.floor(Math.random() * 4))
      setBitrate(b => Math.max(3500, Math.min(8000, b + (Math.random() - 0.5) * 200)))
    }, 1000)
    return () => clearInterval(t)
  }, [cell.id])

  return (
    <div className={`relative rounded-lg overflow-hidden border transition-all duration-300
      ${isPgm ? 'border-vbs-pgm shadow-pgm' : 'border-white/5'}`}
      style={{ aspectRatio: '16/9' }}
    >
      <canvas ref={canvasRef} width={320} height={180} className="w-full h-full object-cover" />

      {/* 底部資訊條 */}
      <div className="absolute bottom-0 left-0 right-0 flex items-center justify-between px-1.5 py-0.5 bg-black/60 backdrop-blur-sm">
        <span className="text-xs font-bold text-white">{cell.label}</span>
        {cell.id !== 8 && (
          <div className="hidden sm:flex items-center gap-1.5">
            <span className="text-xs text-vbs-pvw">{fps}fps</span>
            <span className="text-xs text-vbs-muted">{(bitrate / 1000).toFixed(1)}M</span>
          </div>
        )}
      </div>

      {/* PGM badge */}
      {isPgm && (
        <div className="absolute top-1 left-1 px-1.5 py-0.5 rounded-sm bg-vbs-pgm animate-pulse-pgm">
          <span className="text-xs font-black text-white">PGM</span>
        </div>
      )}

      {/* VU */}
      {cell.id !== 8 && (
        <div className="absolute top-1 right-1 flex items-end gap-[2px] h-4">
          <VuBar /><VuBar />
        </div>
      )}
    </div>
  )
}

function VuBar() {
  const [h, setH] = useState(40)
  useEffect(() => {
    const t = setInterval(() => setH(20 + Math.random() * 70), 100 + Math.random() * 50)
    return () => clearInterval(t)
  }, [])
  return (
    <div className="w-1 bg-black/40 rounded-full overflow-hidden h-full flex items-end">
      <div className="w-full rounded-full transition-all duration-75"
        style={{ height: `${h}%`, backgroundColor: h > 80 ? '#EF4444' : h > 60 ? '#F59E0B' : '#10B981' }} />
    </div>
  )
}
