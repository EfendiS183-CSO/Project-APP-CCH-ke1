import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig, loadEnv } from 'vite'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, '.', '')
  // Laragon nginx occupies :8080, so the CCH Go backend runs on :8081.
  const backendTarget = env.VITE_CCH_BE_TARGET || 'http://127.0.0.1:8081'

  return {
    plugins: [react(), tailwindcss()],
    server: {
      proxy: {
        '/api/v1': {
          target: backendTarget,
          changeOrigin: true,
        },
        '/api/backend': {
          target: backendTarget,
          changeOrigin: true,
          rewrite: (path) => path.replace(/^\/api\/backend/, ''),
        },
        '/api/ai': {
          target: 'http://127.0.0.1:8000',
          changeOrigin: true,
          rewrite: (path) => path.replace(/^\/api\/ai/, ''),
        },
      },
    },
  }
})
