import type {
  RuntimeConfig,
  ApplyResult,
  SwitchState,
  TelemetryLatest,
  ShowConfigPayload,
  ShowConfigState,
  ShowConfigHistoryRow,
} from '../types'
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

export interface GuestSessionRecord {
  id: string
  name: string
  pin: string
  expires_at: number
  access_token?: string
  token_type?: string
  magic_link?: string
}

interface NodeRecord {
  node_type?: string
  metrics?: Record<string, unknown>
}

interface PresenceRecord {
  node_type?: string
  online?: boolean
}

let adminRefreshInFlight: Promise<boolean> | null = null

function resolveApiBase(rawBase: string): string {
  const trimmed = rawBase.trim()
  if (!trimmed) return 'https://vbsapi.cyblisswisdom.org'
  if (trimmed === 'https://vbs.cyblisswisdom.org') return 'https://vbsapi.cyblisswisdom.org'
  return trimmed
}

export async function request<T>(
  method: 'GET' | 'POST' | 'PUT' | 'DELETE',
  path: string,
  body?: unknown
): Promise<ApiResponse<T>> {
  const settings = useSettingsStore.getState().settings
  const apiBase = resolveApiBase(settings.apiBaseUrl)
  const start = performance.now()

  try {
    const first = await performFetch(method, `${apiBase}${path}`, body, settings.apiTimeoutMs, useAuthStore.getState().user?.token ?? '')
    const latencyMs = Math.round(performance.now() - start)
    if (first.status === 401) {
      const refreshed = await tryRefreshAdminToken(apiBase, settings.apiTimeoutMs)
      if (refreshed) {
        const second = await performFetch(method, `${apiBase}${path}`, body, settings.apiTimeoutMs, useAuthStore.getState().user?.token ?? '')
        if (!second.ok) {
          return {
            error: second.data?.error ?? second.data?.message ?? `HTTP ${second.status}`,
            statusCode: second.status,
            latencyMs,
          }
        }
        return { data: second.data, statusCode: second.status, latencyMs }
      }
      return { error: first.data?.error ?? 'unauthorized', statusCode: 401, latencyMs }
    }
    if (!first.ok) {
      return { error: first.data?.error ?? first.data?.message ?? `HTTP ${first.status}`, statusCode: first.status, latencyMs }
    }
    return { data: first.data, statusCode: first.status, latencyMs }
  } catch (err) {
    const latencyMs = Math.round(performance.now() - start)
    if ((err as Error).name === 'AbortError') {
      return { error: 'Request timeout — 請檢查 API 連線', latencyMs }
    }
    return { error: (err as Error).message, latencyMs }
  }
}

async function performFetch(
  method: 'GET' | 'POST' | 'PUT' | 'DELETE',
  url: string,
  body: unknown,
  timeoutMs: number,
  token: string
): Promise<{ ok: boolean; status: number; data: any }> {
  const controller = new AbortController()
  const timeoutId = setTimeout(() => controller.abort(), timeoutMs)
  try {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    }
    if (token) headers.Authorization = `Bearer ${token}`
    const res = await fetch(url, {
      method,
      signal: controller.signal,
      credentials: 'include',
      headers,
      body: body ? JSON.stringify(body) : undefined,
    })
    const data = await res.json().catch(() => null)
    return { ok: res.ok, status: res.status, data }
  } finally {
    clearTimeout(timeoutId)
  }
}

async function tryRefreshAdminToken(apiBase: string, timeoutMs: number): Promise<boolean> {
  if (adminRefreshInFlight) return adminRefreshInFlight
  adminRefreshInFlight = (async () => {
    const user = useAuthStore.getState().user
    if (!user || user.role !== 'admin' || !user.email) {
      useAuthStore.getState().logout()
      return false
    }
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), timeoutMs)
    try {
      const res = await fetch(`${apiBase}/api/v1/auth/admin/email-login`, {
        method: 'POST',
        signal: controller.signal,
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: user.email }),
      })
      const data = await res.json().catch(() => null)
      const token = String(data?.access_token ?? '')
      if (!res.ok || !token) {
        useAuthStore.getState().logout()
        return false
      }
      useAuthStore.getState().login(token, 'admin', user.email)
      return true
    } catch {
      useAuthStore.getState().logout()
      return false
    } finally {
      clearTimeout(timeoutId)
      adminRefreshInFlight = null
    }
  })()
  return adminRefreshInFlight
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

function nodeApplyOK(block: unknown): boolean {
  if (block == null) return true
  if (typeof block !== 'object' || block === null) return Boolean(block)
  const o = block as Record<string, unknown>
  if (o.skipped === true) return true
  return Boolean(o.ok)
}

