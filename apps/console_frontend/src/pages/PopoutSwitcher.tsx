import { useEffect } from 'react'
import { useSwitcherStore } from '../stores/switcherStore'
import Switcher from '../components/Switcher'

export default function PopoutSwitcher() {
  const { fetchState } = useSwitcherStore()

  useEffect(() => { fetchState() }, [fetchState])

  return (
    <div className="w-screen h-screen bg-[#050508] overflow-hidden flex p-1">
      <Switcher fullScreen />
    </div>
  )
}
