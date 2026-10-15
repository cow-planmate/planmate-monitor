import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// 개발 중에는 화면을 Vite(5173)가, API 는 Go 서버(8080)가 맡는다. 운영에서는 둘이 같은 주소라
// 코드는 항상 /api 상대경로만 쓴다.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    proxy: { '/api': 'http://localhost:8080' },
  },
})
