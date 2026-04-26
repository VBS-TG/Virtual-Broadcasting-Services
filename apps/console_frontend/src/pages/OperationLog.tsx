import { useState, useMemo } from 'react'
import { useOperationLogStore } from '../stores/operationLogStore'

export default function OperationLog() {
  const { logs, clear, exportJson } = useOperationLogStore()
  const [keyword, setKeyword] = useState('')

  const filtered = useMemo(() => {
    if (!keyword.trim()) return logs
    const kw = keyword.toLowerCase()
    return logs.filter(
      (l) =>
        l.operation.toLowerCase().includes(kw) ||
        l.payload.toLowerCase().includes(kw) ||
        (l.details ?? '').toLowerCase().includes(kw)
    )
  }, [logs, keyword])

  return (
    <div className="h-full overflow-y-auto p-3 md:p-4 flex flex-col gap-3">
      {/* 頁頭 */}
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="text-[15px] font-black text-vbs-muted uppercase tracking-widest">
          操作日誌 ({filtered.length} / {logs.length})
        </h2>
        <div className="flex gap-2">
          <button
            id="log-export"
            onClick={exportJson}
            disabled={logs.length === 0}
            className="glass border border-vbs-accent/40 text-vbs-accent text-[15px] font-bold px-3 py-1.5 rounded-lg
              hover:bg-vbs-accent/15 transition-all active:scale-95 disabled:opacity-40"
          >
             匯出 JSON
          </button>
          <button
            id="log-clear"
            onClick={clear}
            disabled={logs.length === 0}
            className="glass border border-vbs-pgm/30 text-vbs-pgm text-[15px] font-bold px-3 py-1.5 rounded-lg
              hover:bg-vbs-pgm/10 transition-all active:scale-95 disabled:opacity-40"
          >
             清除
          </button>
        </div>
      </div>

      {/* 搜尋 */}
      <input
        id="log-search"
        type="text"
        placeholder="關鍵字搜尋（操作、payload、結果）"
        value={keyword}
        onChange={(e) => setKeyword(e.target.value)}
        className="glass-dark border border-white/10 rounded-xl px-4 py-2.5 text-[15px] text-vbs-text
          bg-transparent outline-none focus:border-vbs-accent/50 transition-colors font-mono"
      />

      {/* 日誌清單 */}
      {filtered.length === 0 ? (
        <div className="flex-1 flex items-center justify-center">
          <p className="text-[15px] text-vbs-muted">
            {logs.length === 0 ? '尚無操作記錄' : '沒有符合的結果'}
          </p>
        </div>
      ) : (
        <div className="flex flex-col gap-2">
          {filtered.map((log) => (
            <div key={log.id} className="glass rounded-xl p-3 flex flex-col gap-1.5">
              <div className="flex items-center justify-between gap-2 flex-wrap">
                <div className="flex items-center gap-2">
                  <span className={`text-[15px] font-black px-1.5 py-0.5 rounded-md border
                    ${log.result === 'success'
                      ? 'bg-vbs-pvw/10 border-vbs-pvw/30 text-vbs-pvw'
                      : log.result === 'error'
                      ? 'bg-vbs-pgm/10 border-vbs-pgm/30 text-vbs-pgm'
                      : 'bg-vbs-warning/10 border-vbs-warning/30 text-vbs-warning'}`}>
                    {log.result.toUpperCase()}
                  </span>
                  <span className="text-[15px] font-bold text-vbs-text font-mono">{log.operation}</span>
                </div>
                <span className="text-[12px] text-vbs-muted tabular-nums">
                  {new Date(log.time).toLocaleString('zh-TW', { hour12: false })}
                </span>
              </div>

              {log.payload && log.payload !== '{}' && (
                <p className="text-[12px] font-mono text-vbs-muted truncate">{log.payload}</p>
              )}

              {log.details && (
                <p className="text-[12px] font-mono text-vbs-dim truncate">{log.details}</p>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
