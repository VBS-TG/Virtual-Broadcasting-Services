import type { UserRole } from '../types'

export interface JWTPayload {
  sub?: string
  iss?: string
  aud?: string | string[]
  role?: UserRole
  exp?: number
  nbf?: number
  iat?: number
  email?: string
  [key: string]: unknown
}

export function parseJwt(token: string): JWTPayload | null {
  try {
    const parts = token.split('.')
    if (parts.length !== 3) return null
    const base64Url = parts[1]
    if (!base64Url) return null
    const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/')
    const jsonPayload = decodeURIComponent(atob(base64).split('').map(function(c) {
      return '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2)
    }).join(''))
    return JSON.parse(jsonPayload) as JWTPayload
  } catch (e) {
    return null
  }
}

export interface TokenValidationResult {
  ok: boolean
  token?: string
  payload?: JWTPayload
  role?: UserRole
  error?: string
}

export function stripBearerPrefix(input: string): string {
  const raw = input.trim()
  if (raw.toLowerCase().startsWith('bearer ')) {
    return raw.slice(7).trim()
  }
  return raw
}

export function validateAccessToken(rawInput: string): TokenValidationResult {
  const token = stripBearerPrefix(rawInput)
  if (!token) return { ok: false, error: '請輸入 Bearer Token' }
  const parts = token.split('.')
  if (parts.length !== 3) {
    return { ok: false, error: 'Token 格式錯誤：JWT 應為三段式（header.payload.signature）' }
  }
  const payload = parseJwt(token)
  if (!payload) return { ok: false, error: 'Token 解析失敗：payload 非合法 JSON' }

  const nowSec = Math.floor(Date.now() / 1000)
  if (typeof payload.exp !== 'number') {
    return { ok: false, error: 'Token 缺少 exp（到期時間）' }
  }
  if (payload.exp <= nowSec) {
    return { ok: false, error: 'Token 已過期，請重新取得授權' }
  }
  if (typeof payload.nbf === 'number' && payload.nbf > nowSec) {
    return { ok: false, error: 'Token 尚未生效（nbf）' }
  }
  if (typeof payload.iat === 'number' && payload.iat > nowSec+60) {
    return { ok: false, error: 'Token 簽發時間異常（iat > now）' }
  }

  const roleRaw = String(payload.role ?? '').toLowerCase()
  if (roleRaw !== 'admin' && roleRaw !== 'guest') {
    return { ok: false, error: 'Token 角色不符，僅接受 admin/guest' }
  }
  const role = roleRaw as UserRole
  return { ok: true, token, payload, role }
}
