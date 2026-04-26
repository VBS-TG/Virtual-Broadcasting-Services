import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { OperationLogEntry } from '../types'

interface OperationLogState {
  logs: OperationLogEntry[]
  add: (
    operation: string,
    payload: string,
    result: OperationLogEntry['result'],
    details?: string
  ) => void
  clear: () => void
  exportJson: () => void
}

export const useOperationLogStore = create<OperationLogState>()(
  persist(
    (set, get) => ({
      logs: [],

      add: (operation, payload, result, details) => {
        const entry: OperationLogEntry = {
          id: `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
          time: new Date().toISOString(),
          operation,
          payload,
          result,
          details,
        }
        // 最多保留 500 條
        set((s) => ({ logs: [entry, ...s.logs].slice(0, 500) }))
      },

      clear: () => set({ logs: [] }),

      exportJson: () => {
        const data = JSON.stringify(get().logs, null, 2)
        const blob = new Blob([data], { type: 'application/json' })
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = `vbs-log-${new Date().toISOString().slice(0, 10)}.json`
        a.click()
        URL.revokeObjectURL(url)
      },
    }),
    { name: 'vbs-operation-log' }
  )
)
