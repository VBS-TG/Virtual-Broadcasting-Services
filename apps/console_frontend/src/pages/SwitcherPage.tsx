import { useEffect } from 'react'
import { useSwitcherStore } from '../stores/switcherStore'
import Switcher from '../components/Switcher'
import Multiviewer from '../components/Multiviewer'
import { ExternalLink } from 'lucide-react'

export default function SwitcherPage() {
  const { state, fetchState, error } = useSwitcherStore()

  useEffect(() => { fetchState() }, [fetchState])

  const openPopout = (path: string, name: string) => {
    window.open(path, name, 'width=1000,height=600,menubar=no,toolbar=no,location=no,status=no')
  }

  return (
    <div className="h-full w-full overflow-hidden p-3 md:p-4 flex flex-col xl:flex-row gap-4">
      {/* MutiView Section (Left) */}
      <div className="flex-1 flex flex-col gap-2 min-w-0 min-h-0">
        <div className="flex items-center justify-between shrink-0">
          <h2 className="text-[15px] font-black text-vbs-muted uppercase tracking-widest flex items-center gap-2">
            MutiView
            {error && <span className="text-[13px] text-vbs-pgm font-semibold ml-2">{error}</span>}
          </h2>
          <button
            onClick={() => openPopout('/popout/multiviewer', 'mutiview_popout')}
            className="text-vbs-accent hover:text-vbs-accent/70 transition-colors flex items-center gap-1 text-[12px] bg-vbs-accent/10 px-2 py-1 rounded-md"
            title="獨立視窗"
          >
            <ExternalLink className="w-3 h-3" />
            <span>彈出</span>
          </button>
        </div>
        <div className="flex-1 min-h-0">
          <Multiviewer pgm={state.program} pvw={state.preview} fullScreen />
        </div>
      </div>

      {/* Switcher Section (Right) */}
      <div className="w-full xl:w-auto shrink-0 flex flex-col gap-2 overflow-y-auto max-h-full">
        <div className="flex items-center justify-between shrink-0">
          <h2 className="text-[15px] font-black text-vbs-muted uppercase tracking-widest">Virtual Switcher</h2>
          <button
            onClick={() => openPopout('/popout/switcher', 'switcher_popout')}
            className="text-vbs-accent hover:text-vbs-accent/70 transition-colors flex items-center gap-1 text-[12px] bg-vbs-accent/10 px-2 py-1 rounded-md"
            title="獨立視窗"
          >
            <ExternalLink className="w-3 h-3" />
            <span>彈出</span>
          </button>
        </div>
        <div className="flex-1">
          <Switcher compact />
        </div>
      </div>
    </div>
  )
}
