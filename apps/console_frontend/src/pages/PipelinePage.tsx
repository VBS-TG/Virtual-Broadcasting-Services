import { useEffect } from 'react'
import { Network, Activity, Globe } from 'lucide-react'
import { useShowConfigStore } from '../stores/showConfigStore'
import PageShell from '../components/PageShell'

export default function PipelinePage() {
  const { draft, loading, error, fetch } = useShowConfigStore()

  useEffect(() => {
    fetch()
  }, [fetch])

  if (loading && !draft) {
    return (
      <PageShell title="鏈路" description="Pipeline Monitoring">
        <div className="h-[400px] flex items-center justify-center text-vbs-muted text-[15px] font-black uppercase tracking-widest animate-pulse">
          Loading Pipeline Architecture…
        </div>
      </PageShell>
    )
  }

  if (!draft) {
    return (
      <PageShell title="鏈路" description="Pipeline Monitoring">
        <div className="h-[400px] flex items-center justify-center text-vbs-pgm text-[15px] font-black uppercase tracking-widest">
          {error ?? 'Failed to load pipeline configuration'}
        </div>
      </PageShell>
    )
  }

  const t = draft.profile.target ?? { width: 1920, height: 1080, frame_rate: 60 }

  return (
    <PageShell 
      title="鏈路" 
      description="Pipeline Monitoring"
      extra={
        <div className="flex items-center gap-2">
          <span className="text-[10px] font-black text-vbs-muted uppercase tracking-widest">Profile Mode:</span>
          <span className="text-[12px] font-black text-white bg-white/5 px-3 py-1 rounded-lg uppercase tracking-tighter border border-white/5">{draft.profile.mode}</span>
        </div>
      }
    >
      <div className="grid grid-cols-1 md:grid-cols-12 gap-6">
        
        {/* ── 目標輸出規格 ── */}
        <div className="md:col-span-4 glass rounded-[32px] p-8 flex flex-col gap-6 shadow-xl relative overflow-hidden">
          <div className="flex items-center gap-3 mb-2">
            <Activity className="w-5 h-5 text-vbs-accent" />
            <span className="text-[14px] font-black text-white uppercase tracking-widest">目標輸出規格</span>
          </div>
          <div className="grid grid-cols-1 gap-6">
            <div className="flex flex-col">
              <span className="text-[11px] font-black text-vbs-muted uppercase tracking-widest mb-1">解析度 (Resolution)</span>
              <span className="text-4xl font-black text-white tracking-tighter tabular-nums">{t.width}<span className="text-vbs-muted mx-2 text-2xl font-medium">×</span>{t.height}</span>
            </div>
            <div className="flex flex-col">
              <span className="text-[11px] font-black text-vbs-muted uppercase tracking-widest mb-1">幀率 (Frame Rate)</span>
              <span className="text-4xl font-black text-vbs-pvw tracking-tighter tabular-nums">{t.frame_rate}<span className="text-[14px] ml-1 font-bold">FPS</span></span>
            </div>
          </div>
          <div className="mt-4 pt-6 border-t border-white/5">
            <p className="text-[13px] font-bold text-vbs-muted leading-relaxed uppercase tracking-tight">
              自動化配置已生效：端到端（Capture → Route → Engine）同步鎖定此規格。
            </p>
          </div>
        </div>

        {/* ── 輸入源清單 (Automation Sources) ── */}
        <div className="md:col-span-8 glass rounded-[32px] p-8 shadow-xl flex flex-col gap-6">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <Network className="w-5 h-5 text-vbs-accent" />
              <span className="text-[14px] font-black text-white uppercase tracking-widest">鏈路輸入來源</span>
            </div>
            <span className="text-[11px] font-black text-vbs-muted uppercase tracking-widest px-3 py-1 bg-white/5 rounded-lg border border-white/5">ReadOnly</span>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            {draft.sources.map((s) => (
              <div key={s.slot_id} className="glass-dark border border-white/5 rounded-2xl p-5 flex items-center gap-4 group hover:border-vbs-accent/30 transition-all">
                <div className="w-12 h-12 rounded-xl bg-vbs-accent/10 flex items-center justify-center shrink-0 border border-vbs-accent/20">
                  <Globe className="w-6 h-6 text-vbs-accent" />
                </div>
                <div className="flex flex-col overflow-hidden">
                  <span className="text-[10px] font-black text-vbs-muted uppercase tracking-widest mb-0.5">{s.slot_id}</span>
                  <span className="text-[15px] font-black text-white truncate uppercase tracking-tight">{s.display_name}</span>
                </div>
              </div>
            ))}
          </div>

          {draft.sources.length === 0 && (
            <div className="h-[200px] flex items-center justify-center text-vbs-muted text-[13px] font-black uppercase tracking-widest border-2 border-dashed border-white/5 rounded-[24px]">
              No sources configured
            </div>
          )}
        </div>

      </div>
    </PageShell>
  )
}

