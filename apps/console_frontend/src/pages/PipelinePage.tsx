import { useEffect, useState } from 'react'
import { Network, Activity, RotateCcw, Link2 } from 'lucide-react'
import { useShowConfigStore } from '../stores/showConfigStore'
import { useRuntimeStore } from '../stores/runtimeStore'
import { putRuntimeConfig, postSessionKey } from '../lib/apiClient'
import { useOperationLogStore } from '../stores/operationLogStore'
import PageShell from '../components/PageShell'

export default function PipelinePage() {
  const { draft, loading, saving, applying, error, fetch, updateDraft, saveDraft, applyDraft, rollback } = useShowConfigStore()
  const runtime = useRuntimeStore((s) => s.config)
  const fetchRuntime = useRuntimeStore((s) => s.fetch)

  useEffect(() => {
    fetch()
    fetchRuntime()
  }, [fetch, fetchRuntime])

  if (loading && !draft) {
    return (
      <PageShell title="鏈路" description="Pipeline Monitoring">
        <div className="h-[400px] flex items-center justify-center text-vbs-muted text-[15px] font-black uppercase tracking-widest animate-pulse">
          Loading Pipeline Architecture...
        </div>
      </PageShell>
    )
  }

  if (!draft) {
    return (
      <PageShell title="鏈路" description="Pipeline Monitoring">
        <div className="h-[400px] flex items-center justify-center text-vbs-pgm text-[15px] font-black uppercase tracking-widest">
          {error ?? 'Failed to load pipeline configuration'}
        </div>
      </PageShell>
    )
  }

  const t = draft.profile.target ?? { width: 1920, height: 1080, frame_rate: 60 }
  const runtimeInputs = Math.max(1, Math.min(8, runtime?.inputs ?? 8))
  const sourceOptions = Array.from({ length: 8 }, (_, i) => `input${i + 1}`)
  const currentAuxCount = Math.max(0, Math.min(4, runtime?.aux_count ?? 0))
  const currentPGMCount = Math.max(1, runtime?.pgm_count ?? 1)
  const [pendingPGMCount, setPendingPGMCount] = useState(currentPGMCount)
  const [pendingAUXCount, setPendingAUXCount] = useState(currentAuxCount)
  const [pendingAuxSources, setPendingAuxSources] = useState<Record<string, string>>(runtime?.aux_sources ?? {})

  useEffect(() => {
    setPendingPGMCount(currentPGMCount)
    setPendingAUXCount(currentAuxCount)
    setPendingAuxSources(runtime?.aux_sources ?? {})
  }, [currentPGMCount, currentAuxCount, runtime?.aux_sources])

  const setInputLabel = (slot: number, label: string) => {
    const key = `input${slot}`
    updateDraft((old) => {
      const next = [...(old.sources ?? [])]
      const idx = next.findIndex((s) => s.slot_id === key)
      if (idx >= 0) {
        next[idx] = { ...next[idx], display_name: label }
      } else {
        next.push({ slot_id: key, display_name: label, short_label: `S${slot}` })
      }
      return { ...old, sources: next }
    })
  }

  const getInputLabel = (slot: number): string => {
    const key = `input${slot}`
    const found = (draft.sources ?? []).find((s) => s.slot_id === key)
    return found?.display_name || `Source${slot}`
  }

  const setSwitcherButtonSource = (button: number, source: string) => {
    updateDraft((old) => {
      const rows = [...((old.switcher.rows as any[]) ?? [])]
      const row0 = { ...(rows[0] ?? {}), buttons: [...(rows[0]?.buttons ?? [])] }
      while (row0.buttons.length < 8) {
        row0.buttons.push({ id: `${row0.buttons.length + 1}`, source: `input${row0.buttons.length + 1}` })
      }
      row0.buttons[button - 1] = { ...row0.buttons[button - 1], id: String(button), source }
      rows[0] = row0
      return { ...old, switcher: { ...old.switcher, rows } }
    })
  }

  const getSwitcherButtonSource = (button: number): string => {
    const row0: any = ((draft.switcher.rows as any[]) ?? [])[0]
    const source = row0?.buttons?.[button - 1]?.source
    return typeof source === 'string' && source ? source : `input${button}`
  }

  const saveRuntimeRouting = async (nextPGMCount: number, nextAuxCount: number, auxSources: Record<string, string>) => {
    const res = await putRuntimeConfig({
      pgm_count: Math.max(1, nextPGMCount),
      aux_count: Math.max(0, Math.min(4, nextAuxCount)),
      aux_sources: auxSources,
    })
    useOperationLogStore.getState().add(
      'PUT /runtime/config',
      JSON.stringify({ pgm_count: Math.max(1, nextPGMCount), aux_count: Math.max(0, Math.min(4, nextAuxCount)), aux_sources: auxSources }),
      res.error ? 'error' : 'success',
      res.error
    )
    if (res.error) {
      alert(`儲存 PGM/AUX 路由失敗：${res.error}`)
      return
    }
    await fetchRuntime()
    alert('PGM/AUX 路由已儲存')
  }

  const generateSrtURL = async (kind: 'pgm' | 'aux', idx?: number) => {
    const res = await postSessionKey()
    if (res.error || !res.data?.passphrase) {
      alert(`產生 SRT 金鑰失敗：${res.error ?? 'unknown error'}`)
      return
    }
    const stream = kind === 'pgm' ? 'pgm' : `aux${idx ?? 1}`
    const srt = `srt://route.example.com:20030?streamid=${stream}&passphrase=${encodeURIComponent(res.data.passphrase)}`
    await navigator.clipboard.writeText(srt)
    alert(`${stream.toUpperCase()} SRT 已複製:\n${srt}`)
  }

  const applySectionDraft = async (name: string) => {
    const okSave = await saveDraft()
    if (!okSave) {
      alert(`${name} 儲存失敗`)
      return
    }
    const okApply = await applyDraft()
    if (!okApply) {
      alert(`${name} 套用失敗`)
      return
    }
    alert(`${name} 已套用`)
  }

  return (
    <PageShell
      title="鏈路"
      description="Pipeline Monitoring"
      extra={
        <div className="flex items-center gap-2">
          <span className="text-[10px] font-black text-vbs-muted uppercase tracking-widest">Profile Mode:</span>
          <span className="text-[12px] font-black text-white bg-white/5 px-3 py-1 rounded-lg uppercase tracking-tighter border border-white/5">{draft.profile.mode}</span>
        </div>
      }
    >
      <div className="grid grid-cols-1 md:grid-cols-12 gap-6">
        <div className="md:col-span-4 glass rounded-[32px] p-8 flex flex-col gap-6 shadow-xl relative overflow-hidden">
          <div className="flex items-center gap-3 mb-2">
            <Activity className="w-5 h-5 text-vbs-accent" />
            <span className="text-[14px] font-black text-white uppercase tracking-widest">全線畫質與 FPS</span>
          </div>
          <div className="grid grid-cols-1 gap-6">
            <div className="flex flex-col">
              <span className="text-[11px] font-black text-vbs-muted uppercase tracking-widest mb-1">解析度</span>
              <div className="flex items-center gap-2">
                <input type="number" value={t.width} className="w-28 glass-dark border border-white/10 rounded-lg px-2 py-1 text-white font-black"
                  onChange={(e) => updateDraft((old) => ({ ...old, profile: { ...old.profile, target: { ...(old.profile.target ?? t), width: Math.max(320, Number(e.target.value) || 1920) } } }))} />
                <span className="text-vbs-muted">x</span>
                <input type="number" value={t.height} className="w-28 glass-dark border border-white/10 rounded-lg px-2 py-1 text-white font-black"
                  onChange={(e) => updateDraft((old) => ({ ...old, profile: { ...old.profile, target: { ...(old.profile.target ?? t), height: Math.max(180, Number(e.target.value) || 1080) } } }))} />
              </div>
            </div>
            <div className="flex flex-col">
              <span className="text-[11px] font-black text-vbs-muted uppercase tracking-widest mb-1">幀率</span>
              <div className="flex items-center gap-2">
                <input type="number" value={t.frame_rate} className="w-24 glass-dark border border-white/10 rounded-lg px-2 py-1 text-vbs-pvw font-black"
                  onChange={(e) => updateDraft((old) => ({ ...old, profile: { ...old.profile, target: { ...(old.profile.target ?? t), frame_rate: Math.max(1, Number(e.target.value) || 60) } } }))} />
                <span className="text-[14px] ml-1 font-bold text-vbs-pvw">FPS</span>
              </div>
            </div>
          </div>
          <div className="flex justify-end">
            <button
              onClick={() => applySectionDraft('畫質/FPS')}
              disabled={saving || applying}
              className="text-[11px] font-black uppercase tracking-widest px-3 py-2 rounded-lg border border-vbs-pvw/40 bg-vbs-pvw/10 text-vbs-pvw disabled:opacity-50"
            >
              套用本區
            </button>
          </div>
        </div>

        <div className="md:col-span-8 glass rounded-[32px] p-8 shadow-xl flex flex-col gap-6">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <Network className="w-5 h-5 text-vbs-accent" />
              <span className="text-[14px] font-black text-white uppercase tracking-widest">Input Label（固定 8ch）</span>
            </div>
            <span className="text-[11px] text-vbs-muted">來源由系統自動偵測，僅可編輯名稱</span>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            {Array.from({ length: 8 }, (_, i) => i + 1).map((slot) => {
              const online = slot <= runtimeInputs
              return (
                <div key={`slot-${slot}`} className="glass-dark border border-white/5 rounded-2xl p-4 flex items-center gap-3">
                  <span className={`w-2.5 h-2.5 rounded-full ${online ? 'bg-vbs-pvw' : 'bg-white/30'}`} />
                  <span className="text-[11px] font-black text-vbs-muted uppercase w-16">{`INPUT${slot}`}</span>
                  <input
                    value={getInputLabel(slot)}
                    onChange={(e) => setInputLabel(slot, e.target.value)}
                    className="flex-1 text-[13px] font-black text-white bg-transparent border border-white/10 rounded-md px-2 py-1"
                  />
                </div>
              )
            })}
          </div>
          <div className="flex justify-end">
            <button
              onClick={() => applySectionDraft('Input Label')}
              disabled={saving || applying}
              className="text-[11px] font-black uppercase tracking-widest px-3 py-2 rounded-lg border border-vbs-pvw/40 bg-vbs-pvw/10 text-vbs-pvw disabled:opacity-50"
            >
              套用本區
            </button>
          </div>
        </div>

        <div className="md:col-span-6 glass rounded-[32px] p-8 shadow-xl flex flex-col gap-4">
          <h3 className="text-[14px] font-black text-white uppercase tracking-widest">AUX / PGM Mapping（Runtime）</h3>
          <div className="flex items-center gap-3">
            <span className="text-[11px] text-vbs-muted">PGM 條數</span>
            <input
              type="number"
              min={1}
              value={pendingPGMCount}
              onChange={(e) => setPendingPGMCount(Number(e.target.value) || 1)}
              className="w-20 glass-dark border border-white/10 rounded px-2 py-1 text-white"
            />
            <button onClick={() => generateSrtURL('pgm')} className="flex items-center gap-1 text-[11px] px-2 py-1 rounded border border-white/10 bg-white/5 text-white">
              <Link2 className="w-3 h-3" /> 取 PGM SRT
            </button>
          </div>
          <div className="flex items-center gap-3">
            <span className="text-[11px] text-vbs-muted">AUX 條數</span>
            <input
              type="number"
              min={0}
              max={4}
              value={pendingAUXCount}
              onChange={(e) => setPendingAUXCount(Number(e.target.value) || 0)}
              className="w-20 glass-dark border border-white/10 rounded px-2 py-1 text-white"
            />
          </div>
          {Array.from({ length: pendingAUXCount }, (_, i) => i + 1).map((ch) => (
            <div key={`aux-${ch}`} className="flex items-center gap-2">
              <span className="w-14 text-[11px] text-vbs-muted">{`AUX${ch}`}</span>
              <select
                value={pendingAuxSources[String(ch)] ?? `input${ch}`}
                onChange={(e) => {
                  const next = { ...pendingAuxSources, [String(ch)]: e.target.value }
                  setPendingAuxSources(next)
                }}
                className="flex-1 glass-dark border border-white/10 rounded px-2 py-1 text-white"
              >
                {sourceOptions.map((src) => <option key={src} value={src}>{src}</option>)}
              </select>
              <button onClick={() => generateSrtURL('aux', ch)} className="flex items-center gap-1 text-[11px] px-2 py-1 rounded border border-white/10 bg-white/5 text-white">
                <Link2 className="w-3 h-3" /> SRT
              </button>
            </div>
          ))}
          <div className="flex justify-end">
            <button
              onClick={() => saveRuntimeRouting(pendingPGMCount, pendingAUXCount, pendingAuxSources)}
              className="text-[11px] font-black uppercase tracking-widest px-3 py-2 rounded-lg border border-vbs-pvw/40 bg-vbs-pvw/10 text-vbs-pvw"
            >
              套用本區
            </button>
          </div>
        </div>

        <div className="md:col-span-6 glass rounded-[32px] p-8 shadow-xl flex flex-col gap-4">
          <h3 className="text-[14px] font-black text-white uppercase tracking-widest">導播按鈕 Mapping（1..8）</h3>
          <p className="text-[11px] text-vbs-muted">此 mapping 會同步給 Multiview 頁面的按鈕邏輯。</p>
          {Array.from({ length: 8 }, (_, i) => i + 1).map((btn) => (
            <div key={`btn-map-${btn}`} className="flex items-center gap-2">
              <span className="w-16 text-[11px] text-vbs-muted">{`按鈕 ${btn}`}</span>
              <select
                value={getSwitcherButtonSource(btn)}
                onChange={(e) => setSwitcherButtonSource(btn, e.target.value)}
                className="flex-1 glass-dark border border-white/10 rounded px-2 py-1 text-white"
              >
                {sourceOptions.map((src) => <option key={src} value={src}>{src}</option>)}
              </select>
            </div>
          ))}
          <div className="flex justify-end">
            <button
              onClick={() => applySectionDraft('導播按鈕 Mapping')}
              disabled={saving || applying}
              className="text-[11px] font-black uppercase tracking-widest px-3 py-2 rounded-lg border border-vbs-pvw/40 bg-vbs-pvw/10 text-vbs-pvw disabled:opacity-50"
            >
              套用本區
            </button>
          </div>
        </div>
        <div className="md:col-span-12 flex items-center justify-between">
          {runtime && <span className="text-[11px] text-vbs-muted">runtime inputs={runtime.inputs} capture={runtime.capture_inputs ?? 0} other={runtime.other_inputs ?? 0}</span>}
          <button onClick={rollback} disabled={applying} className="flex items-center gap-2 px-4 py-2 rounded-xl border border-vbs-pgm/40 bg-vbs-pgm/10 text-vbs-pgm disabled:opacity-50">
            <RotateCcw className="w-4 h-4" /> 回滾上一版
          </button>
        </div>
      </div>
    </PageShell>
  )
}

