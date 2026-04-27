import { useState, useEffect } from 'react'
import { KeyRound, Plus, Trash2, Clock, RefreshCw } from 'lucide-react'
import { useRentalStore } from '../stores/rentalStore'

export default function RentalSessions() {
  const { sessions, generate, revoke, loading, error, fetch } = useRentalStore()
  const [label, setLabel] = useState('')
  const [ttlHrs, setTtlHrs] = useState('24')

  const handleGenerate = async () => {
    if (!label.trim()) return
    const hrs = parseInt(ttlHrs, 10) || 24
    const ok = await generate(label, hrs * 3600)
    if (ok) setLabel('')
  }

  const [now, setNow] = useState(Date.now())
  useEffect(() => {
    void useRentalStore.getState().fetch()
  }, [])
  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(t)
  }, [])

  return (
    <div className="h-full overflow-y-auto p-4 md:p-6 lg:p-8">
      <div className="grid grid-cols-1 md:grid-cols-12 gap-5 auto-rows-min max-w-[1600px] mx-auto">
        
        {/* Title */}
        <div className="md:col-span-12 glass rounded-2xl px-6 py-4 flex items-center justify-between shadow-lg">
          <div className="flex items-center gap-3">
            <KeyRound className="w-6 h-6 text-vbs-accent" />
            <h1 className="text-xl font-black text-white tracking-widest uppercase">Rental Sessions</h1>
          </div>
          <span className="text-[12px] font-bold text-vbs-muted uppercase tracking-widest">Token Management</span>
        </div>

        {error && (
          <div className="md:col-span-12 glass border border-vbs-pgm/40 rounded-2xl px-5 py-3">
            <p className="text-[14px] text-vbs-pgm font-semibold">{error}</p>
          </div>
        )}

        {/* Generator Card (Span 4) */}
        <div className="md:col-span-4 glass rounded-3xl p-6 shadow-xl flex flex-col gap-6">
          <span className="text-[12px] font-bold text-vbs-muted uppercase tracking-widest">Create New Token</span>
          <div className="flex flex-col gap-4 flex-1">
            <div className="flex flex-col gap-1.5">
              <label className="text-[11px] font-bold text-vbs-muted uppercase tracking-widest">Label / Event Name</label>
              <input type="text" value={label} onChange={(e) => setLabel(e.target.value)} placeholder="e.g. Esports Final Guest"
                className="glass-dark border border-white/10 rounded-xl px-4 py-3 text-white focus:border-vbs-accent/50 outline-none transition-colors" />
            </div>
            <div className="flex flex-col gap-1.5">
              <label className="text-[11px] font-bold text-vbs-muted uppercase tracking-widest">Duration (Hours)</label>
              <input type="number" value={ttlHrs} onChange={(e) => setTtlHrs(e.target.value)} min="1" max="720"
                className="glass-dark border border-white/10 rounded-xl px-4 py-3 text-white font-mono focus:border-vbs-accent/50 outline-none transition-colors" />
            </div>
          </div>
          <button onClick={handleGenerate} disabled={!label.trim() || loading}
            className="w-full btn-gradient rounded-2xl py-4 flex items-center justify-center gap-2 font-black text-[16px] disabled:opacity-50 transition-all active:scale-95 shadow-lg">
            <Plus className="w-5 h-5" /> {loading ? 'GENERATING…' : 'GENERATE'}
          </button>
        </div>

        {/* List Card (Span 8) */}
        <div className="md:col-span-8 glass rounded-3xl p-6 shadow-xl flex flex-col min-h-[400px]">
          <div className="flex justify-between items-center mb-4">
            <span className="text-[12px] font-bold text-vbs-muted uppercase tracking-widest">Active Sessions</span>
            <button
              type="button"
              onClick={() => void fetch()}
              disabled={loading}
              className="text-vbs-accent hover:text-white transition-colors"
            >
              <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
            </button>
          </div>
          
          {error && (
            <div className="bg-vbs-pgm/20 border border-vbs-pgm/50 text-vbs-pgm px-4 py-2 rounded-xl mb-4 text-[13px] font-bold">
              {error}
            </div>
          )}

          <div className="flex flex-col gap-3 flex-1">
            {sessions.length === 0 ? (
              <div className="flex-1 flex flex-col items-center justify-center opacity-50">
                <KeyRound className="w-16 h-16 text-vbs-muted mb-4" />
                <span className="text-[14px] font-bold text-vbs-muted uppercase tracking-widest">No active sessions</span>
              </div>
            ) : (
              sessions.map((s) => {
                const expiresAtMs = s.expires_at * 1000
                const expired = now > expiresAtMs
                const remainingSecs = Math.max(0, Math.floor((expiresAtMs - now) / 1000))
                const hrs = Math.floor(remainingSecs / 3600)
                const mins = Math.floor((remainingSecs % 3600) / 60)
                
                return (
                  <div key={s.id} className={`glass-dark border ${expired ? 'border-vbs-pgm/30 opacity-50' : 'border-white/5'} rounded-2xl p-4 flex items-center justify-between shadow-md`}>
                    <div className="flex flex-col">
                      <span className="text-[12px] font-bold text-vbs-muted uppercase tracking-widest">{s.name}</span>
                      <span className="text-2xl font-black text-white font-mono tracking-widest drop-shadow-md mt-1">{s.pin}</span>
                    </div>
                    <div className="flex items-center gap-6">
                      <div className="flex flex-col items-end">
                        <span className="text-[11px] font-bold text-vbs-muted uppercase tracking-widest flex items-center gap-1">
                          <Clock className="w-3 h-3" /> {expired ? 'EXPIRED' : 'TIME LEFT'}
                        </span>
                        <span className={`text-xl font-black tracking-tight drop-shadow-md mt-1 tabular-nums ${expired ? 'text-vbs-pgm' : 'text-vbs-pvw'}`}>
                          {expired ? '0h 0m' : `${hrs}h ${mins}m`}
                        </span>
                      </div>
                      <button onClick={() => void revoke(s.id)} disabled={loading}
                        className="w-12 h-12 rounded-xl glass border border-vbs-pgm/30 text-vbs-pgm flex items-center justify-center hover:bg-vbs-pgm/20 hover:border-vbs-pgm/80 transition-all active:scale-95 shadow-md">
                        <Trash2 className="w-5 h-5" />
                      </button>
                    </div>
                  </div>
                )
              })
            )}
          </div>
        </div>

      </div>
    </div>
  )
}
