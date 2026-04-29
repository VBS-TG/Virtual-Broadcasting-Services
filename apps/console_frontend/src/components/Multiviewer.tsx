import { useEffect, useRef, useState } from 'react'

interface MultiviewerProps {
  pgm: number
  pvw: number
  compact?: boolean
  fullScreen?: boolean
  inputLabels?: string[]
  cellSources?: string[]
  editable?: boolean
  onCellSourceChange?: (cellIndex: number, source: string) => void
  buttonMappings?: string[]
  activeCellIndex?: number | null
  onActiveCellChange?: (cellIndex: number) => void
}

const CELL_COLORS = ['#0a1628', '#0d1a2e', '#1a0d28', '#1e1408', '#0a1520', '#0a1e14', '#151510', '#080808']
const WHEP_URL_TEMPLATE = String(import.meta.env.VITE_WHEP_URL_TEMPLATE ?? '').trim()

function sourceToIndex(source: string): number {
  const n = Number(String(source ?? '').replace('input', ''))
  if (!Number.isFinite(n) || n < 1 || n > 8) return 1
  return n
}

export default function Multiviewer({
  pgm,
  pvw,
  compact,
  fullScreen,
  inputLabels,
  cellSources,
  editable,
  onCellSourceChange,
  buttonMappings,
  activeCellIndex,
  onActiveCellChange,
}: MultiviewerProps) {
  const labels = Array.from({ length: 8 }, (_, i) => inputLabels?.[i] || `Source${i + 1}`)
  const mappedCells = Array.from({ length: 8 }, (_, i) => {
    const src = cellSources?.[i] || `input${i + 1}`
    const srcIdx = sourceToIndex(src)
    return { id: i + 1, source: src, sourceIndex: srcIdx, label: labels[srcIdx - 1], color: CELL_COLORS[(srcIdx - 1) % CELL_COLORS.length] }
  })
  const pgmCell = mappedCells.find(c => c.sourceIndex === pgm) || mappedCells[0]
  const pvwCell = mappedCells.find(c => c.sourceIndex === pvw) || mappedCells[1]
  const [pickerCell, setPickerCell] = useState<number | null>(null)
  const [selectedCell, setSelectedCell] = useState<number | null>(activeCellIndex ?? null)

  useEffect(() => {
    if (activeCellIndex != null) setSelectedCell(activeCellIndex)
  }, [activeCellIndex])

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
            <ViewCell cell={pvwCell} isPvw={true} isLarge={true} labelOverride={`PVW · ${pvwCell.label}`} />
          </div>
          <div className="col-span-2 row-span-2 relative">
            <ViewCell cell={pgmCell} isPgm={true} isLarge={true} labelOverride={`PGM · ${pgmCell.label}`} />
          </div>
          {mappedCells.map((cell, idx) => (
            <div key={cell.id} className="col-span-1 row-span-1">
              <button
                className={`w-full h-full ${selectedCell === idx ? 'ring-2 ring-vbs-accent rounded-md' : ''}`}
                onClick={() => {
                  if (!editable) return
                  setSelectedCell(idx)
                  onActiveCellChange?.(idx)
                  setPickerCell(idx)
                }}
              >
                <ViewCell cell={{ ...cell, id: cell.sourceIndex }} isPgm={pgm === cell.sourceIndex} isPvw={pvw === cell.sourceIndex} />
              </button>
            </div>
          ))}
        </div>
      </div>
      {editable && (
        <div className="mt-2 flex flex-wrap gap-2">
          {(buttonMappings ?? Array.from({ length: 8 }, (_, i) => `input${i + 1}`)).map((src, i) => (
            <button
              key={`map-btn-${i}`}
              onClick={() => {
                if (selectedCell == null) return
                onCellSourceChange?.(selectedCell, src)
              }}
              className="px-2 py-1 rounded border border-white/10 bg-white/5 text-white text-[11px]"
            >
              {`${i + 1}: ${src}`}
            </button>
          ))}
        </div>
      )}
      {pickerCell != null && editable && (
        <div className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center" onClick={() => setPickerCell(null)}>
          <div className="glass rounded-xl p-4 min-w-[300px]" onClick={(e) => e.stopPropagation()}>
            <div className="text-white font-black mb-3">{`指定 Cell ${pickerCell + 1} 來源`}</div>
            <div className="grid grid-cols-2 gap-2">
              {Array.from({ length: 8 }, (_, i) => `input${i + 1}`).map((src, idx) => (
                <button
                  key={src}
                  onClick={() => {
                    onCellSourceChange?.(pickerCell, src)
                    setPickerCell(null)
                  }}
                  className="px-2 py-2 rounded border border-white/10 bg-white/5 text-white text-[12px]"
                >
                  {`${src} (${labels[idx]})`}
                </button>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function buildWhepURL(sourceID: number): string {
  if (!WHEP_URL_TEMPLATE) return ''
  const source = `input${sourceID}`
  if (WHEP_URL_TEMPLATE.includes('{source}')) {
    return WHEP_URL_TEMPLATE.replaceAll('{source}', source)
  }
  return WHEP_URL_TEMPLATE
}

async function waitIceGathering(pc: RTCPeerConnection): Promise<void> {
  if (pc.iceGatheringState === 'complete') return
  await new Promise<void>((resolve) => {
    const onState = () => {
      if (pc.iceGatheringState === 'complete') {
        pc.removeEventListener('icegatheringstatechange', onState)
        resolve()
      }
    }
    pc.addEventListener('icegatheringstatechange', onState)
    setTimeout(() => {
      pc.removeEventListener('icegatheringstatechange', onState)
      resolve()
    }, 1500)
  })
}

function looksLikeSDP(raw: string): boolean {
  const s = String(raw ?? '').trim()
  return s.startsWith('v=0')
}

async function postServerOfferInit(whepURL: string): Promise<{ offerSDP: string; resourceURL: string }> {
  const res = await fetch(whepURL, {
    method: 'POST',
    headers: { 'Content-Type': 'application/sdp' },
    body: '',
  })
  const body = await res.text()
  if (!res.ok || !looksLikeSDP(body)) {
    throw new Error(`server-init unavailable status=${res.status}`)
  }
  const location = String(res.headers.get('Location') ?? '').trim()
  const resourceURL = location ? new URL(location, whepURL).toString() : whepURL
  return { offerSDP: body, resourceURL }
}

async function sendAnswerToResource(resourceURL: string, answerSDP: string): Promise<void> {
  const methods: Array<'PATCH' | 'POST' | 'PUT'> = ['PATCH', 'POST', 'PUT']
  let lastErr = ''
  for (const method of methods) {
    const res = await fetch(resourceURL, {
      method,
      headers: { 'Content-Type': 'application/sdp' },
      body: answerSDP,
    })
    if (res.ok) return
    lastErr = `${method}:${res.status}`
  }
  throw new Error(`send answer failed ${lastErr}`)
}

function ViewCell({ cell, isPgm, isPvw, isLarge, labelOverride }: { cell: { id: number; sourceIndex?: number; label: string; color: string }; isPgm?: boolean; isPvw?: boolean; isLarge?: boolean; labelOverride?: string }) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const videoRef = useRef<HTMLVideoElement>(null)
  const [fps, setFps] = useState(60)
  const [bitrate, setBitrate] = useState(Math.floor(Math.random() * 3000 + 4000))
  const [streamReady, setStreamReady] = useState(false)
  const mappedSource = Number(cell.sourceIndex ?? cell.id)
  const whepURL = buildWhepURL(mappedSource)

  useEffect(() => {
    if (whepURL) return
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
      if (mappedSource !== 8) {
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
  }, [cell, whepURL, mappedSource])

  useEffect(() => {
    if (!whepURL) {
      setStreamReady(false)
      return
    }
    const video = videoRef.current
    if (!video) return
    let resourceURL = ''
    const pc = new RTCPeerConnection()
    let cancelled = false

    pc.ontrack = (ev) => {
      if (!video) return
      video.srcObject = ev.streams?.[0] ?? null
      setStreamReady(true)
    }

    ;(async () => {
      try {
        pc.addTransceiver('video', { direction: 'recvonly' })
        // Prefer server-init WHEP (srt-whep), fallback to client-init.
        try {
          const { offerSDP, resourceURL: sessionURL } = await postServerOfferInit(whepURL)
          resourceURL = sessionURL
          if (cancelled) return
          await pc.setRemoteDescription({ type: 'offer', sdp: offerSDP })
          const answer = await pc.createAnswer()
          await pc.setLocalDescription(answer)
          await waitIceGathering(pc)
          const localAnswer = pc.localDescription?.sdp ?? ''
          if (!localAnswer) throw new Error('missing local answer sdp')
          await sendAnswerToResource(resourceURL, localAnswer)
        } catch {
          const offer = await pc.createOffer()
          await pc.setLocalDescription(offer)
          await waitIceGathering(pc)
          const sdp = pc.localDescription?.sdp ?? ''
          if (!sdp) throw new Error('missing local sdp')
          const res = await fetch(whepURL, {
            method: 'POST',
            headers: { 'Content-Type': 'application/sdp' },
            body: sdp,
          })
          if (!res.ok) throw new Error(`WHEP status ${res.status}`)
          const answerSDP = await res.text()
          const location = String(res.headers.get('Location') ?? '').trim()
          if (location) resourceURL = new URL(location, whepURL).toString()
          if (!cancelled) {
            await pc.setRemoteDescription({ type: 'answer', sdp: answerSDP })
          }
        }
        if (!cancelled) {
          setStreamReady(true)
        }
      } catch {
        setStreamReady(false)
      }
    })()

    return () => {
      cancelled = true
      const stream = video.srcObject as MediaStream | null
      stream?.getTracks().forEach((t) => t.stop())
      video.srcObject = null
      pc.close()
      if (resourceURL) {
        fetch(resourceURL, { method: 'DELETE' }).catch(() => undefined)
      }
    }
  }, [whepURL])

  useEffect(() => {
    if (mappedSource === 8) return
    const t = setInterval(() => {
      setFps(58 + Math.floor(Math.random() * 4))
      setBitrate((b) => Math.max(3500, Math.min(8000, b + (Math.random() - 0.5) * 200)))
    }, 1000)
    return () => clearInterval(t)
  }, [mappedSource])

  return (
    <div className={`relative w-full h-full rounded-md overflow-hidden border transition-all duration-300
      ${isPgm ? 'border-vbs-pgm shadow-pgm z-10' : isPvw ? 'border-vbs-pvw shadow-pvw z-10' : 'border-white/5'}`}>
      {streamReady ? (
        <video ref={videoRef} autoPlay muted playsInline className="w-full h-full object-cover" />
      ) : (
        <canvas ref={canvasRef} width={320} height={180} className="w-full h-full object-cover" />
      )}

      <div className={`absolute bottom-0 left-0 right-0 flex items-center ${isLarge ? 'justify-center py-1.5 bg-black/70' : 'justify-between px-1.5 py-0.5 bg-black/60'} backdrop-blur-sm`}>
        <span className={`${isLarge ? 'text-[20px] tracking-widest' : 'text-[12px] sm:text-[15px]'} font-bold text-white`}>{labelOverride || cell.label}</span>
        {mappedSource !== 8 && !isLarge && (
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

      {mappedSource !== 8 && (
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
