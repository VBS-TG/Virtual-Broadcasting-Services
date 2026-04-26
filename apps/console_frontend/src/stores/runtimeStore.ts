import { create } from 'zustand'
import type { RuntimeConfig, ApplyResult } from '../types'
import { getRuntimeConfig, putRuntimeConfig, postApplyConfig } from '../lib/apiClient'
import { useOperationLogStore } from './operationLogStore'

interface RuntimeState {
  config: RuntimeConfig | null
  loading: boolean
  saving: boolean
  applying: boolean
  error: string | null
  lastApplyResult: ApplyResult | null
  fetch: () => Promise<void>
  save: (config: RuntimeConfig) => Promise<boolean>
  apply: () => Promise<boolean>
}

export const useRuntimeStore = create<RuntimeState>((set, get) => ({
  config: null,
  loading: false,
  saving: false,
  applying: false,
  error: null,
  lastApplyResult: null,

  fetch: async () => {
    set({ loading: true, error: null })
    const res = await getRuntimeConfig()
    if (res.error) { set({ loading: false, error: res.error }); return }
    set({ loading: false, config: res.data ?? null })
  },

  save: async (config) => {
    set({ saving: true, error: null })
    const res = await putRuntimeConfig(config)
    useOperationLogStore.getState().add(
      'PUT /runtime/config',
      JSON.stringify(config),
      res.error ? 'error' : 'success',
      res.error
    )
    if (res.error) { set({ saving: false, error: res.error }); return false }
    set({ saving: false, config })
    return true
  },

  apply: async () => {
    const cfg = get().config
    set({ applying: true, error: null })
    const res = await postApplyConfig()
    useOperationLogStore.getState().add(
      'POST /runtime/config/apply',
      cfg ? `inputs=${cfg.inputs}` : '{}',
      res.error ? 'error' : 'success',
      res.error ?? JSON.stringify(res.data)
    )
    if (res.error) { set({ applying: false, error: res.error }); return false }
    set({ applying: false, lastApplyResult: res.data ?? null })
    return true
  },
}))
