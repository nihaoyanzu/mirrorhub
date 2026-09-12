import { createI18n } from 'vue-i18n'
import zhCN from './zh-CN'
import en from './en'

const STORAGE_KEY = 'mirrorhub_lang'

function detectLocale(): string {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved === 'zh-CN' || saved === 'en') return saved
  } catch {}
  return navigator.language.startsWith('zh') ? 'zh-CN' : 'en'
}

export const i18n = createI18n({
  legacy: false,
  locale: detectLocale(),
  fallbackLocale: 'en',
  messages: { 'zh-CN': zhCN, en },
})

export function setLocale(locale: 'zh-CN' | 'en') {
  ;(i18n.global.locale as any).value = locale
  try {
    localStorage.setItem(STORAGE_KEY, locale)
  } catch {}
}

export type SupportedLocale = 'zh-CN' | 'en'
