import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 8080,
    proxy: {
      '/api': {
        target: 'http://localhost:8001',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:8002',
        ws: true,
        changeOrigin: true,
      },
      '/crush-images': {
        target: 'http://localhost:9000',
        changeOrigin: true,
      }
    }
  }
})
