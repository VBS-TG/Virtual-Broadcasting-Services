// vite.config.ts
import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');
  const isDev = mode === 'development';

  const RAW_TOKEN = env.VITE_DEV_AUTH_TOKEN || '';
  const VBS_TOKEN = (RAW_TOKEN.startsWith('Bearer ') ? RAW_TOKEN : `Bearer ${RAW_TOKEN}`)
    .replace(/[^\x20-\x7E]/g, '')
    .trim();

  return {
    plugins: [react()],
    server: isDev ? {
      proxy: {
        '/api': {
          target: env.VITE_API_BASE_URL || 'https://vbsapi.cyblisswisdom.org',
          changeOrigin: true,
          secure: true,
          headers: {
            'Authorization': VBS_TOKEN,
            // 💡 關鍵新增：加入 Cloudflare Service Token 穿透門禁
            'CF-Access-Client-Id': env.VITE_CF_ACCESS_CLIENT_ID || '',
            'CF-Access-Client-Secret': env.VITE_CF_ACCESS_CLIENT_SECRET || '',
          },
          configure: (proxy, _options) => {
            proxy.on('proxyReq', (proxyReq, req, _res) => {
              console.log('--- Vite Dev Proxy ---');
              console.log('Origin:', req.url);
              console.log('Target:', proxyReq.protocol + '//' + proxyReq.host + proxyReq.path);
            });
          },
        },
        // 💡 如果 RTC 也要透過 Proxy，請同步加入
        '/whep': {
          target: 'https://vbsrtc.cyblisswisdom.org',
          changeOrigin: true,
          secure: true,
          headers: {
            'CF-Access-Client-Id': env.VITE_CF_ACCESS_CLIENT_ID || '',
            'CF-Access-Client-Secret': env.VITE_CF_ACCESS_CLIENT_SECRET || '',
          }
        }
      },
    } : {},
  }
})