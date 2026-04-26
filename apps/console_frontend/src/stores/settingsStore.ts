import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { AppSettings } from '../types'

const DEFAULT: AppSettings = {
  // Formal deployment should use same-origin reverse proxy for /api.
  apiBaseUrl: '',
  engineBaseUrl: 'https://vbsswitcher.cyblisswisdom.org',
  refreshInterval: 1000,
  theme: 'dark',
  apiTimeoutMs: 10000,
}

interface SettingsState {
  settings: AppSettings
  update: (partial: Partial<AppSettings>) => void
  reset: () => void
}

export const useSettingsStore = create<SettingsState>()(
  persist(
    (set) => ({
      settings: DEFAULT,
      update: (partial) => set((s) => ({ settings: { ...s.settings, ...partial } })),
      reset: () => set({ settings: DEFAULT }),
    }),
    { name: 'vbs-settings' }
  )
)
