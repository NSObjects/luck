import {fileURLToPath, URL} from 'node:url'

import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    vueDevTools(),
  ],
    base: '/',                 // 生产部署在根路径（配合 go:embed）
    build: {
        outDir: '../backend/web/dist',      // 打包到后端工程里的 web/dist
        assetsDir: 'assets',     // 生成 /assets/xxx 静态资源
        sourcemap: false,
        emptyOutDir: true,
        target: 'es2017',
    },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080'
    }
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    },
  },
})
