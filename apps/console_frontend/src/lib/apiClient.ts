// ─────────────────────────────────────────────────────────────────────────────
// lib/apiClient.ts
//
// [MOCK] 目前所有 API 呼叫均回傳假資料。
// 後端就緒後：
//   1. 將各函式內的 [MOCK] 區塊整個移除
//   2. 取消下方 request() 呼叫的註解
// ─────────────────────────────────────────────────────────────────────────────

import type { RuntimeConfig, ApplyResult, SwitchState, TelemetryLatest } from '../types'
import { useSettingsStore } from '../stores/settingsStore'
import { useAuthStore } from '../stores/authStore'

// [MOCK] 假資料延遲（模擬網路）
const MOCK_DELAY_MS = 450

async function mockDelay<T>(data: T): Promise<ApiResponse<T>> {
  await new Promise(r => setTimeout(r, MOCK_DELAY_MS))
  return { data, statusCode: 200, latencyMs: MOCK_DELAY_MS }
}

export interface ApiResponse<T> {
  data?: T
  error?: string
  statusCode?: number
  latencyMs?: number
}

// ── 底層 fetch 封裝（後端就緒後啟用） ────────────────────────────────────────
export async function request<T>(
  method: 'GET' | 'POST' | 'PUT',
  path: string,
  body?: unknown
): Promise<ApiResponse<T>> {
  const settings = useSettingsStore.getState().settings
  const token = useAuthStore.getState().user?.token ?? ''
  const start = performance.now()

  try {
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), settings.apiTimeoutMs)
    const res = await fetch(`${settings.apiBaseUrl}${path}`, {
      method,
      signal: controller.signal,
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: body ? JSON.stringify(body) : undefined,
    })
    clearTimeout(timeoutId)
    const latencyMs = Math.round(performance.now() - start)
    const data = await res.json().catch(() => null)
    if (!res.ok) {
      return { error: data?.message ?? `HTTP ${res.status}`, statusCode: res.status, latencyMs }
    }
    return { data, statusCode: res.status, latencyMs }
  } catch (err) {
    const latencyMs = Math.round(performance.now() - start)
    if ((err as Error).name === 'AbortError') {
      return { error: 'Request timeout — 請檢查 API 連線', latencyMs }
    }
    return { error: (err as Error).message, latencyMs }
  }
}

// ── Runtime Config ────────────────────────────────────────────────────────────

export async function getRuntimeConfig(): Promise<ApiResponse<RuntimeConfig>> {
  // [MOCK] 後端就緒後改為：return request('GET', '/api/v1/runtime/config')
  return mockDelay<RuntimeConfig>({
    inputs: 4,
    pgm_count: 1,
    aux_count: 2,
    input_sources: [
      'srt://capture.vbsapi.cyblisswisdom.org:9001',
      'srt://capture.vbsapi.cyblisswisdom.org:9002',
      'srt://capture.vbsapi.cyblisswisdom.org:9003',
      'srt://capture.vbsapi.cyblisswisdom.org:9004',
    ],
    aux_sources: { aux1: 'input1', aux2: 'input2' },
  })
}

export async function putRuntimeConfig(config: RuntimeConfig): Promise<ApiResponse<RuntimeConfig>> {
  // [MOCK] 後端就緒後改為：return request('PUT', '/api/v1/runtime/config', config)
  console.log('[MOCK] PUT /api/v1/runtime/config', config)
  return mockDelay<RuntimeConfig>(config)
}

export async function postApplyConfig(): Promise<ApiResponse<ApplyResult>> {
  // [MOCK] 後端就緒後改為：return request('POST', '/api/v1/runtime/config/apply')
  console.log('[MOCK] POST /api/v1/runtime/config/apply')
  return mockDelay<ApplyResult>({
    route: true,
    engine: true,
    rolled_back: false,
    message: '[MOCK] Config applied successfully',
    timestamp: new Date().toISOString(),
  })
}

