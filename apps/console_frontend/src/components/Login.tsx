import { useState } from 'react'
import logoTxtImg from '../assets/images/vbslogo-txtimg.svg'

interface LoginProps {
  onLogin: () => void
}

export default function Login({ onLogin }: LoginProps) {
  const [code, setCode] = useState('')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (code.trim()) {
      onLogin()
    }
  }

  return (
    <div className="flex flex-col md:flex-row h-full w-full items-center justify-center p-6 bg-vbs-carbon relative overflow-hidden">
      {/* 裝飾性背景光暈 */}
      <div className="absolute top-[-10%] left-[-10%] w-[50%] h-[50%] bg-vbs-blue/30 rounded-full blur-[120px] pointer-events-none" />
      <div className="absolute bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-vbs-navy/50 rounded-full blur-[100px] pointer-events-none" />

      {/* 左側：大 Logo */}
      <div className="flex-1 flex justify-center md:justify-end md:pr-16 lg:pr-24 mb-12 md:mb-0 z-10 w-full max-w-md md:max-w-none">
        <img src={logoTxtImg} alt="VBS" className="w-64 md:w-80 lg:w-96 drop-shadow-2xl animate-slide-in" />
      </div>

      {/* 右側：登入框 */}
      <div className="flex-1 flex justify-center md:justify-start md:pl-16 lg:pl-24 w-full z-10">
        <div className="glass rounded-2xl p-8 w-full max-w-sm animate-slide-in" style={{ animationDelay: '100ms', animationFillMode: 'both' }}>
          <div className="mb-8">
            <h1 className="text-[17px] font-bold text-vbs-text mb-2">系統登入</h1>
            <p className="text-[17px] font-medium text-vbs-muted">請輸入您的信箱或專屬租賃碼</p>
          </div>

          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <div className="flex flex-col gap-1.5">
              <input
                type="text"
                placeholder="Email or Rental Code"
                value={code}
                onChange={e => setCode(e.target.value)}
                className="w-full glass-dark border border-white/10 rounded-xl px-4 py-3
                  text-[9px\] font-medium text-vbs-text bg-transparent outline-none
                  focus:border-vbs-accent/50 focus:shadow-[0_0_15px_rgba(30,144,255,0.2)] transition-all"
                autoFocus
              />
            </div>
            
            <button
              type="submit"
              disabled={!code.trim()}
              className={`
                w-full mt-2 py-3 rounded-xl font-bold text-[9px\] transition-all active:scale-95
                ${code.trim() 
                  ? 'bg-vbs-accent/20 border border-vbs-accent/50 text-vbs-accent hover:bg-vbs-accent/30 hover:border-vbs-accent shadow-[0_0_15px_rgba(30,144,255,0.2)]' 
                  : 'glass-dark border border-white/5 text-vbs-muted cursor-not-allowed'}
              `}
            >
              進入控制台
            </button>
          </form>
          
          <div className="mt-8 pt-4 border-t border-white/5 flex items-center justify-between">
            <span className="text-[17px] text-vbs-muted">Zero Trust Access</span>
            <span className="text-[17px] text-vbs-muted">v1.2.0</span>
          </div>
        </div>
      </div>
    </div>
  )
}
