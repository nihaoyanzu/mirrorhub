import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { i18n } from './i18n'
import { useThemeStore } from './stores/theme'
import './styles/main.css'

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.use(i18n)

// 等首屏路由解析完成再挂载，避免 / 等公开页短暂落到 AppShell 触发 /me→401→跳登录
router.isReady().then(() => {
  app.mount('#app')
  useThemeStore().init()
})
