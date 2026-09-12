import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'

// 开发期用 Vite 代理替代 Nginx：
// - 管理台 API / metrics → admin :18082
// - PyPI 数据面 /simple、/packages/* → proxy :18081（勿代理裸 /packages，留给 SPA 包检索页）
export default defineConfig({
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:18082',
        changeOrigin: true,
      },
      '/metrics': {
        target: 'http://127.0.0.1:18082',
        changeOrigin: true,
      },
      '/health': {
        target: 'http://127.0.0.1:18082',
        changeOrigin: true,
      },
      '/simple': {
        target: 'http://127.0.0.1:18081',
        changeOrigin: true,
      },
      // 发行文件走数据面；裸 /packages 留给 SPA 包检索页
      '/packages': {
        target: 'http://127.0.0.1:18081',
        changeOrigin: true,
        bypass(req) {
          const path = (req.url || '').split('?')[0]
          if (path === '/packages' || path === '/packages/') {
            return path
          }
        },
      },
    },
  },
})
