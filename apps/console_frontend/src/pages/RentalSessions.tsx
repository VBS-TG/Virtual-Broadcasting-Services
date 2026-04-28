import { useState, useEffect } from 'react'
import { KeyRound, Plus, Trash2, Clock, RefreshCw, Ticket } from 'lucide-react'
import { useRentalStore } from '../stores/rentalStore'
import PageShell from '../components/PageShell'

export default function RentalSessions() {
  const { sessions, generate, revoke, loading, error, fetch } = useRentalStore()
  const [label, setLabel] = useState('')
  const [ttlHrs, setTtlHrs] = useState('24')
  const [now, setNow] = useState(Date.now())

  useEffect(() => {
    fetch()
  }, [fetch])

  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(t)
  }, [])

  const handleGenerate = async () => {
    if (!label.trim()) return
    const hrs = parseInt(ttlHrs, 10) || 24
    const ok = await generate(label, hrs * 3600)
    if (ok) setLabel('')
  }

  return (
    <PageShell
      title="租賃"
      description="Guest Access Management"
      extra={
        <button
          onClick={fetch}
          disabled={loading}
          className="w-10 h-10 glass-dark border border-white/10 rounded-xl flex items-center justify-center text-vbs-muted hover:text-white transition-all shadow-lg"
        >
          <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
        </button>
      }
    >
      <div className="grid grid-cols-1 md:grid-cols-12 gap-6">
        {error && (
          <div className="md:col-span-12 glass bg-vbs-pgm/10 border border-vbs-pgm/30 rounded-[24px] px-6 py-4 text-[14px] font-black text-vbs-pgm uppercase tracking-widest">
            {error}
          </div>
        )}

        {/* Generator Card */}
        <div className="md:col-span-4 glass rounded-[36px] p-8 shadow-2xl flex flex-col gap-8 h-fit sticky top-0">
          <div className="flex flex-col gap-1">
            <span className="text-[12px] font-black text-vbs-muted uppercase tracking-widest">Create Access Token</span>
            <h2 className="text-2xl font-black text-white uppercase tracking-tighter">生成新租約</h2>
          </div>
          
          <div className="flex flex-col gap-6">
            <div className="flex flex-col gap-2">
              <label className="text-[11px] font-black text-vbs-muted uppercase tracking-widest ml-1">標籤名稱 / Event Name</label>
              <input 
                type="text" 
                value={label} 
                onChange={(e) => setLabel(e.target.value)} 
                placeholder="例如: 第五頻道外播嘉賓"
                className="w-full glass-dark border border-white/5 rounded-2xl px-5 py-4 text-white font-bold placeholder:text-vbs-muted/30 outline-none focus:border-vbs-accent/40 focus:shadow-[0_0_20px_rgba(59,130,246,0.1)] transition-all" 
              />
            </div>
            
            <div className="flex flex-col gap-2">
              <label className="text-[11px] font-black text-vbs-muted uppercase tracking-widest ml-1">有效時數 (Hours)</label>
              <div className="relative">
                <input 
                  type="number" 
                  value={ttlHrs} 
                  onChange={(e) => setTtlHrs(e.target.value)} 
                  min="1" 
                  max="720"
                  className="w-full glass-dark border border-white/5 rounded-2xl px-5 py-4 text-white font-black tracking-widest focus:border-vbs-accent/40 outline-none transition-all tabular-nums" 
                />
                <span className="absolute right-5 top-1/2 -translate-y-1/2 text-[12px] font-black text-vbs-muted uppercase">Hrs</span>
              </div>
            </div>
          </div>

          <button 
            onClick={handleGenerate} 
            disabled={!label.trim() || loading}
            className="w-full h-16 rounded-[22px] bg-vbs-accent hover:bg-vbs-accent/90 disabled:opacity-30 disabled:cursor-not-allowed transition-all active:scale-[0.98] shadow-[0_10px_20px_rgba(59,130,246,0.3)] flex items-center justify-center gap-3 text-white font-black text-[16px] uppercase tracking-widest"
          >
            {loading ? <RefreshCw className="w-5 h-5 animate-spin" /> : <Plus className="w-6 h-6" />}
            {loading ? 'Generating…' : '確認生成'}
          </button>
        </div>

        {/* Sessions List */}
        <div className="md:col-span-8 flex flex-col gap-4">
          <div className="px-2 flex items-center justify-between mb-2">
             <span className="text-[12px] font-black text-vbs-muted uppercase tracking-widest">Active Rental Sessions</span>
             <span className="text-[12px] font-black text-vbs-accent bg-vbs-accent/10 px-3 py-1 rounded-full uppercase tracking-tighter border border-vbs-accent/20">{sessions.length} 活躍中</span>
          </div>

          {sessions.length === 0 ? (
            <div className="glass rounded-[40px] p-24 flex flex-col items-center justify-center text-center">
              <KeyRound className="w-20 h-20 text-vbs-muted mb-8 opacity-10" />
              <p className="text-[15px] font-black text-vbs-muted uppercase tracking-widest opacity-40">目前無活躍租約紀錄</p>
            </div>
          ) : (
            <div className="grid grid-cols-1 gap-4">
              {sessions.map((s) => {
                const expiresAtMs = s.expires_at * 1000
                const expired = now > expiresAtMs
                const remainingSecs = Math.max(0, Math.floor((expiresAtMs - now) / 1000))
                const hrs = Math.floor(remainingSecs / 3600)
                const mins = Math.floor((remainingSecs % 3600) / 60)
                
                return (
                  <div key={s.id} className={`glass rounded-[32px] p-6 flex flex-col sm:flex-row sm:items-center justify-between gap-6 border border-white/5 hover:border-white/10 transition-all group shadow-xl relative overflow-hidden ${expired ? 'opacity-50 grayscale' : ''}`}>
                    <div className="flex items-center gap-5">
                      <div className="w-14 h-14 rounded-[22px] bg-vbs-accent/10 flex items-center justify-center border border-vbs-accent/20 shrink-0 group-hover:scale-105 transition-transform">
                        <Ticket className="w-7 h-7 text-vbs-accent" />
                      </div>
                      <div className="flex flex-col">
                        <span className="text-[11px] font-black text-vbs-muted uppercase tracking-widest mb-1">{s.name}</span>
                        <span className="text-3xl font-black text-white font-mono tracking-[0.2em] drop-shadow-md leading-none">{s.pin}</span>
                      </div>
                    </div>

                    <div className="flex items-center gap-8 justify-between sm:justify-end">
                      <div className="flex flex-col items-end">
                        <span className="text-[10px] font-black text-vbs-muted uppercase tracking-widest flex items-center gap-1.5 mb-1">
                          <Clock className="w-3 h-3" /> {expired ? '已過期' : '剩餘時間'}
                        </span>
                        <span className={`text-xl font-black tracking-tighter drop-shadow-md tabular-nums ${expired ? 'text-vbs-pgm' : 'text-vbs-pvw'}`}>
                          {expired ? '0H 0M' : `${hrs}H ${mins}M`}
                        </span>
                      </div>
                      
                      <button 
                        onClick={() => void revoke(s.id)} 
                        disabled={loading}
                        className="w-14 h-14 rounded-2xl glass-dark border border-white/5 text-vbs-muted hover:text-vbs-pgm hover:border-vbs-pgm/30 flex items-center justify-center transition-all active:scale-90 shadow-lg"
                        title="撤銷租約"
                      >
                        <Trash2 className="w-6 h-6" />
                      </button>
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </div>
    </PageShell>
  )
}

