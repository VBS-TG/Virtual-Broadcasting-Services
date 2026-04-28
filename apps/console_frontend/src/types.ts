// ── 頁面路由 key ──────────────────────────────────────────────────────────────
export type PageKey =
  | 'dashboard'
  | 'runtime'
  | 'switcher'
  | 'multiviewer'
  | 'telemetry'
  | 'system'
  | 'logs'
  | 'settings'

// ── 角色 ──────────────────────────────────────────────────────────────────────
export type UserRole = 'admin' | 'guest'

// ── 登入使用者 ────────────────────────────────────────────────────────────────
export interface AuthUser {
  token: string
  role: UserRole
  tokenPreview: string // e.g. "eyJhb...xyz1"
  expiresAt: number | null
  email?: string
}

// ── 節點狀態（Dashboard 用） ──────────────────────────────────────────────────
export interface NodeStatus {
  id: 'console' | 'route' | 'engine'
  label: string
  status: 'ONLINE' | 'OFFLINE' | 'UNKNOWN'
}

// ── Runtime Config ────────────────────────────────────────────────────────────
export interface RuntimeConfig {
  inputs: number           // 1~8
  pgm_count: number        // 固定 1
  aux_count: number        // 0~4
  input_sources: string[]  // srt://...
  aux_sources: Record<string, string> // { aux1: 'input1' | 'srt://...' }
}

// ── Apply 結果 ────────────────────────────────────────────────────────────────
export interface ApplyResult {
  route: boolean
  engine: boolean
  rolled_back: boolean
  message: string
  timestamp: string
  /** Runtime apply：下游細節（原樣保留供 UI 進階展示） */
  downstream?: Record<string, unknown>
}

// ── Show Config（對齊後端 JSON 形狀，snake_case）────────────────────────────────
export interface ShowConfigPayload {
  schema_version: string
  profile: {
    mode: string
    target?: {
      width: number
      height: number
      frame_rate: number
    }
    hop_overrides?: Record<string, Record<string, unknown>>
  }
  sources: Array<{
    slot_id: string
    display_name: string
    short_label?: string
    group_id?: string
  }>
  switcher: {
    panel_id: string
    rows: unknown[]
  }
  multiview: {
    template_id: string
    cells: unknown[]
  }
}

export interface ShowConfigState {
  draft: ShowConfigPayload | null
  draft_updated_at: number | null
  effective: ShowConfigPayload | null
  effective_version: number
  effective_updated_at: number | null
}

export interface ShowConfigHistoryRow {
  version: number
  applied_at: number
  downstream_result?: unknown
}

// ── Switcher 狀態 ─────────────────────────────────────────────────────────────
export interface SwitchState {
  program: number
  preview: number
  aux: Record<string, number>
}

// ── Telemetry ─────────────────────────────────────────────────────────────────
export interface TelemetryLatest {
  timestamp: string
  capture: NodeTelemetry | null
  route: NodeTelemetry | null
  engine: NodeTelemetry | null
}

export interface NodeTelemetry {
  online: boolean
  cpu_pct: number
  mem_pct: number
  throughput_mbps: number
  fps?: number
  temp_c?: number
  extra?: Record<string, string | number>
}

// ── Operation Log ─────────────────────────────────────────────────────────────
export interface OperationLogEntry {
  id: string
  time: string
  operation: string
  payload: string
  result: 'success' | 'error' | 'pending'
  details?: string
}

// ── App Settings ──────────────────────────────────────────────────────────────
export interface AppSettings {
  apiBaseUrl: string
  engineBaseUrl: string
  /** Route HTTP 控制面基底 URL（供客戶端健康檢查 GET …/healthz） */
  routeBaseUrl: string
  refreshInterval: number // ms
  theme: 'dark' | 'light'
  apiTimeoutMs: number
}

// ── Health Check ──────────────────────────────────────────────────────────────
export interface HealthCheckResult {
  label: string
  url: string
  statusCode: number | null
  ok: boolean
  latencyMs: number | null
  error?: string
  checkedAt: string | null
}
