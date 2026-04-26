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
export type UserRole = 'admin' | 'operator'

// ── 登入使用者 ────────────────────────────────────────────────────────────────
export interface AuthUser {
  token: string
  role: UserRole
  tokenPreview: string // e.g. "eyJhb...xyz1"
  expiresAt: number | null
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
