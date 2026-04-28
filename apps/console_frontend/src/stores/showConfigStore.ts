import { create } from 'zustand'
import type { ShowConfigPayload, ShowConfigHistoryRow } from '../types'
import {
  defaultShowConfigDraft,
  getShowConfig,
  getShowConfigHistory,
  putShowConfigDraft,
  postShowConfigApply,
  postShowConfigRollback,
} from '../lib/apiClient'
import { useOperationLogStore } from './operationLogStore'

interface ShowConfigState {
  loading: boolean
  saving: boolean
  applying: boolean
  error: string | null
  draft: ShowConfigPayload | null
  draft_updated_at: number | null
  effective_version: number
  effective_updated_at: number | null
  history: ShowConfigHistoryRow[]
  fetch: () => Promise<void>
  fetchHistory: () => Promise<void>
  updateDraft: (updater: (draft: ShowConfigPayload) => ShowConfigPayload) => void
  saveDraft: () => Promise<boolean>
  applyDraft: () => Promise<boolean>
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

export const useShowConfigStore = create<ShowConfigState>((set) => ({
  loading: false,
  saving: false,
  applying: false,
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

  updateDraft: (updater) => {
    set((s) => {
      const base = mergeDraft(s.draft)
      return { draft: updater(base) }
    })
  },

  saveDraft: async () => {
    let currentDraft: ShowConfigPayload | null = null
    set((s) => {
      currentDraft = s.draft ? mergeDraft(s.draft) : null
      return { saving: true, error: null }
    })
    if (!currentDraft) {
      set({ saving: false, error: 'draft 不存在' })
      return false
    }
    const res = await putShowConfigDraft(currentDraft)
    useOperationLogStore.getState().add(
      'PUT /show-config/draft',
      JSON.stringify(currentDraft),
      res.error ? 'error' : 'success',
      res.error
    )
    if (res.error) {
      set({ saving: false, error: res.error })
      return false
    }
    set({
      saving: false,
      draft: mergeDraft(res.data?.draft ?? currentDraft),
      draft_updated_at: res.data?.draft_updated_at ?? Math.floor(Date.now() / 1000),
    })
    return true
  },

  applyDraft: async () => {
    set({ applying: true, error: null })
    const res = await postShowConfigApply()
    useOperationLogStore.getState().add(
      'POST /show-config/apply',
      '{}',
      res.error ? 'error' : 'success',
      res.error
    )
    if (res.error) {
      set({ applying: false, error: res.error })
      return false
    }
    set({
      applying: false,
      effective_version: Number(res.data?.effective_version ?? 0),
      effective_updated_at: Number(res.data?.effective_updated_at ?? Math.floor(Date.now() / 1000)),
    })
    return true
  },

  rollback: async () => {
    set({ applying: true, error: null })
    const res = await postShowConfigRollback()
    useOperationLogStore.getState().add(
      'POST /show-config/rollback',
      '{}',
      res.error ? 'error' : 'success',
      res.error
    )
    if (res.error) {
      set({ applying: false, error: res.error })
      return false
    }
    set({
      applying: false,
      effective_version: Number(res.data?.effective_version ?? 0),
      effective_updated_at: Number(res.data?.effective_updated_at ?? Math.floor(Date.now() / 1000)),
    })
    return true
  },
}))

