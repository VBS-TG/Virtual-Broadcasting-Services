import { useEffect, useState } from 'react'
import { useRuntimeStore } from '../stores/runtimeStore'
import { useToastStore } from '../stores/toastStore'
import { canAccess } from '../lib/permissions'
import type { RuntimeConfig } from '../types'

// 前端驗證規則（frontend.md §4.3）
function validateConfig(cfg: RuntimeConfig): string[] {
  const errs: string[] = []
  if (!Number.isInteger(cfg.inputs) || cfg.inputs < 1 || cfg.inputs > 8)
    errs.push('inputs 必須是 1~8 的整數')
  if (cfg.pgm_count !== 1)
    errs.push('pgm_count 固定為 1')
  if (!Number.isInteger(cfg.aux_count) || cfg.aux_count < 0 || cfg.aux_count > 4)
    errs.push('aux_count 必須是 0~4 的整數')
  cfg.input_sources.forEach((src, i) => {
    if (!src.startsWith('srt://'))
      errs.push(`input_sources[${i}] 必須以 srt:// 開頭`)
  })
  Object.entries(cfg.aux_sources).forEach(([k, v]) => {
    if (!/^(input\d+|srt:\/\/)/.test(v))
      errs.push(`aux_sources.${k} 只能是 inputN 或 srt://…`)
  })
  return errs
}

