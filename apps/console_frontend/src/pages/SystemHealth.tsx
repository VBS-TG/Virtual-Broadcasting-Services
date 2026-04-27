import { useState } from 'react'
import { checkHealth } from '../lib/apiClient'
import { useSettingsStore } from '../stores/settingsStore'
import type { HealthCheckResult } from '../types'



const ERROR_HINTS: Record<string, string> = {
  timeout: ' Timeout — 請確認 tunnel/domain 是否正常、防火牆設定',
  '302':   ' 302 Redirect — 可能需要登入或 SSO 尚未導通',
  '401':   ' 401 Unauthorized — Bearer Token 無效或已過期',
  '502':   ' 502 Bad Gateway — 後端服務未啟動或 nginx 配置問題',
}

function hintFor(result: HealthCheckResult): string | null {
  if (result.error === 'timeout') return ERROR_HINTS.timeout
  const code = String(result.statusCode ?? '')
  return ERROR_HINTS[code] ?? null
}

export default function SystemHealth() {
  const { settings } = useSettingsStore()
  const [results, setResults] = useState<HealthCheckResult[]>([])
  const [checking, setChecking] = useState(false)

  const runCheck = async () => {
    setChecking(true)
    const api = settings.apiBaseUrl.replace(/\/$/, '')
    const eng = settings.engineBaseUrl.replace(/\/$/, '')
    const targets = [
      { label: 'Console API (/healthz)', url: `${api}/healthz` },
      { label: 'Engine Switcher', url: `${eng}/healthz` },
    ]
    const route = (settings.routeBaseUrl ?? '').trim().replace(/\/$/, '')
    if (route) {
      targets.push({ label: 'Route', url: `${route}/healthz` })
    }
    const checks = await Promise.all(
      targets.map(async (t) => {
        const r = await checkHealth(t.url)
        return {
          label: t.label,
          url: t.url,
          statusCode: r.statusCode,
          ok: r.ok,
          latencyMs: r.latencyMs,
          error: r.error,
          checkedAt: new Date().toISOString(),
        } as HealthCheckResult
      })
    )
    setResults(checks)
    setChecking(false)
  }

  return (
    <div className="h-full overflow-y-auto p-3 md:p-4 flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h2 className="text-[15px] font-black text-vbs-muted uppercase tracking-widest">系統健康 & 連線檢查</h2>
        <button
          id="run-health-check"
          onClick={runCheck}
          disabled={checking}
          className="glass border border-vbs-accent/40 text-vbs-accent text-[15px] font-bold px-4 py-2 rounded-xl
            hover:bg-vbs-accent/15 hover:border-vbs-accent transition-all active:scale-95 disabled:opacity-50"
        >
          {checking ? '檢查中…' : ' 執行健康檢查'}
        </button>
      </div>

      {/* 說明 */}
      <div className="glass rounded-xl p-4 flex flex-col gap-1.5">
        <p className="text-[15px] font-semibold text-vbs-muted">目標端點</p>
        <p className="text-[15px] font-mono text-vbs-text break-all">{settings.apiBaseUrl.replace(/\/$/, '')}/healthz</p>
        <p className="text-[15px] font-mono text-vbs-text break-all">{settings.engineBaseUrl.replace(/\/$/, '')}/healthz</p>
        {(settings.routeBaseUrl ?? '').trim() ? (
          <p className="text-[15px] font-mono text-vbs-text break-all">
            {(settings.routeBaseUrl ?? '').trim().replace(/\/$/, '')}/healthz
          </p>
        ) : (
          <p className="text-[15px] text-vbs-muted">Route：未設定（至 Settings 填寫 Route Base URL）</p>
        )}
        <p className="text-[15px] text-vbs-muted mt-1">
          點擊「執行健康檢查」後，系統將對以上端點發送 GET 請求並記錄 HTTP 狀態碼與延遲。
        </p>
      </div>

      {/* 結果卡片 */}
      {results.length > 0 && (
        <div className="flex flex-col gap-3">
          {results.map((r, i) => {
            const hint = hintFor(r)
            return (
              <div
                key={i}
                className={`glass rounded-xl p-4 flex flex-col gap-2 border
                  ${r.ok ? 'border-vbs-pvw/20' : 'border-vbs-pgm/30'}`}
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className={`w-2 h-2 rounded-full ${r.ok ? 'bg-vbs-pvw' : 'bg-vbs-pgm'} shrink-0`} />
                    <span className="text-[17px] font-bold text-vbs-text">{r.label}</span>
                  </div>
                  <span className={`text-[15px] font-black px-2 py-1 rounded-lg
                    ${r.ok ? 'bg-vbs-pvw/20 text-vbs-pvw' : 'bg-vbs-pgm/20 text-vbs-pgm'}`}>
                    {r.ok ? 'OK' : 'FAIL'}
                  </span>
                </div>

                <div className="grid grid-cols-3 gap-3">
                  <div>
                    <p className="text-[15px] text-vbs-muted">HTTP Code</p>
                    <p className="text-[17px] font-bold text-vbs-text">{r.statusCode ?? '—'}</p>
                  </div>
                  <div>
                    <p className="text-[15px] text-vbs-muted">延遲</p>
                    <p className="text-[17px] font-bold text-vbs-text">{r.latencyMs != null ? `${r.latencyMs} ms` : '—'}</p>
                  </div>
                  <div>
                    <p className="text-[15px] text-vbs-muted">時間</p>
                    <p className="text-[17px] font-bold text-vbs-text">
                      {r.checkedAt ? new Date(r.checkedAt).toLocaleTimeString('zh-TW', { hour12: false }) : '—'}
                    </p>
                  </div>
                </div>

                {r.error && (
                  <p className="text-[15px] font-mono text-vbs-pgm">{r.error}</p>
                )}

                {/* 提示 */}
                {hint && (
                  <div className="mt-1 p-2.5 rounded-lg bg-vbs-warning/10 border border-vbs-warning/20">
                    <p className="text-[15px] text-vbs-warning font-medium">{hint}</p>
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}

      {/* 常見問題說明 */}
      <div className="glass rounded-xl p-4 flex flex-col gap-3">
        <p className="text-[15px] font-black text-vbs-muted uppercase tracking-widest">常見錯誤識別</p>
        {Object.entries(ERROR_HINTS).map(([code, hint]) => (
          <div key={code} className="flex gap-3 items-start">
            <span className="text-[15px] font-black text-vbs-accent w-12 shrink-0">{code.toUpperCase()}</span>
            <p className="text-[15px] text-vbs-muted">{hint}</p>
          </div>
        ))}
      </div>
    </div>
  )
}
