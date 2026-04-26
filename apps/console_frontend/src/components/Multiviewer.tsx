import { useEffect, useRef, useState } from 'react'

interface MultiviewerProps {
  pgm: number
  pvw: number
  compact?: boolean
  fullScreen?: boolean
}

const CELLS = [
  { id: 1, label: 'CAM-1', color: '#0a1628' },
  { id: 2, label: 'CAM-2', color: '#0d1a2e' },
  { id: 3, label: 'CAM-3', color: '#1a0d28' },
  { id: 4, label: 'CAM-4', color: '#1e1408' },
  { id: 5, label: 'HDMI-5', color: '#0a1520' },
  { id: 6, label: 'FILE-6', color: '#0a1e14' },
  { id: 7, label: 'GFX-7', color: '#151510' },
  { id: 8, label: 'BLK-8', color: '#080808' },
]

export default function Multiviewer({ pgm, pvw, compact, fullScreen }: MultiviewerProps) {
  const pgmCell = CELLS.find(c => c.id === pgm) || CELLS[0]
  const pvwCell = CELLS.find(c => c.id === pvw) || CELLS[1]

  return (
    <div className={`flex flex-col ${fullScreen ? 'w-full h-full p-1' : `glass rounded-xl ${compact ? 'p-3' : 'p-4'} h-full w-full`}`}>
      {!fullScreen && (
        <div className="flex items-center justify-between shrink-0 mb-2">
          <span className="text-[12px] font-semibold text-vbs-muted uppercase tracking-widest">Multiviewer</span>
          <span className="text-[12px] text-vbs-muted hidden sm:inline">WHEP WebRTC</span>
        </div>
      )}
      
      {/* Container ensures the 16:9 grid maximizes available space without exceeding it */}
      <div className="flex-1 min-h-0 w-full flex items-center justify-center overflow-hidden bg-black/20 rounded-lg">
        {/* The 4x4 Grid which naturally forms a 16:9 ratio (4 cols * 16 = 64, 4 rows * 9 = 36 -> 16:9 aspect) */}
        <div className="grid grid-cols-4 grid-rows-4 gap-0.5 sm:gap-1 max-w-full max-h-full" style={{ aspectRatio: '16/9', height: '100%' }}>
          <div className="col-span-2 row-span-2 relative">
            <ViewCell cell={pvwCell} isPvw={true} isLarge={true} labelOverride="PVW" />
          </div>
          <div className="col-span-2 row-span-2 relative">
            <ViewCell cell={pgmCell} isPgm={true} isLarge={true} labelOverride="PGM" />
          </div>
          {CELLS.map((cell) => (
            <div key={cell.id} className="col-span-1 row-span-1">
              <ViewCell cell={cell} isPgm={pgm === cell.id} isPvw={pvw === cell.id} />
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

function ViewCell({ cell, isPgm, isPvw, isLarge, labelOverride }: { cell: typeof CELLS[0]; isPgm?: boolean; isPvw?: boolean; isLarge?: boolean; labelOverride?: string }) {
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
          ctx.beginPath(); ctx.arc(x, y, 5 + i * 3, 0, Math.PI * 2)
          ctx.fillStyle = 'rgba(255,255,255,0.035)'; ctx.fill()
        }
      }
      frame++; raf = requestAnimationFrame(draw)
    }
    draw()
    return () => cancelAnimationFrame(raf)
  }, [cell])

  useEffect(() => {
    if (cell.id === 8) return
    const t = setInterval(() => {
      setFps(58 + Math.floor(Math.random() * 4))
      setBitrate((b) => Math.max(3500, Math.min(8000, b + (Math.random() - 0.5) * 200)))
    }, 1000)
    return () => clearInterval(t)
  }, [cell.id])

  return (
    <div className={`relative w-full h-full rounded-md overflow-hidden border transition-all duration-300
      ${isPgm ? 'border-vbs-pgm shadow-pgm z-10' : isPvw ? 'border-vbs-pvw shadow-pvw z-10' : 'border-white/5'}`}>
      <canvas ref={canvasRef} width={320} height={180} className="w-full h-full object-cover" />

      <div className={`absolute bottom-0 left-0 right-0 flex items-center ${isLarge ? 'justify-center py-1.5 bg-black/70' : 'justify-between px-1.5 py-0.5 bg-black/60'} backdrop-blur-sm`}>
        <span className={`${isLarge ? 'text-[20px] tracking-widest' : 'text-[12px] sm:text-[15px]'} font-bold text-white`}>{labelOverride || cell.label}</span>
        {cell.id !== 8 && !isLarge && (
          <div className="hidden sm:flex items-center gap-1.5">
            <span className="text-[15px] text-vbs-pvw">{fps}fps</span>
            <span className="text-[15px] text-vbs-muted">{(bitrate / 1000).toFixed(1)}M</span>
          </div>
        )}
      </div>

      {isPgm && !isLarge && (
        <div className="absolute top-1 left-1 px-1.5 py-0.5 rounded-sm bg-vbs-pgm">
          <span className="text-[12px] sm:text-[15px] font-black text-white">PGM</span>
        </div>
      )}
      {isPvw && !isPgm && !isLarge && (
        <div className="absolute top-1 left-1 px-1.5 py-0.5 rounded-sm bg-vbs-pvw">
          <span className="text-[12px] sm:text-[15px] font-black text-white">PVW</span>
        </div>
      )}

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
