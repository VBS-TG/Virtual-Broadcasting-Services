import { useEffect } from 'react'
import { Network } from 'lucide-react'
import { useShowConfigStore } from '../stores/showConfigStore'
import { canAccess } from '../lib/permissions'
import { useToastStore } from '../stores/toastStore'

export default function PipelinePage() {
  const isAdmin = canAccess('admin')
  const { draft, loading, saving, error, saveDraft, setLocalDraft } = useShowConfigStore()

  useEffect(() => {
    void useShowConfigStore.getState().fetch()
  }, [])

  const showToast = (msg: string, type: 'success' | 'error' = 'success') => {
    useToastStore.getState().addToast({ title: msg, type })
  }

  const handleSave = async () => {
    if (!draft || !isAdmin) return
    const ok = await saveDraft(draft)
    const err = useShowConfigStore.getState().error
    showToast(ok ? '草稿已儲存' : (err ?? '儲存失敗'), ok ? 'success' : 'error')
  }

  if (loading && !draft) {
    return (
      <div className="h-full flex items-center justify-center p-6 text-vbs-muted text-[15px] font-semibold">
        載入製作設定…
      </div>
    )
  }

  if (!draft) {
    return (
      <div className="h-full flex items-center justify-center p-6 text-vbs-pgm text-[15px] font-semibold">
        {error ?? '無法載入草稿'}
      </div>
    )
  }

  const t = draft.profile.target ?? { width: 1920, height: 1080, frame_rate: 60 }

  return (
    <div className="h-full overflow-y-auto p-3 md:p-4 flex flex-col gap-4 max-w-3xl">
      <div className="flex items-center gap-2">
        <Network className="w-5 h-5 text-vbs-accent shrink-0" />
        <h2 className="text-[15px] font-black text-vbs-muted uppercase tracking-widest">Pipeline（編輯草稿）</h2>
      </div>

      {error && (
        <div className="glass border border-vbs-pgm/40 rounded-xl px-4 py-3 text-[14px] text-vbs-pgm font-medium">{error}</div>
      )}

      {!isAdmin && (
        <p className="text-[14px] text-vbs-warning font-semibold">
          目前為唯讀：僅管理員可儲存草稿。完整映射編輯介面將依 frontend_Revise.md 擴充。
        </p>
      )}

      <div className="glass rounded-xl p-4 flex flex-col gap-4">
        <span className="text-[12px] font-black text-vbs-muted uppercase tracking-widest">端到端目標格式（Capture → Route → Engine）</span>
        <div className="grid grid-cols-3 gap-3">
          <label className="flex flex-col gap-1">
            <span className="text-[11px] font-bold text-vbs-muted uppercase">寬度</span>
            <input
              type="number"
              disabled={!isAdmin}
              value={t.width}
              onChange={(e) =>
                setLocalDraft({
                  ...draft,
                  profile: {
                    ...draft.profile,
                    target: { ...t, width: Number(e.target.value) || 0 },
                  },
                })
              }
              className="glass-dark border border-white/10 rounded-xl px-3 py-2 text-vbs-text font-mono disabled:opacity-60"
            />
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-[11px] font-bold text-vbs-muted uppercase">高度</span>
            <input
              type="number"
              disabled={!isAdmin}
              value={t.height}
              onChange={(e) =>
                setLocalDraft({
                  ...draft,
                  profile: {
                    ...draft.profile,
                    target: { ...t, height: Number(e.target.value) || 0 },
                  },
                })
              }
              className="glass-dark border border-white/10 rounded-xl px-3 py-2 text-vbs-text font-mono disabled:opacity-60"
            />
          </label>
          <label className="flex flex-col gap-1">
            <span className="text-[11px] font-bold text-vbs-muted uppercase">幀率 fps</span>
            <input
              type="number"
              step={0.01}
              disabled={!isAdmin}
              value={t.frame_rate}
              onChange={(e) =>
                setLocalDraft({
                  ...draft,
                  profile: {
                    ...draft.profile,
                    target: { ...t, frame_rate: Number(e.target.value) || 0 },
                  },
                })
              }
              className="glass-dark border border-white/10 rounded-xl px-3 py-2 text-vbs-text font-mono disabled:opacity-60"
            />
          </label>
        </div>
        <p className="text-[13px] text-vbs-muted leading-relaxed">
          profile.mode：<span className="font-mono text-vbs-text">{draft.profile.mode}</span>
          。儲存後請至「Show Control」執行套用，始會下發至各節點。
        </p>
        <button
          type="button"
          disabled={!isAdmin || saving}
          onClick={() => void handleSave()}
          className="self-start glass border border-vbs-accent/50 text-vbs-accent font-bold px-5 py-2.5 rounded-xl
            hover:bg-vbs-accent/15 disabled:opacity-40 disabled:cursor-not-allowed"
        >
          {saving ? '儲存中…' : '儲存草稿'}
        </button>
      </div>
    </div>
  )
}
