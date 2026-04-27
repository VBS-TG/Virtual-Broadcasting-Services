import { create } from 'zustand'
import { createGuestSession, deleteGuestSession, type GuestSessionRecord } from '../lib/apiClient'
import { useOperationLogStore } from './operationLogStore'

export interface RentalSession {
  id: string
  label: string
  token: string
  createdAt: number
  expiresAt: number
  magicLink?: string
}

interface RentalStore {
  sessions: RentalSession[]
  loading: boolean
  error: string | null
  generate: (label: string, ttlSeconds: number) => Promise<boolean>
  revoke: (id: string) => Promise<boolean>
}

export const useRentalStore = create<RentalStore>()((set) => ({
  sessions: [],
  loading: false,
  error: null,
  generate: async (label, ttlSeconds) => {
    const cleanLabel = String(label).trim()
    if (!cleanLabel) {
      set({ error: 'label required' })
      return false
    }
    set({ loading: true, error: null })
    const res = await createGuestSession(cleanLabel, ttlSeconds)
    useOperationLogStore.getState().add(
      'POST /guest/sessions',
      JSON.stringify({ name: cleanLabel, ttl_seconds: ttlSeconds }),
      res.error ? 'error' : 'success',
      res.error ?? JSON.stringify(res.data)
    )
    if (res.error || !res.data?.id) {
      set({ loading: false, error: res.error ?? 'create session failed' })
      return false
    }
    const created = toRentalSession(res.data)
    set((state) => ({
      loading: false,
      error: null,
      sessions: [created, ...state.sessions],
    }))
    return true
  },
  revoke: async (id) => {
    const targetID = String(id).trim()
    if (!targetID) {
      set({ error: 'session id required' })
      return false
    }
    set({ loading: true, error: null })
    const res = await deleteGuestSession(targetID)
    useOperationLogStore.getState().add(
      'DELETE /guest/sessions/{id}',
      JSON.stringify({ id: targetID }),
      res.error ? 'error' : 'success',
      res.error ?? JSON.stringify(res.data)
    )
    if (res.error) {
      set({ loading: false, error: res.error })
      return false
    }
    set((state) => ({
      loading: false,
      error: null,
      sessions: state.sessions.filter((s) => s.id !== targetID),
    }))
    return true
  },
}))

function toRentalSession(data: GuestSessionRecord): RentalSession {
  const expiresAtMs = Number(data.expires_at) * 1000
  const createdAtMs = Date.now()
  return {
    id: String(data.id),
    label: String(data.name || 'guest'),
    token: String(data.pin || ''),
    createdAt: Number.isFinite(createdAtMs) ? createdAtMs : Date.now(),
    expiresAt: Number.isFinite(expiresAtMs) ? expiresAtMs : Date.now(),
    magicLink: data.magic_link,
  }
}
