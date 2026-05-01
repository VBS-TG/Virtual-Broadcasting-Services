import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

import { cloudflare } from "@cloudflare/vite-plugin";

export default defineConfig({
  // Release-only behavior: frontend always calls same-origin /api and /whep.
  // Routing/auth forwarding is handled by deployed reverse-proxy/BFF layer.
  plugins: [react(), cloudflare()],
})