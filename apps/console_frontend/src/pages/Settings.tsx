import { useState } from 'react'
import { useSettingsStore } from '../stores/settingsStore'
import { useToastStore } from '../stores/toastStore'
import { canAccess } from '../lib/permissions'

export default function Settings() {
  const { settings, update, reset } = useSettingsStore()
  const [saved, setSaved] = useState(false)

  const [local, setLocal] = useState({ ...settings })
  const isAdmin = canAccess('admin')

  const handleSave = () => {
    update(local)
    setSaved(true)
    useToastStore.getState().addToast({ title: '設定已儲存', type: 'success' })
    setTimeout(() => setSaved(false), 2000)
  }

  return (
    <div className="h-full overflow-y-auto p-3 md:p-4 flex flex-col gap-4">
      <h2 className="text-[15px] font-black text-vbs-muted uppercase tracking-widest">前端設定</h2>

      <div className="glass rounded-xl p-4 flex flex-col gap-4">
        {/* API Base URL */}
        <div className="flex flex-col gap-1.5">
          <label htmlFor="settings-api-url" className="text-[15px] font-semibold text-vbs-muted uppercase tracking-widest">
            Console API Base URL
          </label>
          <input
            id="settings-api-url"
            type="text"
            value={local.apiBaseUrl}
            disabled={!isAdmin}
            onChange={(e) => setLocal({ ...local, apiBaseUrl: e.target.value })}
            className="glass-dark border border-white/10 rounded-xl px-4 py-3 text-[17px] font-mono text-vbs-text
              bg-transparent outline-none focus:border-vbs-accent/50 transition-colors disabled:opacity-50"
          />
        </div>

        {/* Engine Base URL */}
        <div className="flex flex-col gap-1.5">
          <label htmlFor="settings-engine-url" className="text-[15px] font-semibold text-vbs-muted uppercase tracking-widest">
            Engine Switcher Base URL
          </label>
          <input
            id="settings-engine-url"
            type="text"
            value={local.engineBaseUrl}
            disabled={!isAdmin}
            onChange={(e) => setLocal({ ...local, engineBaseUrl: e.target.value })}
            className="glass-dark border border-white/10 rounded-xl px-4 py-3 text-[17px] font-mono text-vbs-text
              bg-transparent outline-none focus:border-vbs-accent/50 transition-colors disabled:opacity-50"
          />
        </div>

        {/* API Timeout */}
        <div className="flex flex-col gap-1.5">
          <label htmlFor="settings-timeout" className="text-[15px] font-semibold text-vbs-muted uppercase tracking-widest">
            API Timeout (ms)
          </label>
          <select
            id="settings-timeout"
            value={local.apiTimeoutMs}
            onChange={(e) => setLocal({ ...local, apiTimeoutMs: Number(e.target.value) })}
            className="glass-dark border border-white/10 rounded-xl px-4 py-3 text-[17px] text-vbs-text
              bg-transparent outline-none focus:border-vbs-accent/50 transition-colors"
          >
            <option value={5000}>5 秒</option>
            <option value={10000}>10 秒（預設）</option>
            <option value={15000}>15 秒</option>
            <option value={30000}>30 秒</option>
          </select>
        </div>

        {/* 自動刷新間隔 */}
        <div className="flex flex-col gap-1.5">
          <label htmlFor="settings-refresh" className="text-[15px] font-semibold text-vbs-muted uppercase tracking-widest">
            Telemetry 自動刷新間隔
          </label>
          <select
            id="settings-refresh"
            value={local.refreshInterval}
            onChange={(e) => setLocal({ ...local, refreshInterval: Number(e.target.value) })}
            className="glass-dark border border-white/10 rounded-xl px-4 py-3 text-[17px] text-vbs-text
              bg-transparent outline-none focus:border-vbs-accent/50 transition-colors"
          >
            <option value={500}>0.5 秒</option>
            <option value={1000}>1 秒（預設）</option>
            <option value={3000}>3 秒</option>
            <option value={5000}>5 秒</option>
          </select>
        </div>

        {/* 主題 */}
        <div className="flex flex-col gap-1.5">
          <label className="text-[15px] font-semibold text-vbs-muted uppercase tracking-widest">主題</label>
          <div className="flex gap-2">
            {(['dark', 'light'] as const).map((t) => (
              <button
                key={t}
                id={`theme-${t}`}
                onClick={() => setLocal({ ...local, theme: t })}
                className={`flex-1 py-2.5 rounded-xl text-[15px] font-bold border transition-all
                  ${local.theme === t
                    ? 'bg-vbs-accent/20 border-vbs-accent/50 text-vbs-accent'
                    : 'glass-dark border-white/10 text-vbs-muted hover:text-vbs-text'}`}
              >
                {t === 'dark' ? ' 暗色' : ' 亮色'}
              </button>
            ))}
          </div>
          <p className="text-[12px] text-vbs-muted">
             亮色主題尚未實作，目前固定使用暗色。
          </p>
        </div>
      </div>

      {/* 操作按鈕 */}
      <div className="flex gap-3">
        <button
          id="settings-save"
          onClick={handleSave}
          disabled={!isAdmin}
          title={!isAdmin ? "權限不足：僅管理員可修改" : ""}
          className={`flex-1 py-3 rounded-xl font-bold text-[17px] border transition-all active:scale-95
            ${!isAdmin ? 'glass border-white/5 text-vbs-muted opacity-50 cursor-not-allowed' : 'glass border-vbs-accent/40 text-vbs-accent hover:bg-vbs-accent/15 hover:border-vbs-accent'}`}
        >
          {saved ? ' 已儲存' : ' 儲存設定'}
        </button>
        <button
          id="settings-reset"
          onClick={() => { reset(); setLocal({ ...settings }) }}
          disabled={!isAdmin}
          title={!isAdmin ? "權限不足：僅管理員可修改" : ""}
          className={`px-6 py-3 rounded-xl font-bold text-[17px] border transition-all active:scale-95
            ${!isAdmin ? 'glass border-white/5 text-vbs-muted opacity-50 cursor-not-allowed' : 'glass border-vbs-pgm/30 text-vbs-pgm hover:bg-vbs-pgm/10 hover:border-vbs-pgm'}`}
        >
           重設預設值
        </button>
      </div>

      {/* 目前設定摘要 */}
      <div className="glass rounded-xl p-4">
        <p className="text-[15px] font-black text-vbs-muted uppercase tracking-widest mb-2">目前生效設定</p>
        <pre className="text-[15px] font-mono text-vbs-dim whitespace-pre-wrap">
          {JSON.stringify(settings, null, 2)}
        </pre>
      </div>
    </div>
  )
}
