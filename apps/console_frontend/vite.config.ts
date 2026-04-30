import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const isDev = mode === 'development'

  return {
    plugins: [react()],
    // Production build 不使用 Vite dev proxy，正式站由網頁反代處理 /api 與 /whep。
    server: isDev
      ? {
          proxy: {
            '/api': {
              target: 'https://vbsapi.cyblisswisdom.org',
              changeOrigin: true,
              secure: true,
            },
            '/whep': {
              target: 'https://vbsrtc.cyblisswisdom.org',
              changeOrigin: true,
              secure: true,
              headers: {
                'CF-Access-Client-Id': env.VITE_CF_ACCESS_CLIENT_ID || '',
                'CF-Access-Client-Secret': env.VITE_CF_ACCESS_CLIENT_SECRET || '',
              },
            },
          },
        }
      : undefined,
  }
})