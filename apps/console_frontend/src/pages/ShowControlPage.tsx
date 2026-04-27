import { useEffect } from 'react'
import { Clapperboard } from 'lucide-react'
import { useShowConfigStore } from '../stores/showConfigStore'
import { canAccess } from '../lib/permissions'
import { useToastStore } from '../stores/toastStore'

export default function ShowControlPage() {
  const isAdmin = canAccess('admin')
  const {
    applying,
    error,
    lastApplyMessage,
    effective_version,
    effective_updated_at,
    history,
    fetchHistory,
    apply,
    rollback,
  } = useShowConfigStore()

  useEffect(() => {
    void useShowConfigStore.getState().fetch()
    void useShowConfigStore.getState().fetchHistory()
  }, [])

  const showToast = (msg: string, type: 'success' | 'error' = 'success') => {
    useToastStore.getState().addToast({ title: msg, type })
  }

  const handleApply = async () => {
    const ok = await apply()
    const st = useShowConfigStore.getState()
    showToast(ok ? '演出設定已套用' : (st.lastApplyMessage ?? st.error ?? '套用失敗'), ok ? 'success' : 'error')
  }

  const handleRollback = async () => {
    const ok = await rollback()
    const st = useShowConfigStore.getState()
    showToast(ok ? '已回滾至上一版' : (st.lastApplyMessage ?? st.error ?? '回滾失敗'), ok ? 'success' : 'error')
  }

  const handleRefreshHistory = async () => {
    await fetchHistory()
    await useShowConfigStore.getState().fetch()
  }

  return (
    <div className="h-full overflow-y-auto p-3 md:p-4 flex flex-col gap-4 max-w-4xl">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Clapperboard className="w-5 h-5 text-vbs-accent shrink-0" />
          <h2 className="text-[15px] font-black text-vbs-muted uppercase tracking-widest">Show Control（版本與套用）</h2>
        </div>
        <button
          type="button"
          onClick={() => void handleRefreshHistory()}
          className="text-[13px] font-bold text-vbs-accent hover:text-white transition-colors"
        >
          重新整理
        </button>
      </div>

      {error && (
        <div className="glass border border-vbs-pgm/40 rounded-xl px-4 py-3 text-[14px] text-vbs-pgm font-medium">{error}</div>
      )}

      <div className="glass rounded-xl p-4 flex flex-col gap-2">
        <span className="text-[12px] font-black text-vbs-muted uppercase tracking-widest">目前生效版本</span>
        <p className="text-[17px] font-bold text-vbs-text">
          v{effective_version}{' '}
          {effective_updated_at ? (
            <span className="text-vbs-muted font-mono text-[14px]">
              （{new Date(effective_updated_at * 1000).toLocaleString('zh-TW')}）
            </span>
          ) : null}
        </p>
        {lastApplyMessage && (
          <p className="text-[14px] text-vbs-muted mt-1">{lastApplyMessage}</p>
        )}
      </div>

      <div className="flex flex-wrap gap-3">
        <button
          type="button"
          disabled={!isAdmin || applying}
          onClick={() => void handleApply()}
          className="glass border border-vbs-pvw/40 text-vbs-pvw font-black px-6 py-3 rounded-xl hover:bg-vbs-pvw/10 disabled:opacity-40"
        >
          {applying ? '處理中…' : '套用至節點'}
        </button>
        <button
          type="button"
          disabled={!isAdmin || applying || effective_version <= 0}
          onClick={() => void handleRollback()}
          className="glass border border-vbs-pgm/40 text-vbs-pgm font-black px-6 py-3 rounded-xl hover:bg-vbs-pgm/10 disabled:opacity-40"
        >
          回滾上一版
        </button>
      </div>

      {!isAdmin && (
        <p className="text-[13px] text-vbs-warning font-medium">操作員僅可檢視；套用與回滾需管理員。</p>
      )}

      <div className="glass rounded-xl p-4 flex flex-col gap-3">
        <span className="text-[12px] font-black text-vbs-muted uppercase tracking-widest">套用紀錄</span>
        {history.length === 0 ? (
          <p className="text-[14px] text-vbs-muted">尚無紀錄</p>
        ) : (
          <ul className="flex flex-col gap-2">
            {history.map((h) => (
              <li
                key={h.version}
                className="glass-dark border border-white/5 rounded-lg px-3 py-2 flex justify-between items-start gap-3"
              >
                <span className="font-mono font-bold text-vbs-text">v{h.version}</span>
                <span className="text-[13px] text-vbs-muted shrink-0">
                  {h.applied_at ? new Date(h.applied_at * 1000).toLocaleString('zh-TW') : '—'}
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}