export default function RuntimeConfig() {
  const { config, loading, saving, applying, error, lastApplyResult, fetch, save, apply } =
    useRuntimeStore()

  const [draft, setDraft] = useState<RuntimeConfig | null>(null)
  const [validationErrors, setValidationErrors] = useState<string[]>([])
  
  const isAdmin = canAccess('admin')

  useEffect(() => { fetch() }, [fetch])
  useEffect(() => { if (config && !draft) setDraft({ ...config }) }, [config, draft])

  const showToast = (msg: string, type: 'success' | 'error' = 'success') => {
    useToastStore.getState().addToast({ title: msg, type })
  }

  const handleSave = async () => {
    if (!draft) return
    const errs = validateConfig(draft)
    if (errs.length) { setValidationErrors(errs); return }
    setValidationErrors([])
    const ok = await save(draft)
    showToast(ok ? ' 儲存成功' : (error ?? '儲存失敗'), ok ? 'success' : 'error')
  }

  const handleApply = async () => {
    const ok = await apply()
    showToast(ok ? ' Apply 成功' : (error ?? 'Apply 失敗'), ok ? 'success' : 'error')
  }

  if (loading) return <PageLoader label="載入 Runtime Config…" />

  return (
    <div className="h-full overflow-y-auto p-3 md:p-4 flex flex-col gap-4">
      <h2 className="text-[15px] font-black text-vbs-muted uppercase tracking-widest">Runtime Config</h2>

      {/* Validation Errors */}
      {validationErrors.length > 0 && (
        <div className="glass border border-vbs-pgm/40 rounded-xl p-3 flex flex-col gap-1">
          {validationErrors.map((e, i) => (
            <p key={i} className="text-[15px] text-vbs-pgm font-medium"> {e}</p>
          ))}
        </div>
      )}

      {draft && (
        <>
          {/* ── 數量設定區 ── */}
          <div className="glass rounded-xl p-4 flex flex-col gap-4">
            <span className="text-[15px] font-black text-vbs-muted uppercase tracking-widest">數量設定</span>
            <div className="grid grid-cols-3 gap-4">
              <NumberField
                id="inputs-field"
                label="Inputs (1~8)"
                value={draft.inputs}
                min={1} max={8}
                onChange={(v) => setDraft({ ...draft, inputs: v })}
              />
              <NumberField
                id="pgm-count-field"
                label="PGM Count (固定)"
                value={draft.pgm_count}
                min={1} max={1}
                onChange={(v) => setDraft({ ...draft, pgm_count: v })}
                disabled
              />
              <NumberField
                id="aux-count-field"
                label="AUX Count (0~4)"
                value={draft.aux_count}
                min={0} max={4}
                onChange={(v) => setDraft({ ...draft, aux_count: v })}
              />
            </div>
          </div>

          {/* ── Input Sources ── */}
          <div className="glass rounded-xl p-4 flex flex-col gap-3">
            <div className="flex items-center justify-between">
              <span className="text-[15px] font-black text-vbs-muted uppercase tracking-widest">Input Sources</span>
              <button
                id="add-input-source"
                onClick={() => setDraft({
                  ...draft,
                  input_sources: [...draft.input_sources, 'srt://'],
                })}
                disabled={draft.input_sources.length >= 8}
                className="text-[15px] text-vbs-accent hover:underline disabled:opacity-40 font-semibold"
              >
                + 新增
              </button>
            </div>
            {draft.input_sources.map((src, i) => (
              <div key={i} className="flex gap-2 items-center">
                <span className="text-[15px] text-vbs-muted w-8 shrink-0">#{i + 1}</span>
                <input
                  id={`input-src-${i}`}
                  type="text"
                  value={src}
                  onChange={(e) => {
                    const arr = [...draft.input_sources]
                    arr[i] = e.target.value
                    setDraft({ ...draft, input_sources: arr })
                  }}
                  className="flex-1 glass-dark border border-white/10 rounded-lg px-3 py-2 text-[15px]
                    font-mono text-vbs-text bg-transparent outline-none focus:border-vbs-accent/50 transition-colors"
                  placeholder="srt://..."
                />
                <button
                  id={`remove-input-src-${i}`}
                  onClick={() => {
                    const arr = draft.input_sources.filter((_, j) => j !== i)
                    setDraft({ ...draft, input_sources: arr })
                  }}
                  className="text-vbs-pgm hover:text-vbs-pgm/70 text-[17px] font-bold px-1"
                ></button>
              </div>
            ))}
          </div>

          {/* ── AUX Sources ── */}
          {draft.aux_count > 0 && (
            <div className="glass rounded-xl p-4 flex flex-col gap-3">
              <span className="text-[15px] font-black text-vbs-muted uppercase tracking-widest">AUX Sources</span>
              {Array.from({ length: draft.aux_count }, (_, i) => {
                const key = String(i + 1)
                return (
                  <div key={key} className="flex gap-2 items-center">
                    <span className="text-[15px] text-vbs-muted w-10 shrink-0">AUX {i + 1}</span>
                    <input
                      id={`aux-src-${key}`}
                      type="text"
                      value={draft.aux_sources[key] ?? ''}
                      onChange={(e) =>
                        setDraft({ ...draft, aux_sources: { ...draft.aux_sources, [key]: e.target.value } })
                      }
                      className="flex-1 glass-dark border border-white/10 rounded-lg px-3 py-2 text-[15px]
                        font-mono text-vbs-text bg-transparent outline-none focus:border-vbs-accent/50 transition-colors"
                      placeholder="inputN 或 srt://..."
                    />
                  </div>
                )
              })}
            </div>
          )}

          {/* ── Actions ── */}
          <div className="flex gap-3">
            <button
              id="runtime-save"
              onClick={handleSave}
              disabled={saving || !isAdmin}
              title={!isAdmin ? "權限不足：僅管理員可儲存" : ""}
              className={`flex-1 py-3 rounded-xl font-bold text-[17px] border transition-all active:scale-95
                ${saving || !isAdmin ? 'glass border-white/5 text-vbs-muted opacity-50 cursor-not-allowed' : 'glass border-vbs-accent/40 text-vbs-accent hover:bg-vbs-accent/15 hover:border-vbs-accent'}
              `}
            >
              {saving ? '儲存中…' : ' Save'}
            </button>
            <button
              id="runtime-apply"
              onClick={handleApply}
              disabled={applying || !isAdmin}
              title={!isAdmin ? "權限不足：僅管理員可 Apply" : ""}
              className={`flex-1 py-3 rounded-xl font-bold text-[17px] border transition-all active:scale-95
                ${applying || !isAdmin ? 'glass border-white/5 text-vbs-muted opacity-50 cursor-not-allowed' : 'glass border-vbs-pvw/40 text-vbs-pvw hover:bg-vbs-pvw/15 hover:border-vbs-pvw'}
              `}
            >
              {applying ? 'Apply 中…' : ' Apply'}
            </button>
          </div>

          {/* ── Apply Result ── */}
          {lastApplyResult && (
            <div className="glass rounded-xl p-4 flex flex-col gap-2">
              <span className="text-[15px] font-black text-vbs-muted uppercase tracking-widest">最後一次 Apply 結果</span>
              <pre className="text-[15px] font-mono text-vbs-text whitespace-pre-wrap break-all leading-relaxed">
                {JSON.stringify(lastApplyResult, null, 2)}
              </pre>
            </div>
          )}
        </>
      )}
    </div>
  )
}

function PageLoader({ label }: { label: string }) {
  return (
    <div className="h-full flex items-center justify-center">
      <p className="text-[15px] text-vbs-muted animate-pulse">{label}</p>
    </div>
  )
}

function NumberField({
  id, label, value, min, max, onChange, disabled,
}: {
  id: string; label: string; value: number; min: number; max: number
  onChange: (v: number) => void; disabled?: boolean
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-[15px] text-vbs-muted font-semibold">{label}</label>
      <input
        id={id}
        type="number"
        min={min}
        max={max}
        value={value}
        disabled={disabled}
        onChange={(e) => onChange(Number(e.target.value))}
        className="glass-dark border border-white/10 rounded-lg px-3 py-2 text-[17px] font-bold text-vbs-text
          bg-transparent outline-none focus:border-vbs-accent/50 transition-colors disabled:opacity-50 text-center"
      />
    </div>
  )
}
