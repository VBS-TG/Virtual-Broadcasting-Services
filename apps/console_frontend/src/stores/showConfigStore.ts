import { create } from 'zustand'
import type { ShowConfigPayload, ShowConfigHistoryRow } from '../types'
import {
  defaultShowConfigDraft,
  getShowConfig,
  getShowConfigHistory,
} from '../lib/apiClient'

interface ShowConfigState {
  loading: boolean
  error: string | null
  draft: ShowConfigPayload | null
  draft_updated_at: number | null
  effective_version: number
  effective_updated_at: number | null
  history: ShowConfigHistoryRow[]
  fetch: () => Promise<void>
  fetchHistory: () => Promise<void>
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

export const useShowConfigStore = create<ShowConfigState>((set) => ({
  loading: false,
  error: null,
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
}))

