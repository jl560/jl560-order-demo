import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// 浏览器打开的是 :5173，Go API 在 :8080。
// 相对路径 fetch("/users") 会打到 5173，Vite 再转发到 Go。
// 这样开发时看起来仍像 F2 的同源请求。CORS 的课放到 F6。
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/users': 'http://127.0.0.1:8080',
      '/ping': 'http://127.0.0.1:8080',
    },
  },
})
