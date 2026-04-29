// vite.config.ts
import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');
  const isDev = mode === 'development'; // 判斷是否為開發模式

  const RAW_TOKEN = env.VITE_DEV_AUTH_TOKEN || '';
  const VBS_TOKEN = (RAW_TOKEN.startsWith('Bearer ') ? RAW_TOKEN : `Bearer ${RAW_TOKEN}`)
    .replace(/[^\x20-\x7E]/g, '')
    .trim();

  return {
    plugins: [react()],
    // 只有在開發環境才需要 server 設定
    server: isDev ? {
      proxy: {
        '/api': {
          target: env.VITE_API_BASE_URL || 'https://vbsapi.cyblisswisdom.org',
          changeOrigin: true,
          secure: true,
          headers: {
            'Authorization': VBS_TOKEN
          },
          // 如果後端路徑沒有 /api，請取消下一行註解
          // rewrite: (path) => path.replace(/^\/api/, ''),
          configure: (proxy, _options) => {
            proxy.on('proxyReq', (proxyReq, req, _res) => {
              console.log('--- Vite Dev Proxy ---');
              console.log('Origin:', req.url);
              console.log('Target:', proxyReq.protocol + '//' + proxyReq.host + proxyReq.path);
            });
          },
        },
      },
    } : {}, // 正式環境 server 區塊為空
  }
})