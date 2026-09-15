<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import copy from 'clipboard-copy'
import { api, getToken, type PublicGuide } from '@/api/client'
import { setLocale, type SupportedLocale } from '@/i18n'

const { t, locale } = useI18n()
const loading = ref(true)
const guide = ref<PublicGuide | null>(null)
const copied = ref('')
const activeTab = ref('')
const toolTab = ref<'pip' | 'uv'>('pip')

const loggedIn = computed(() => !!getToken())

const enabledModules = computed(() => (guide.value?.modules || []).filter((m) => m.enabled))
const disabledModules = computed(() => (guide.value?.modules || []).filter((m) => !m.enabled))

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
const displayHost = computed(() => baseURL.value.replace(/^https?:\/\//i, ''))

const trustedHost = computed(() => {
  try {
    return new URL(baseURL.value).hostname
  } catch {
    return 'localhost'
  }
})

const pipSnippets = computed(() => [
  {
    key: 'pipOnce',
    title: t('guide.pipOnce'),
    text: `pip install <pkg> -i ${indexURL.value} --trusted-host ${trustedHost.value}`,
  },
  {
    key: 'pipConfig',
    title: t('guide.pipConfig'),
    text: `pip config set global.index-url ${indexURL.value}\npip config set global.trusted-host ${trustedHost.value}`,
  },
])

const uvSnippets = computed(() => [
  {
    key: 'uvOnce',
    title: t('guide.uvOnce'),
    text: `uv pip install <pkg> -i ${indexURL.value}`,
  },
  {
    key: 'uvConfig',
    title: t('guide.uvConfig'),
    text: `# Linux / macOS: ~/.config/uv/uv.toml\n# Windows: %APPDATA%\\uv\\uv.toml\n\n[[index]]\nurl = "${indexURL.value}"\ndefault = true\n\nallow-insecure-host = ["${trustedHost.value}"]`,
  },
  {
    key: 'uvEnv',
    title: t('guide.uvEnv'),
    text: `# Linux / macOS\nexport UV_DEFAULT_INDEX=${indexURL.value}\n\n# Windows PowerShell\n$env:UV_DEFAULT_INDEX="${indexURL.value}"`,
  },
])

const activeSnippets = computed(() => (toolTab.value === 'pip' ? pipSnippets.value : uvSnippets.value))

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
  if (id === 'pypi') return 'PyPI'
  return id
}

onMounted(async () => {
  loading.value = true
  try {
    guide.value = await api.getPublicGuide()
  } catch {
    guide.value = {
      proxy_port: '18081',
      modules: [{ id: 'pypi', enabled: true }],
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

          <section v-if="activeTab === 'pypi'" class="ui-panel overflow-hidden">
            <div class="flex flex-wrap items-center justify-between gap-3 border-b border-line p-4 sm:p-5">
              <p class="text-xs text-muted">{{ t('guide.pypiHint') }}</p>
              <div class="flex gap-1 rounded-xl border border-line bg-bg/60 p-1">
                <button
                  type="button"
                  class="ui-chip"
                  :class="{ 'ui-chip-active': toolTab === 'pip' }"
                  @click="toolTab = 'pip'"
                >
                  pip
                </button>
                <button
                  type="button"
                  class="ui-chip"
                  :class="{ 'ui-chip-active': toolTab === 'uv' }"
                  @click="toolTab = 'uv'"
                >
                  uv
                </button>
              </div>
            </div>

            <div class="grid gap-3 p-4 sm:p-5">
              <article
                v-for="item in activeSnippets"
                :key="item.key"
                class="rounded-xl border border-line bg-bg/40 p-3 sm:p-4"
              >
                <div class="mb-2 flex items-center justify-between gap-2">
                  <h3 class="text-sm font-medium text-fg">{{ item.title }}</h3>
                  <button
                    type="button"
                    class="ui-btn-ghost !px-2 !py-1 text-xs"
                    @click="copyText(item.key, item.text)"
                  >
                    {{ copied === item.key ? t('guide.copied') : t('guide.copy') }}
                  </button>
                </div>
                <pre
                  class="overflow-x-auto whitespace-pre-wrap break-all font-mono text-xs leading-relaxed text-muted"
                >{{ item.text }}</pre>
              </article>
            </div>
          </section>
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
