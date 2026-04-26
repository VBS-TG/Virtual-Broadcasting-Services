import { create } from 'zustand'
import type { SwitchState } from '../types'
import { getSwitchState, switchProgram, switchPreview, switchAux } from '../lib/apiClient'
import { useOperationLogStore } from './operationLogStore'

const DEFAULT_STATE: SwitchState = {
  program: 1,
  preview: 2,
  aux: { '1': 1, '2': 2, '3': 3, '4': 4 },
}

interface SwitcherState {
  state: SwitchState
  loading: boolean
  error: string | null
  fetchState: () => Promise<void>
  setProgram: (input: number) => Promise<void>
  setPreview: (input: number) => Promise<void>
  setAux: (channel: number, input: number) => Promise<void>
}

export const useSwitcherStore = create<SwitcherState>((set, get) => ({
  state: DEFAULT_STATE,
  loading: false,
  error: null,

  fetchState: async () => {
    set({ loading: true })
    const res = await getSwitchState()
    if (res.error) { set({ loading: false, error: res.error }); return }
    set({ loading: false, state: res.data ?? DEFAULT_STATE })
  },

  setProgram: async (input) => {
    const prev = get().state.program
    // Optimistic update
    set((s) => ({ state: { ...s.state, program: input }, error: null }))
    const res = await switchProgram(input)
    useOperationLogStore.getState().add(
      'POST /switch/program', `{"input":${input}}`,
      res.error ? 'error' : 'success', res.error
    )
    if (res.error) {
      // Rollback
      set((s) => ({ state: { ...s.state, program: prev }, error: res.error! }))
    }
  },

  setPreview: async (input) => {
    const prev = get().state.preview
    set((s) => ({ state: { ...s.state, preview: input }, error: null }))
    const res = await switchPreview(input)
    useOperationLogStore.getState().add(
      'POST /switch/preview', `{"input":${input}}`,
      res.error ? 'error' : 'success', res.error
    )
    if (res.error) {
      set((s) => ({ state: { ...s.state, preview: prev }, error: res.error! }))
    }
  },

  setAux: async (channel, input) => {
    const prev = get().state.aux[String(channel)]
    set((s) => ({
      state: { ...s.state, aux: { ...s.state.aux, [String(channel)]: input } },
      error: null,
    }))
    const res = await switchAux(channel, input)
    useOperationLogStore.getState().add(
      'POST /switch/aux', `{"channel":${channel},"input":${input}}`,
      res.error ? 'error' : 'success', res.error
    )
    if (res.error) {
      set((s) => ({
        state: { ...s.state, aux: { ...s.state.aux, [String(channel)]: prev } },
        error: res.error!,
      }))
    }
  },
}))