function parseApplyResult(payload: any): ApplyResult {
  const routeOK = nodeApplyOK(payload?.route)
  const engineOK = nodeApplyOK(payload?.engine)
  const rolledRaw = payload?.rolled_back
  let rolledBack = false
  if (rolledRaw && typeof rolledRaw === 'object' && !Array.isArray(rolledRaw)) {
    rolledBack = Object.values(rolledRaw as Record<string, boolean>).some(Boolean)
  }
  const t = Number(payload?.applied_at ?? 0) || Math.floor(Date.now() / 1000)
  const parts: string[] = []
  if (!routeOK) parts.push('Route 節點套用失敗')
  if (!engineOK) parts.push('Engine 節點套用失敗')
  if (rolledBack) parts.push('已嘗試回滾上一版設定')
  return {
    route: routeOK,
    engine: engineOK,
    rolled_back: rolledBack,
    message: parts.length ? parts.join('；') : '套用完成',
    timestamp: new Date(t * 1000).toISOString(),
    downstream: typeof payload === 'object' && payload !== null ? (payload as Record<string, unknown>) : undefined,
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

export async function createGuestSession(name: string, ttlSeconds: number): Promise<ApiResponse<GuestSessionRecord>> {
  const res = await request<any>('POST', '/api/v1/guest/sessions', {
    name,
    ttl_seconds: ttlSeconds,
  })
  if (res.error) return res as ApiResponse<GuestSessionRecord>
  const data = res.data ?? {}
  return {
    ...res,
    data: {
      id: String(data.id ?? ''),
      name: String(data.name ?? ''),
      pin: String(data.pin ?? ''),
      expires_at: Number(data.expires_at ?? 0),
      access_token: data.access_token ? String(data.access_token) : undefined,
      token_type: data.token_type ? String(data.token_type) : undefined,
      magic_link: data.magic_link ? String(data.magic_link) : undefined,
    },
  }
}

export async function deleteGuestSession(id: string): Promise<ApiResponse<{ deleted: boolean }>> {
  const cleanID = String(id).trim()
  if (!cleanID) return { error: 'session id required' }
  const res = await request<any>('DELETE', `/api/v1/guest/sessions/${encodeURIComponent(cleanID)}`)
  if (res.error) return res as ApiResponse<{ deleted: boolean }>
  const data = res.data ?? {}
  return {
    ...res,
    data: {
      deleted: Boolean(data.deleted),
    },
  }
}

export async function listGuestSessions(): Promise<ApiResponse<GuestSessionRecord[]>> {
  const res = await request<any>('GET', '/api/v1/guest/sessions')
  if (res.error) return res as ApiResponse<GuestSessionRecord[]>
  
  const data = Array.isArray(res.data?.sessions) ? res.data.sessions : []
  return {
    ...res,
    data: data.map((d: any) => ({
      id: String(d.id ?? ''),
      name: String(d.name ?? ''),
      pin: String(d.pin ?? ''),
      expires_at: Number(d.expires_at ?? 0),
      access_token: d.access_token ? String(d.access_token) : undefined,
      token_type: d.token_type ? String(d.token_type) : undefined,
      magic_link: d.magic_link ? String(d.magic_link) : undefined,
    }))
  }
}

export function defaultShowConfigDraft(): ShowConfigPayload {
  return {
    schema_version: '1.0',
    profile: {
      mode: 'pipeline_locked',
      target: { width: 1920, height: 1080, frame_rate: 60 },
    },
    sources: [],
    switcher: { panel_id: 'main_atem_like', rows: [] },
    multiview: { template_id: 'grid_4x4', cells: [] },
  }
}

export async function getShowConfig(): Promise<ApiResponse<ShowConfigState>> {
  const res = await request<any>('GET', '/api/v1/show-config')
  if (res.error || !res.data) return res as ApiResponse<ShowConfigState>
  const d = res.data
  return {
    ...res,
    data: {
      draft: (d.draft ?? null) as ShowConfigPayload | null,
      draft_updated_at: d.draft_updated_at != null ? Number(d.draft_updated_at) : null,
      effective: (d.effective ?? null) as ShowConfigPayload | null,
      effective_version: Number(d.effective_version ?? 0),
      effective_updated_at: d.effective_updated_at != null ? Number(d.effective_updated_at) : null,
    },
  }
}

export async function putShowConfigDraft(cfg: ShowConfigPayload): Promise<
  ApiResponse<{ saved: boolean; draft: ShowConfigPayload; draft_updated_at?: number }>
> {
  return request('PUT', '/api/v1/show-config/draft', cfg)
}

export interface ShowConfigApplyResponse {
  ok?: boolean
  effective?: ShowConfigPayload
  effective_version?: number
  effective_updated_at?: number
  downstream?: unknown
  message?: string
}

export async function postShowConfigApply(): Promise<ApiResponse<ShowConfigApplyResponse>> {
  return request('POST', '/api/v1/show-config/apply')
}

export async function postShowConfigRollback(): Promise<ApiResponse<ShowConfigApplyResponse>> {
  return request('POST', '/api/v1/show-config/rollback')
}

export async function getShowConfigHistory(limit = 50): Promise<ApiResponse<{ history: ShowConfigHistoryRow[] }>> {
  const res = await request<any>('GET', `/api/v1/show-config/history?limit=${encodeURIComponent(String(limit))}`)
  if (res.error) return res as ApiResponse<{ history: ShowConfigHistoryRow[] }>
  const rows = Array.isArray(res.data?.history) ? res.data.history : []
  const history: ShowConfigHistoryRow[] = rows.map((r: any) => ({
    version: Number(r.version ?? 0),
    applied_at: Number(r.applied_at ?? 0),
    downstream_result: r.downstream_result,
  }))
  return { ...res, data: { history } }
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
