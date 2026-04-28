import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { AppSettings } from '../types'

const DEFAULT: AppSettings = {
  // Production frontend domain: vbs.cyblisswisdom.org
  // BFF proxy base on same-origin console backend
  apiBaseUrl: '/api/proxy',
  engineBaseUrl: 'https://vbsswitcher.cyblisswisdom.org',
  routeBaseUrl: '',
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
