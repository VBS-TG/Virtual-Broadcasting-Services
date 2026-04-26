import { create } from 'zustand'
import type { TelemetryLatest } from '../types'
import { getTelemetryLatest } from '../lib/apiClient'

interface TelemetryState {
  data: TelemetryLatest | null
  loading: boolean
  error: string | null
  autoRefresh: boolean
  refreshInterval: number // ms
  fetch: () => Promise<void>
  setAutoRefresh: (v: boolean) => void
  setRefreshInterval: (ms: number) => void
}

export const useTelemetryStore = create<TelemetryState>((set) => ({
  data: null,
  loading: false,
  error: null,
  autoRefresh: true,
  refreshInterval: 1000,

  fetch: async () => {
    set({ loading: true })
    const res = await getTelemetryLatest()
    if (res.error) { set({ loading: false, error: res.error }); return }
    set({ loading: false, data: res.data ?? null, error: null })
  },

  setAutoRefresh: (v) => set({ autoRefresh: v }),
  setRefreshInterval: (ms) => set({ refreshInterval: ms }),
}))
