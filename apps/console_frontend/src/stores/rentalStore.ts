import { create } from 'zustand'
import { createGuestSession, deleteGuestSession, listGuestSessions } from '../lib/apiClient'
import type { GuestSessionRecord } from '../lib/apiClient'

interface RentalStore {
  sessions: GuestSessionRecord[]
  loading: boolean
  error: string | null
  fetch: () => Promise<void>
  generate: (label: string, ttlSeconds: number) => Promise<boolean>
  revoke: (id: string) => Promise<boolean>
}

export const useRentalStore = create<RentalStore>()((set) => ({
  sessions: [],
  loading: false,
  error: null,

  fetch: async () => {
    set({ loading: true, error: null })
    const res = await listGuestSessions()
    if (res.error) {
      set({ loading: false, error: res.error })
    } else {
      set({ loading: false, sessions: res.data ?? [] })
    }
  },

  generate: async (label, ttlSeconds) => {
    set({ loading: true, error: null })
    const res = await createGuestSession(label, ttlSeconds)
    if (res.error) {
      set({ loading: false, error: res.error })
      return false
    }
    const listRes = await listGuestSessions()
    set({
      loading: false,
      sessions: listRes.data ?? [],
      error: listRes.error ?? null,
    })
    return !listRes.error
  },

  revoke: async (id) => {
    set({ loading: true, error: null })
    const res = await deleteGuestSession(id)
    if (res.error) {
      set({ loading: false, error: res.error })
      return false
    }
    set((state) => ({
      loading: false,
      sessions: state.sessions.filter((s) => s.id !== id),
    }))
    return true
  },
}))
