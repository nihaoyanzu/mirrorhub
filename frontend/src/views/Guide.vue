<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import copy from 'clipboard-copy'
import { api, getToken, type PublicGuide } from '@/api/client'
import { setLocale, type SupportedLocale } from '@/i18n'
import { guideFallbackModules, MODULE_BY_ID } from '@/modules/registry'
import { GUIDE_SECTIONS } from '@/modules/guide/sections'

const { t, locale } = useI18n()
const loading = ref(true)
const guide = ref<PublicGuide | null>(null)
const copied = ref('')
const activeTab = ref('')

const loggedIn = computed(() => !!getToken())

const enabledModules = computed(() => (guide.value?.modules || []).filter((m) => m.enabled))
const disabledModules = computed(() => (guide.value?.modules || []).filter((m) => !m.enabled))

const activeSection = computed(() => GUIDE_SECTIONS[activeTab.value] || null)

/** 用当前打开说明页的 Host + 下载端口（适配 Docker 端口映射）。 */
function accessDownloadURL(proxyPort: string): string {
  const port = String(proxyPort || '18081').trim() || '18081'
  if (typeof location === 'undefined') {
    return `http://127.0.0.1:${port}`
  }
  const scheme = location.protocol === 'https:' ? 'https' : 'http'
  let host = location.hostname || '127.0.0.1'
  if (host === '::1' || host === '[::1]') host = '127.0.0.1'
  const hostPart = host.includes(':') ? `[${host}]` : host
  return `${scheme}://${hostPart}:${port}`
}

const baseURL = computed(() => accessDownloadURL(guide.value?.proxy_port || '18081'))
const indexURL = computed(() => `${baseURL.value}/simple/`)
const registryURL = computed(() => `${baseURL.value}/`)
const displayHost = computed(() => baseURL.value.replace(/^https?:\/\//i, ''))

const trustedHost = computed(() => {
  try {
    return new URL(baseURL.value).hostname
  } catch {
    return 'localhost'
  }
})

function syncTab() {
  const ids = enabledModules.value.map((m) => m.id)
  if (!ids.length) {
    activeTab.value = ''
    return
  }
  if (!ids.includes(activeTab.value)) activeTab.value = ids[0]
}

watch(enabledModules, syncTab, { immediate: true })

function toggleLang() {
  const next: SupportedLocale = locale.value === 'zh-CN' ? 'en' : 'zh-CN'
  setLocale(next)
}

async function copyText(key: string, text: string) {
  try {
    await copy(text)
    copied.value = key
    window.setTimeout(() => {
      if (copied.value === key) copied.value = ''
    }, 1500)
  } catch {
    /* ignore */
  }
}

function moduleTitle(id: string) {
  return MODULE_BY_ID[id]?.guideTitle || id
}

onMounted(async () => {
  loading.value = true
  try {
    guide.value = await api.getPublicGuide()
  } catch {
    guide.value = {
      proxy_port: '18081',
      modules: guideFallbackModules(),
    }
  } finally {
    syncTab()
    loading.value = false
  }
})
</script>

<template>
  <div class="relative min-h-screen overflow-hidden">
    <div class="pointer-events-none absolute inset-0">
      <div class="absolute -left-24 top-10 h-72 w-72 rounded-full bg-accent/10 blur-3xl" />
      <div class="absolute -right-20 bottom-0 h-80 w-80 rounded-full bg-[#1e3a4c]/35 blur-3xl" />
    </div>

    <div class="relative mx-auto max-w-3xl px-4 py-8 sm:px-6 sm:py-10">
      <header class="mb-8 flex flex-wrap items-end justify-between gap-4 border-b border-line pb-6">
        <div class="flex items-center gap-3">
          <img src="/favicon.svg" alt="MirrorHub" class="h-10 w-10" width="40" height="40" />
          <div>
            <div class="text-2xl font-semibold tracking-wide">
              mirror<span class="text-accent">hub</span>
            </div>
            <p class="mt-0.5 text-sm text-muted">{{ t('guide.tagline') }}</p>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <button type="button" class="ui-btn-ghost !py-1.5 text-xs" @click="toggleLang">
            {{ locale === 'zh-CN' ? 'EN' : '中文' }}
          </button>
          <RouterLink v-if="loggedIn" class="ui-btn" to="/dashboard">{{ t('nav.dashboard') }}</RouterLink>
          <RouterLink v-else class="ui-btn-primary" to="/login">{{ t('login.loginBtn') }}</RouterLink>
        </div>
      </header>

      <div v-if="loading" class="ui-panel p-6 text-sm text-muted">{{ t('common.loading') }}</div>

      <template v-else>
        <div class="mb-4">
          <div class="ui-label">{{ t('guide.addresses') }}</div>
          <div class="ui-input font-mono text-sm text-muted">{{ displayHost }}</div>
          <p class="mt-2 text-xs text-muted">{{ t('guide.addressesHint') }}</p>
        </div>

        <div v-if="!enabledModules.length" class="ui-panel p-5 text-sm text-muted">
          {{ t('guide.noModule') }}
        </div>

        <template v-else>
          <div class="mb-4 flex flex-wrap gap-1 rounded-xl border border-line bg-panel/60 p-1">
            <button
              v-for="m in enabledModules"
              :key="m.id"
              type="button"
              class="ui-tab"
              :class="{ 'ui-tab-active': activeTab === m.id }"
              @click="activeTab = m.id"
            >
              {{ moduleTitle(m.id) }}
            </button>
          </div>

          <component
            :is="activeSection"
            v-if="activeSection"
            :base-url="baseURL"
            :index-url="indexURL"
            :registry-url="registryURL"
            :display-host="displayHost"
            :trusted-host="trustedHost"
            :copied="copied"
            @copy="copyText"
          />
        </template>

        <p v-if="disabledModules.length" class="mt-4 text-xs text-muted">
          {{ t('guide.disabled') }}：
          <span v-for="(m, i) in disabledModules" :key="m.id">
            {{ moduleTitle(m.id) }}<span v-if="i < disabledModules.length - 1"> · </span>
          </span>
        </p>
      </template>
    </div>
  </div>
</template>