// ── Switch ────────────────────────────────────────────────────────────────────

export async function getSwitchState(): Promise<ApiResponse<SwitchState>> {
  // [MOCK] 後端就緒後改為：return request('GET', '/api/v1/switch/state')
  return mockDelay<SwitchState>({ program: 1, preview: 2, aux: { '1': 1, '2': 2, '3': 3, '4': 4 } })
}

export async function switchProgram(input: number): Promise<ApiResponse<void>> {
  // [MOCK] 後端就緒後改為：return request('POST', '/api/v1/switch/program', { input })
  console.log('[MOCK] POST /api/v1/switch/program', { input })
  return mockDelay<void>(undefined)
}

export async function switchPreview(input: number): Promise<ApiResponse<void>> {
  // [MOCK] 後端就緒後改為：return request('POST', '/api/v1/switch/preview', { input })
  console.log('[MOCK] POST /api/v1/switch/preview', { input })
  return mockDelay<void>(undefined)
}

export async function switchAux(channel: number, input: number): Promise<ApiResponse<void>> {
  // [MOCK] 後端就緒後改為：return request('POST', '/api/v1/switch/aux', { channel, input })
  console.log('[MOCK] POST /api/v1/switch/aux', { channel, input })
  return mockDelay<void>(undefined)
}

// ── Telemetry ─────────────────────────────────────────────────────────────────

export async function getTelemetryLatest(): Promise<ApiResponse<TelemetryLatest>> {
  // [MOCK] 後端就緒後改為：return request('GET', '/api/v1/telemetry/latest')
  const r = (base: number, spread: number) =>
    Math.max(0, base + (Math.random() - 0.5) * spread)
  return mockDelay<TelemetryLatest>({
    timestamp: new Date().toISOString(),
    capture: {
      online: true,
      cpu_pct: r(45, 20),
      mem_pct: r(55, 15),
      throughput_mbps: r(12, 4),
      fps: r(60, 2),
      temp_c: r(58, 8),
      extra: {
        'NIC-0 上行': `${r(12, 4).toFixed(1)} Mbps`,
        'SRTLA 掉包': `${Math.round(r(2, 3))} pkts`,
      },
    },
    route: {
      online: true,
      cpu_pct: r(28, 10),
      mem_pct: r(45, 8),
      throughput_mbps: r(22, 5),
      extra: {
        '封包排序錯誤率': `${r(0.3, 0.3).toFixed(2)}%`,
        'Engine 拉流': 'CONNECTED',
      },
    },
    engine: {
      online: true,
      cpu_pct: r(40, 15),
      mem_pct: r(60, 10),
      throughput_mbps: r(15, 5),
      fps: r(60, 1),
      temp_c: r(62, 10),
      extra: {
        'GPU 負載': `${r(55, 15).toFixed(0)}%`,
        '顯存佔用': `${(r(4200, 500) / 1024).toFixed(1)} GB`,
        '推流狀態': 'SRT → ROUTE ',
      },
    },
  })
}

// ── Health Check（實際嘗試連線） ──────────────────────────────────────────────

export async function checkHealth(url: string): Promise<{
  statusCode: number | null
  latencyMs: number | null
  ok: boolean
  error?: string
}> {
  const start = performance.now()
  try {
    const controller = new AbortController()
    setTimeout(() => controller.abort(), 8000)
    // mode: 'no-cors' 避免 CORS 阻擋，但 status 會是 0
    const res = await fetch(url, { method: 'GET', signal: controller.signal, mode: 'no-cors' })
    const latencyMs = Math.round(performance.now() - start)
    return { statusCode: res.status || null, latencyMs, ok: true }
  } catch (err) {
    const latencyMs = Math.round(performance.now() - start)
    if ((err as Error).name === 'AbortError') {
      return { statusCode: null, latencyMs, ok: false, error: 'timeout' }
    }
    return { statusCode: null, latencyMs, ok: false, error: (err as Error).message }
  }
}
