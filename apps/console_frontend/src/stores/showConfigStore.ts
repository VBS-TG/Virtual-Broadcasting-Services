import { create } from 'zustand'
import type { ShowConfigPayload, ShowConfigHistoryRow } from '../types'
import type { ShowConfigApplyResponse } from '../lib/apiClient'
import {
  defaultShowConfigDraft,
  getShowConfig,
  getShowConfigHistory,
  postShowConfigApply,
  postShowConfigRollback,
  putShowConfigDraft,
} from '../lib/apiClient'

interface ShowConfigState {
  loading: boolean
  saving: boolean
  applying: boolean
  error: string | null
  lastApplyMessage: string | null
  draft: ShowConfigPayload | null
  draft_updated_at: number | null
  effective_version: number
  effective_updated_at: number | null
  history: ShowConfigHistoryRow[]
  fetch: () => Promise<void>
  fetchHistory: () => Promise<void>
  saveDraft: (cfg: ShowConfigPayload) => Promise<boolean>
  setLocalDraft: (cfg: ShowConfigPayload) => void
  apply: () => Promise<boolean>
  rollback: () => Promise<boolean>
}

function mergeDraft(raw: ShowConfigPayload | null): ShowConfigPayload {
  const base = defaultShowConfigDraft()
  if (!raw || typeof raw !== 'object') return base
  const t = raw.profile?.target
  return {
    ...base,
    ...raw,
    schema_version: raw.schema_version || base.schema_version,
    profile: {
      ...base.profile,
      ...raw.profile,
      mode: raw.profile?.mode || base.profile.mode,
      target: t
        ? { width: t.width, height: t.height, frame_rate: t.frame_rate }
        : base.profile.target,
    },
    sources: Array.isArray(raw.sources) ? raw.sources : [],
    switcher: raw.switcher || base.switcher,
    multiview: raw.multiview || base.multiview,
  }
}

export const useShowConfigStore = create<ShowConfigState>((set, get) => ({
  loading: false,
  saving: false,
  applying: false,
  error: null,
  lastApplyMessage: null,
  draft: null,
  draft_updated_at: null,
  effective_version: 0,
  effective_updated_at: null,
  history: [],

  fetch: async () => {
    set({ loading: true, error: null })
    const res = await getShowConfig()
    if (res.error) {
      set({ loading: false, error: res.error })
      return
    }
    const d = res.data!
    set({
      loading: false,
      draft: mergeDraft(d.draft),
      draft_updated_at: d.draft_updated_at,
      effective_version: d.effective_version,
      effective_updated_at: d.effective_updated_at,
    })
  },

  fetchHistory: async () => {
    const res = await getShowConfigHistory(50)
    if (res.error) {
      set({ error: res.error })
      return
    }
    set({ history: res.data?.history ?? [] })
  },

  setLocalDraft: (cfg) => set({ draft: cfg }),

  saveDraft: async (cfg) => {
    set({ saving: true, error: null })
    const res = await putShowConfigDraft(cfg)
    if (res.error) {
      set({ saving: false, error: res.error })
      return false
    }
    const saved = res.data?.draft ?? cfg
    const at =
      res.data?.draft_updated_at != null ? Number(res.data.draft_updated_at) : Math.floor(Date.now() / 1000)
    set({
      saving: false,
      error: null,
      draft: mergeDraft(saved),
      draft_updated_at: at,
    })
    return true
  },

  apply: async () => {
    set({ applying: true, error: null, lastApplyMessage: null })
    const res = await postShowConfigApply()
    if (res.error) {
      set({ applying: false, error: res.error, lastApplyMessage: res.error })
      return false
    }
    const data = (res.data ?? {}) as ShowConfigApplyResponse
    const msg = data.message ? String(data.message) : data.ok ? '套用成功' : '套用未完成'
    set({
      applying: false,
      error: null,
      lastApplyMessage: msg,
      effective_version: Number(data.effective_version ?? get().effective_version),
      effective_updated_at:
        data.effective_updated_at != null ? Number(data.effective_updated_at) : get().effective_updated_at,
    })
    await get().fetch()
    await get().fetchHistory()
    return Boolean(data.ok)
  },

  rollback: async () => {
    set({ applying: true, error: null, lastApplyMessage: null })
    const res = await postShowConfigRollback()
    if (res.error) {
      set({ applying: false, error: res.error, lastApplyMessage: res.error })
      return false
    }
    const data = (res.data ?? {}) as ShowConfigApplyResponse
    set({
      applying: false,
      error: null,
      lastApplyMessage: data.ok ? '回滾完成' : String(data.message ?? ''),
      effective_version: Number(data.effective_version ?? get().effective_version),
      effective_updated_at:
        data.effective_updated_at != null ? Number(data.effective_updated_at) : get().effective_updated_at,
    })
    await get().fetch()
    await get().fetchHistory()
    return Boolean(data.ok)
  },
}))
