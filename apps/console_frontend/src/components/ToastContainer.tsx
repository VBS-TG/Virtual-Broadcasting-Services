import { useToastStore } from '../stores/toastStore'

export default function ToastContainer() {
  const toasts = useToastStore((s) => s.toasts)
  const removeToast = useToastStore((s) => s.removeToast)
  
  if (toasts.length === 0) return null;
  
  return (
    <div className="fixed top-4 right-4 z-50 flex flex-col gap-2 pointer-events-none">
      {toasts.map((t) => (
        <div key={t.id} className={`p-4 rounded-xl shadow-lg border backdrop-blur-md min-w-[280px] pointer-events-auto cursor-pointer animate-slide-in
          ${t.type === 'error' ? 'bg-vbs-pgm/20 border-vbs-pgm/50 text-vbs-pgm' :
            t.type === 'success' ? 'bg-vbs-pvw/20 border-vbs-pvw/50 text-vbs-pvw' :
            t.type === 'warning' ? 'bg-vbs-warning/20 border-vbs-warning/50 text-vbs-warning' :
            'bg-vbs-navy/50 border-white/10 text-white'}`}
          onClick={() => removeToast(t.id)}
        >
          <div className="font-bold text-[15px]">{t.title}</div>
          {t.message && <div className="text-[14px] mt-1 opacity-80">{t.message}</div>}
        </div>
      ))}
    </div>
  )
}
