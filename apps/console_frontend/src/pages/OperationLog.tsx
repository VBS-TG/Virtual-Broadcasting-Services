import { useState, useMemo } from 'react'
import { useOperationLogStore } from '../stores/operationLogStore'
import PageShell from '../components/PageShell'
import { FileText, Trash2, Download, Search } from 'lucide-react'

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
    <PageShell
      title="日誌"
      description="Local Operation Activity"
      extra={
        <div className="flex items-center gap-3">
          <button
            onClick={exportJson}
            disabled={logs.length === 0}
            className="flex items-center gap-2 px-4 py-2 rounded-xl text-[12px] font-black uppercase tracking-widest bg-vbs-accent/10 border border-vbs-accent/30 text-vbs-accent hover:bg-vbs-accent/20 transition-all disabled:opacity-30"
          >
            <Download className="w-4 h-4" />
            <span>匯出</span>
          </button>
          <button
            onClick={clear}
            disabled={logs.length === 0}
            className="flex items-center gap-2 px-4 py-2 rounded-xl text-[12px] font-black uppercase tracking-widest bg-vbs-pgm/10 border border-vbs-pgm/30 text-vbs-pgm hover:bg-vbs-pgm/20 transition-all disabled:opacity-30"
          >
            <Trash2 className="w-4 h-4" />
            <span>清除</span>
          </button>
        </div>
      }
    >
      <div className="flex flex-col gap-6">
        {/* Search Bar */}
        <div className="relative group">
          <div className="absolute inset-y-0 left-4 flex items-center pointer-events-none text-vbs-muted group-focus-within:text-vbs-accent transition-colors">
            <Search className="w-5 h-5" />
          </div>
          <input
            type="text"
            placeholder="搜尋關鍵字 (操作名稱、內容、結果)..."
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            className="w-full glass-dark border border-white/5 rounded-[24px] pl-12 pr-6 py-4 text-[15px] font-bold text-white placeholder:text-vbs-muted/50 outline-none focus:border-vbs-accent/30 focus:shadow-[0_0_30px_rgba(59,130,246,0.1)] transition-all"
          />
        </div>

        {/* Logs Container */}
        <div className="flex flex-col gap-3">
          {filtered.length === 0 ? (
            <div className="glass rounded-[32px] p-20 flex flex-col items-center justify-center text-center">
              <FileText className="w-16 h-16 text-vbs-muted mb-6 opacity-20" />
              <p className="text-[15px] font-black text-vbs-muted uppercase tracking-widest">
                {logs.length === 0 ? '目前尚無操作紀錄' : '未找到符合關鍵字的結果'}
              </p>
            </div>
          ) : (
            filtered.map((log) => (
              <div key={log.id} className="glass rounded-[24px] p-5 flex flex-col gap-3 border border-white/5 hover:border-white/10 transition-all group shadow-lg">
                <div className="flex items-center justify-between gap-4">
                  <div className="flex items-center gap-3">
                    <span className={`text-[10px] font-black px-2.5 py-1 rounded-lg uppercase tracking-widest shadow-sm
                      ${log.result === 'success' 
                        ? 'bg-vbs-pvw/10 text-vbs-pvw border border-vbs-pvw/20' 
                        : 'bg-vbs-pgm/10 text-vbs-pgm border border-vbs-pgm/20'}`}>
                      {log.result}
                    </span>
                    <span className="text-[16px] font-black text-white uppercase tracking-tight group-hover:text-vbs-accent transition-colors">
                      {log.operation}
                    </span>
                  </div>
                  <span className="text-[12px] font-bold text-vbs-muted tabular-nums opacity-60">
                    {new Date(log.time).toLocaleString('zh-TW', { hour12: false })}
                  </span>
                </div>

                {(log.payload && log.payload !== '{}' || log.details) && (
                  <div className="glass-dark bg-black/20 rounded-xl p-4 border border-white/5">
                    {log.payload && log.payload !== '{}' && (
                      <p className="text-[12px] font-mono text-white/40 truncate mb-1">Payload: {log.payload}</p>
                    )}
                    {log.details && (
                      <p className="text-[13px] font-bold text-vbs-muted break-all">{log.details}</p>
                    )}
                  </div>
                )}
              </div>
            ))
          )}
        </div>
      </div>
    </PageShell>
  )
}

