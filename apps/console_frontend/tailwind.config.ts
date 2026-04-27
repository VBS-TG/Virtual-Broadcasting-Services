import type { Config } from 'tailwindcss'

const config: Config = {
  content: [
    './index.html',
    './src/**/*.{js,ts,jsx,tsx}',
  ],
  theme: {
    extend: {
      colors: {
        vbs: {
          carbon: '#0d0d10',
          dark: '#111116',
          navy: '#0a1628',
          blue: '#0d1f3e',
          mid: '#0f2a55',
          pgm: '#FF3B3B',
          pvw: '#10B981',
          accent: '#1E90FF',
          cyan: '#22D3EE',
          warning: '#F59E0B',
          text: '#E2E8F0',
          muted: '#64748B',
          dim: '#334155',
        },
      },
      fontFamily: {
        sans: ['Montserrat', 'ui-sans-serif', 'system-ui'],
        mono: ['Montserrat', 'ui-sans-serif', 'system-ui'],
      },
      boxShadow: {
        pgm: '0 0 20px rgba(255,59,59,0.5), 0 0 40px rgba(255,59,59,0.2)',
        pvw: '0 0 20px rgba(16,185,129,0.5), 0 0 40px rgba(16,185,129,0.2)',
        accent: '0 0 20px rgba(30,144,255,0.4)',
      },
      animation: {
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        'pulse-pgm': 'pulsePgm 1.5s ease-in-out infinite',
        'slide-in': 'slideIn 0.3s ease-out',
      },
      keyframes: {
        pulsePgm: {
          '0%, 100%': { opacity: '1', boxShadow: '0 0 20px rgba(255,59,59,0.6)' },
          '50%': { opacity: '0.85', boxShadow: '0 0 35px rgba(255,59,59,0.9)' },
        },
        slideIn: {
          '0%': { opacity: '0', transform: 'translateY(-8px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
      },
    },
  },
  plugins: [],
}

export default config
