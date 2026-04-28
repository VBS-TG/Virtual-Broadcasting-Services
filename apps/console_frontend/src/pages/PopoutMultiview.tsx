import { useEffect, useMemo, useState } from 'react'
import { useSwitcherStore } from '../stores/switcherStore'
import Multiviewer from '../components/Multiviewer'
import { useShowConfigStore } from '../stores/showConfigStore'

export default function PopoutMultiview() {
  const { state, fetchState } = useSwitcherStore()
  const { draft, fetch, updateDraft } = useShowConfigStore()
  const [activeCell, setActiveCell] = useState<number | null>(0)

  useEffect(() => {
    fetchState()
    fetch()
  }, [fetchState, fetch])

  const inputLabels = useMemo(
    () => Array.from({ length: 8 }, (_, i) => draft?.sources?.find((s) => s.slot_id === `input${i + 1}`)?.display_name || `Source${i + 1}`),
    [draft]
  )
  const cellSources = useMemo(
    () => Array.from({ length: 8 }, (_, i) => String((draft?.multiview?.cells as any[])?.[i]?.source ?? `input${i + 1}`)),
    [draft]
  )
  const buttonMappings = useMemo(
    () => Array.from({ length: 8 }, (_, i) => String(((draft?.switcher?.rows as any[])?.[0]?.buttons ?? [])[i]?.source ?? `input${i + 1}`)),
    [draft]
  )

  return (
    <div className="w-screen h-screen bg-[#050508] overflow-hidden flex flex-col p-2 gap-2">
      <div className="flex items-center justify-between shrink-0">
        <h2 className="text-[14px] font-black text-vbs-muted uppercase tracking-widest">MutiView</h2>
      </div>
      <div className="flex-1 overflow-hidden">
        <Multiviewer
          pgm={state.program}
          pvw={state.preview}
          fullScreen
          editable
          inputLabels={inputLabels}
          cellSources={cellSources}
          buttonMappings={buttonMappings}
          activeCellIndex={activeCell}
          onActiveCellChange={setActiveCell}
          onCellSourceChange={(cellIndex, source) => {
            updateDraft((old) => {
              const cells = [...(((old.multiview?.cells as any[]) ?? []))]
              while (cells.length < 8) cells.push({ source: `input${cells.length + 1}` })
              cells[cellIndex] = { ...cells[cellIndex], source }
              return { ...old, multiview: { ...old.multiview, cells } }
            })
          }}
        />
      </div>
    </div>
  )
}
