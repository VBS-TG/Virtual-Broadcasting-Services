import type { RuntimeConfig, ApplyResult, SwitchState, TelemetryLatest } from '../types'
import { useSettingsStore } from '../stores/settingsStore'
import { useAuthStore } from '../stores/authStore'

export interface ApiResponse<T> {
  data?: T
  error?: string
  statusCode?: number
  latencyMs?: number
}

export interface GuestExchangeResult {
  access_token: string
  token_type?: string
  expires_at?: number
}

export interface AdminEmailLoginResult {
  access_token: string
  token_type?: string
  expires_at?: number
  role?: string
  email?: string
}

interface NodeRecord {
  node_type?: string
  metrics?: Record<string, unknown>
}

interface PresenceRecord {
  node_type?: string
  online?: boolean
}

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
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    }
    if (token) headers.Authorization = `Bearer ${token}`
    const res = await fetch(`${settings.apiBaseUrl}${path}`, {
      method,
      signal: controller.signal,
      headers,
      body: body ? JSON.stringify(body) : undefined,
    })
    clearTimeout(timeoutId)
    const latencyMs = Math.round(performance.now() - start)
    const data = await res.json().catch(() => null)
    if (!res.ok) {
      return { error: data?.error ?? data?.message ?? `HTTP ${res.status}`, statusCode: res.status, latencyMs }
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

function inputNumberToSource(input: number): string {
  return `input${input}`
}

function sourceToInputNumber(source: unknown): number {
  const raw = String(source ?? '').trim()
  if (!raw.startsWith('input')) return 1
  const n = Number(raw.slice('input'.length))
  return Number.isFinite(n) && n >= 1 && n <= 8 ? n : 1
}

function parseRuntimeConfig(payload: any): RuntimeConfig {
  const cfg = payload?.config ?? payload ?? {}
  const inputSources = Array.isArray(cfg.input_sources) ? cfg.input_sources.map((v: unknown) => String(v)) : []
  const auxSourcesRaw = cfg.aux_sources && typeof cfg.aux_sources === 'object' ? cfg.aux_sources : {}
  const auxSources: Record<string, string> = {}
  for (const [k, v] of Object.entries(auxSourcesRaw)) {
    auxSources[String(k)] = String(v ?? '')
  }
  return {
    inputs: Number(cfg.inputs ?? 8),
    pgm_count: Number(cfg.pgm_count ?? 1),
    aux_count: Number(cfg.aux_count ?? 0),
    input_sources: inputSources,
    aux_sources: auxSources,
  }
}

function parseApplyResult(payload: any): ApplyResult {
  const routeOK = Boolean(payload?.route?.ok)
  const engineOK = Boolean(payload?.engine?.ok)
  const rolled = Boolean(payload?.rolled_back && Object.keys(payload.rolled_back).length > 0)
  return {
    route: routeOK,
    engine: engineOK,
    rolled_back: rolled,
    message: routeOK && engineOK ? 'Applied' : 'Apply partially failed',
    timestamp: new Date((Number(payload?.applied_at ?? 0) || Math.floor(Date.now() / 1000)) * 1000).toISOString(),
  }
}

export async function getSwitchState(): Promise<ApiResponse<SwitchState>> {
  const res = await request<any>('GET', '/api/v1/switch/state')
  if (res.error) return res as ApiResponse<SwitchState>
  const d = res.data ?? {}
  return {
    ...res,
    data: {
      program: sourceToInputNumber(d.program),
      preview: sourceToInputNumber(d.preview),
      aux: {
        '1': sourceToInputNumber(d.aux?.['1']),
        '2': sourceToInputNumber(d.aux?.['2']),
        '3': sourceToInputNumber(d.aux?.['3']),
        '4': sourceToInputNumber(d.aux?.['4']),
      },
    },
  }
}

export async function switchProgram(input: number): Promise<ApiResponse<void>> {
  const res = await request<any>('POST', '/api/v1/switch/program', { source: inputNumberToSource(input) })
  return { ...res, data: undefined }
}

export async function switchPreview(input: number): Promise<ApiResponse<void>> {
  const res = await request<any>('POST', '/api/v1/switch/preview', { source: inputNumberToSource(input) })
  return { ...res, data: undefined }
}

export async function switchAux(channel: number, input: number): Promise<ApiResponse<void>> {
  const res = await request<any>('POST', '/api/v1/switch/aux', {
    channel: String(channel),
    source: inputNumberToSource(input),
  })
  return { ...res, data: undefined }
}

export async function getTelemetryLatest(): Promise<ApiResponse<TelemetryLatest>> {
  const res = await request<any>('GET', '/api/v1/telemetry/latest')
  if (res.error) return res as ApiResponse<TelemetryLatest>
  const latest = res.data?.latest ?? {}
  const presence = res.data?.presence ?? {}

  const pickByType = (nodeType: string): [NodeRecord, PresenceRecord] => {
    for (const [nodeID, rec] of Object.entries(latest as Record<string, NodeRecord>)) {
      const p = (presence as Record<string, PresenceRecord>)[nodeID]
      if (String(rec?.node_type ?? '').toLowerCase() === nodeType) return [rec ?? {}, p ?? {}]
    }
    return [{}, {}]
  }

  const toNodeTelemetry = (nodeType: string) => {
    const [rec, p] = pickByType(nodeType)
    const m = (rec.metrics ?? {}) as Record<string, unknown>
    const extraEntries: Array<[string, string | number]> = []
    for (const [k, v] of Object.entries(m)) {
      if (['cpu_percent', 'cpu_pct', 'mem_pct', 'throughput_mbps', 'total_ingest_mbps', 'fps', 'temp_c'].includes(k)) continue
      if (typeof v === 'number' || typeof v === 'string') extraEntries.push([k, v])
    }
    return {
      online: Boolean(p.online),
      cpu_pct: Number(m.cpu_percent ?? m.cpu_pct ?? 0),
      mem_pct: Number(m.mem_pct ?? 0),
      throughput_mbps: Number(m.throughput_mbps ?? m.total_ingest_mbps ?? 0),
      fps: m.fps !== undefined ? Number(m.fps) : undefined,
      temp_c: m.temp_c !== undefined ? Number(m.temp_c) : undefined,
      extra: Object.fromEntries(extraEntries),
    }
  }

  return {
    ...res,
    data: {
      timestamp: new Date().toISOString(),
      capture: toNodeTelemetry('capture'),
      route: toNodeTelemetry('route'),
      engine: toNodeTelemetry('engine'),
    },
  }
}

export async function getRuntimeConfig(): Promise<ApiResponse<RuntimeConfig>> {
  const res = await request<any>('GET', '/api/v1/runtime/config')
  if (res.error) return res as ApiResponse<RuntimeConfig>
  return { ...res, data: parseRuntimeConfig(res.data) }
}

export async function putRuntimeConfig(config: RuntimeConfig): Promise<ApiResponse<RuntimeConfig>> {
  const res = await request<any>('PUT', '/api/v1/runtime/config', config)
  if (res.error) return res as ApiResponse<RuntimeConfig>
  return { ...res, data: parseRuntimeConfig(res.data) }
}

export async function postApplyConfig(): Promise<ApiResponse<ApplyResult>> {
  const res = await request<any>('POST', '/api/v1/runtime/config/apply')
  if (res.error) return res as ApiResponse<ApplyResult>
  return { ...res, data: parseApplyResult(res.data) }
}

export async function exchangeGuestPIN(pin: string): Promise<ApiResponse<GuestExchangeResult>> {
  const res = await request<any>('POST', '/api/v1/guest/exchange-pin', { pin })
  if (res.error) return res as ApiResponse<GuestExchangeResult>
  const data = res.data ?? {}
  return {
    ...res,
    data: {
      access_token: String(data.access_token ?? ''),
      token_type: data.token_type ? String(data.token_type) : undefined,
      expires_at: data.expires_at !== undefined ? Number(data.expires_at) : undefined,
    },
  }
}

export async function adminEmailLogin(email: string): Promise<ApiResponse<AdminEmailLoginResult>> {
  const res = await request<any>('POST', '/api/v1/auth/admin/email-login', { email })
  if (res.error) return res as ApiResponse<AdminEmailLoginResult>
  const data = res.data ?? {}
  return {
    ...res,
    data: {
      access_token: String(data.access_token ?? ''),
      token_type: data.token_type ? String(data.token_type) : undefined,
      expires_at: data.expires_at !== undefined ? Number(data.expires_at) : undefined,
      role: data.role ? String(data.role) : undefined,
      email: data.email ? String(data.email) : undefined,
    },
  }
}

export async function checkHealth(url: string): Promise<{
  statusCode: number | null
  latencyMs: number | null
  ok: boolean
  error?: string
}> {
  const start = performance.now()
  try {
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), 8000)
    const res = await fetch(url, { method: 'GET', signal: controller.signal })
    clearTimeout(timeoutId)
    const latencyMs = Math.round(performance.now() - start)
    return { statusCode: res.status || null, latencyMs, ok: res.ok }
  } catch (err) {
    const latencyMs = Math.round(performance.now() - start)
    if ((err as Error).name === 'AbortError') {
      return { statusCode: null, latencyMs, ok: false, error: 'timeout' }
    }
    return { statusCode: null, latencyMs, ok: false, error: (err as Error).message }
  }
}
