import React from 'react'

interface PageShellProps {
  title: string
  description?: string
  children: React.ReactNode
  extra?: React.ReactNode
}

export default function PageShell({ title, description, children, extra }: PageShellProps) {
  return (
    <div className="h-full flex flex-col overflow-hidden">
      {/* Header */}
      <header className="px-6 py-5 flex items-center justify-between border-b border-white/5 shrink-0">
        <div className="flex flex-col">
          <h1 className="text-xl font-black text-white tracking-widest uppercase">{title}</h1>
          {description && (
            <p className="text-[11px] font-bold text-vbs-muted uppercase tracking-widest mt-0.5">{description}</p>
          )}
        </div>
        {extra && <div className="flex items-center gap-4">{extra}</div>}
      </header>

      {/* Content */}
      <main className="flex-1 overflow-y-auto p-4 md:p-6 lg:p-8 custom-scrollbar">
        <div className="max-w-[1600px] mx-auto">
          {children}
        </div>
      </main>
    </div>
  )
}
