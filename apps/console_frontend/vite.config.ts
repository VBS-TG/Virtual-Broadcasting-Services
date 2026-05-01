import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

import { cloudflare } from "@cloudflare/vite-plugin";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const isDev = mode === 'development'

  return {
    plugins: [react(), cloudflare()],
    // Production build 不使用 Vite dev proxy，正式站由網頁反代處理 /api 與 /whep。
    server: isDev
      ? {
          proxy: {
            // Human control-plane API (JWT in X-VBS-Authorization).
            '/api': {
              target: 'https://vbsapi.cyblisswisdom.org',
              changeOrigin: true,
              secure: true,
            },
            // Machine/media path (Cloudflare Access service token).
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
  };
})