// vite.config.ts
import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig(({ mode }) => {
  // 1. 載入 .env.local 裡面的變數
  const env = loadEnv(mode, process.cwd(), '');
  
  // 2. 清理 Token：確保沒有任何不可見字元導致 Node 崩潰
  const RAW_TOKEN = env.VITE_DEV_AUTH_TOKEN || '';
  const VBS_TOKEN = (RAW_TOKEN.startsWith('Bearer ') ? RAW_TOKEN : `Bearer ${RAW_TOKEN}`)
    .replace(/[^\x20-\x7E]/g, '')
    .trim();

  return {
    plugins: [react()],
    server: {
      proxy: {
        // 修改重點：直接攔截 /api，這樣 /api/v1/... 才會被抓到
        '/api': {
          target: 'http://localhost:5000',
          changeOrigin: true,
          secure: false,
          headers: {
            'Authorization': VBS_TOKEN
          },
          // 根據你的後端路由決定：
          // 如果後端本來就期待 /api/v1...，則不需要 rewrite。
          // 如果後端期待的是 /v1...，請取消下面這行的註解：
          // rewrite: (path) => path.replace(/^\/api/, '')
          
          // 加一個調試小工具，讓你在終端機看到請求轉發到哪了
          configure: (proxy, _options) => {
            proxy.on('proxyReq', (proxyReq, req, _res) => {
              console.log('--- Vite Proxy 轉發中 ---');
              console.log('原始路徑:', req.url);
              console.log('目標地址:', proxyReq.protocol + '//' + proxyReq.host + proxyReq.path);
            });
          },
        },
      },
    },
  }
})