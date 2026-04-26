import { useEffect } from 'react'
import { useSwitcherStore } from '../stores/switcherStore'
import Multiviewer from '../components/Multiviewer'

export default function PopoutMultiview() {
  const { state, fetchState } = useSwitcherStore()

  useEffect(() => { fetchState() }, [fetchState])

  return (
    <div className="w-screen h-screen bg-[#050508] overflow-hidden flex flex-col p-2 gap-2">
      <div className="flex items-center justify-between shrink-0">
        <h2 className="text-[14px] font-black text-vbs-muted uppercase tracking-widest">MutiView</h2>
      </div>
      <div className="flex-1 overflow-hidden">
        <Multiviewer pgm={state.program} pvw={state.preview} fullScreen />
      </div>
    </div>
  )
}
