import { create } from 'zustand'
import type { RuntimeConfig } from '../types'
import { getRuntimeConfig } from '../lib/apiClient'

interface RuntimeState {
  config: RuntimeConfig | null
  loading: boolean
  error: string | null
  fetch: () => Promise<void>
}

export const useRuntimeStore = create<RuntimeState>((set) => ({
  config: null,
  loading: false,
  error: null,

  fetch: async () => {
    set({ loading: true, error: null })
    const res = await getRuntimeConfig()
    if (res.error) { 
      set({ loading: false, error: res.error })
      return 
    }
    set({ loading: false, config: res.data ?? null })
  },
}))

