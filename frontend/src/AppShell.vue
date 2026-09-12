<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useSessionStore } from '@/stores/session'
import { useToastStore } from '@/stores/toast'
import { useThemeStore } from '@/stores/theme'
import { useStatsStore } from '@/stores/stats'
import { setLocale, type SupportedLocale } from '@/i18n'
import { api } from '@/api/client'
import { fmtBytes, fmtRate } from '@/lib/format'

const { t, locale } = useI18n()
const router = useRouter()
const session = useSessionStore()
const toast = useToastStore()
const theme = useThemeStore()
const statsStore = useStatsStore()

const sidebarOpen = ref(false)
const userMenuOpen = ref(false)

const navGroups = computed(() => [
  {
    id: 'overview',
    label: t('nav.overview'),
    highlight: false,
    links: [
      { to: '/', icon: 'dash', text: t('nav.dashboard') },
      { to: '/access', icon: 'access', text: t('nav.access') },
    ],
  },
  {
    id: 'pypi',
    label: t('nav.pypi'),
    highlight: true,
    links: [
      { to: '/packages', icon: 'pkg', text: t('nav.packages') },
      { to: '/prefetch', icon: 'pf', text: t('nav.prefetch') },
      { to: '/queue', icon: 'queue', text: t('nav.queue') },
      { to: '/platform', icon: 'plat', text: t('nav.platform') },
    ],
  },
  {
    id: 'system',
    label: t('nav.system'),
    highlight: false,
    links: [{ to: '/settings', icon: 'sys', text: t('nav.settings') }],
  },
])

const stats = computed(() => statsStore.data)
const pullBps = computed(() => Number(stats.value?.traffic?.upstream_bps ?? 0))
const shareBps = computed(() => Number(stats.value?.traffic?.downstream_bps ?? 0))
const pullTotal = computed(() => Number(stats.value?.traffic?.upstream_total ?? 0))
const shareTotal = computed(() => Number(stats.value?.traffic?.downstream_total ?? 0))
const p0Active = computed(() => Number(stats.value?.interactive_active ?? 0))
const p1Active = computed(() => Number(stats.value?.resume_count ?? 0))
const cacheSize = computed(() => Number(stats.value?.cache?.total_size ?? 0))

const healthOk = ref(true)
let healthTimer: number | undefined

async function checkHealth() {
  try {
    const res = await fetch('/health')
    healthOk.value = res.ok
  } catch {
    healthOk.value = false
  }
}

onMounted(() => {
  session.refresh()
  statsStore.startPolling(3000)
  checkHealth()
  healthTimer = window.setInterval(checkHealth, 15000)
  document.addEventListener('click', onDocClick)
})

onUnmounted(() => {
  statsStore.stopPolling()
  if (healthTimer) clearInterval(healthTimer)
  document.removeEventListener('click', onDocClick)
})

const userMenuRef = ref<HTMLElement | null>(null)

function toggleLang() {
  const next: SupportedLocale = locale.value === 'zh-CN' ? 'en' : 'zh-CN'
  setLocale(next)
}

function closeSidebar() {
  sidebarOpen.value = false
}

function toggleUserMenu() {
  userMenuOpen.value = !userMenuOpen.value
}

function closeUserMenu() {
  userMenuOpen.value = false
}

function onDocClick(e: MouseEvent) {
  const el = userMenuRef.value
  if (!el) return
  if (!el.contains(e.target as Node)) closeUserMenu()
}

async function logout() {
  closeUserMenu()
  try {
    await api.logout()
  } catch {
    /* 本地清会话即可 */
  }
  session.reset()
  toast.info(t('toast.loggedOut'))
  router.replace('/login')
}

function goAccount() {
  closeUserMenu()
  router.push({ path: '/settings', query: { tab: 'account' } })
}
</script>

<template>
  <Transition name="fade">
    <div
      v-if="sidebarOpen"
      class="fixed inset-0 z-40 bg-black/50 backdrop-blur-sm lg:hidden"
      @click="closeSidebar"
    />
  </Transition>

  <aside
    class="fixed inset-y-0 left-0 z-50 flex w-[288px] flex-col border-r border-line bg-panel/95 backdrop-blur-md
      transition-transform duration-200 lg:translate-x-0"
    :class="sidebarOpen ? 'translate-x-0' : '-translate-x-full'"
  >
    <div class="flex h-16 shrink-0 items-center gap-2.5 border-b border-line px-5">
      <img src="/favicon.svg" alt="" class="h-7 w-7 shrink-0" width="28" height="28" />
      <div class="text-xl font-semibold tracking-wide">
        mirror<span class="text-accent">hub</span>
      </div>
    </div>

    <nav class="flex-1 overflow-y-auto px-3 py-5">
      <div
        v-for="g in navGroups"
        :key="g.id"
        class="mb-5"
        :class="{ 'nav-group-pypi': g.highlight }"
      >
        <p
          class="nav-group-label mb-2.5 px-3 text-xs font-medium uppercase tracking-widest text-muted/60"
          :class="{ 'pt-1': g.highlight }"
        >
          {{ g.label }}
        </p>
        <router-link
          v-for="link in g.links"
          :key="link.to"
          :to="link.to"
          class="nav-link mb-0.5 gap-3"
          @click="closeSidebar"
        >
          <svg class="h-5 w-5 shrink-0 opacity-80" viewBox="0 0 20 20" fill="currentColor">
            <path v-if="link.icon === 'dash'" d="M10 2a8 8 0 100 16 8 8 0 000-16zm0 2c-.56 0-1.1.1-1.6.28a1 1 0 00.32 1.96A4 4 0 0110 6zm-4.32.68a1 1 0 00-1.4 0A6 6 0 006 12a1 1 0 001.96-.32A4 4 0 015.68 6.68zm8.64 0a4 4 0 01-2.32 5A1 1 0 0013.4 12a6 6 0 00.92-5.32z" />
            <path v-else-if="link.icon === 'access'" d="M3 4a1 1 0 011-1h12a1 1 0 011 1v2H3V4zm0 4h14v8a1 1 0 01-1 1H4a1 1 0 01-1-1V8zm3 2a1 1 0 100 2h4a1 1 0 100-2H6z" />
            <path v-else-if="link.icon === 'queue'" d="M3 4a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V4zm0 5a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V9zm0 5a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1v-2z" />
            <path v-else-if="link.icon === 'pkg'" d="M10 2l7 4v8l-7 4-7-4V6l7-4zm0 2.2L5 7.5v5l5 3 5-3v-5l-5-3.3z" />
            <path v-else-if="link.icon === 'pf'" d="M4 4a2 2 0 012-2h8a2 2 0 012 2v12l-6-3-6 3V4z" />
            <path v-else-if="link.icon === 'plat'" d="M10 2a8 8 0 100 16 8 8 0 000-16zm-1 4h2v4l3 2-1 1.5L9 11V6z" />
            <path v-else-if="link.icon === 'sys'" d="M5 4a2 2 0 012-2h6a2 2 0 012 2v1h1a1 1 0 011 1v2a1 1 0 01-1 1h-1v6a2 2 0 01-2 2H7a2 2 0 01-2-2v-6H4a1 1 0 01-1-1V6a1 1 0 011-1h1V4zm2 0v1h6V4H7zm0 10h6v-6H7v6z" />
            <circle v-else cx="10" cy="10" r="3" />
          </svg>
          {{ link.text }}
        </router-link>
      </div>
    </nav>

    <div class="shrink-0 border-t border-line p-4 text-sm text-muted">
      <div class="flex items-center gap-2">
        <span class="relative flex h-2 w-2">
          <span
            class="absolute inline-flex h-full w-full rounded-full opacity-75"
            :class="healthOk ? 'bg-ok animate-ping' : 'bg-danger'"
          />
          <span
            class="relative inline-flex h-2 w-2 rounded-full"
            :class="healthOk ? 'bg-ok' : 'bg-danger'"
          />
        </span>
        <span>{{ healthOk ? t('common.connected') : t('common.unreachable') }}</span>
      </div>
    </div>
  </aside>

  <div class="flex min-h-screen flex-col lg:pl-[288px]">
    <header
      class="sticky top-0 z-30 flex h-16 shrink-0 items-center gap-3 border-b border-line bg-panel/80
        px-4 backdrop-blur-md sm:px-6"
    >
      <button
        type="button"
        class="ui-btn-ghost !p-2 lg:hidden"
        aria-label="Menu"
        @click="sidebarOpen = !sidebarOpen"
      >
        <svg class="h-6 w-6" viewBox="0 0 20 20" fill="currentColor">
          <path d="M3 5h14M3 10h14M3 15h14" stroke="currentColor" stroke-width="1.5" fill="none" stroke-linecap="round" />
        </svg>
      </button>

      <div class="hidden min-w-0 items-center gap-3.5 text-sm text-muted lg:flex">
        <span class="flex items-center gap-1.5">
          <svg class="h-4 w-4 text-accent" viewBox="0 0 20 20" fill="currentColor">
            <path d="M10 3a1 1 0 011 1v7.586l2.293-2.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L10 11.586V4a1 1 0 011-1z" />
            <path d="M4 15a1 1 0 011-1h10a1 1 0 110 2H5a1 1 0 01-1-1z" />
          </svg>
          <strong class="font-mono text-fg">{{ fmtRate(pullBps) }}</strong>
          <span class="text-muted/60">({{ fmtBytes(pullTotal) }})</span>
        </span>
        <span class="flex items-center gap-1.5">
          <svg class="h-4 w-4 text-ok" viewBox="0 0 20 20" fill="currentColor">
            <path d="M10 17a1 1 0 01-1-1V9.414l-2.293 2.293a1 1 0 01-1.414-1.414l4-4a1 1 0 011.414 0l4 4a1 1 0 01-1.414 1.414L10 9.414V16a1 1 0 01-1 1z" />
            <path d="M4 5a1 1 0 011-1h10a1 1 0 110 2H5a1 1 0 01-1-1z" />
          </svg>
          <strong class="font-mono text-fg">{{ fmtRate(shareBps) }}</strong>
          <span class="text-muted/60">({{ fmtBytes(shareTotal) }})</span>
        </span>
        <span class="flex items-center gap-1.5">
          <svg class="h-4 w-4 text-muted" viewBox="0 0 20 20" fill="currentColor">
            <path d="M4 4a2 2 0 012-2h8a2 2 0 012 2v12a2 2 0 01-2 2H6a2 2 0 01-2-2V4zm2 1v10h8V5H6zm2 2h4v1.5H8V7zm0 3h4V11.5H8V10z" />
          </svg>
          <strong class="font-mono text-fg">{{ fmtBytes(cacheSize) }}</strong>
        </span>
        <span class="h-3.5 w-px bg-line" />
        <span class="font-mono text-accent">P0 {{ p0Active }}</span>
        <span class="font-mono" :class="p1Active ? 'text-ok' : 'text-muted'">P1 {{ p1Active }}</span>
      </div>

      <div class="ml-auto flex items-center gap-1">
        <button
          type="button"
          class="ui-btn-ghost !px-2 !py-1 text-xs"
          @click="theme.toggle()"
        >
          <svg v-if="theme.isDark" class="h-4 w-4" viewBox="0 0 20 20" fill="currentColor">
            <path d="M10 2a1 1 0 011 1v1a1 1 0 01-2 0V3a1 1 0 011-1zm4.22 1.78a1 1 0 011.41 0l.71.71a1 1 0 01-1.41 1.41l-.71-.71a1 1 0 010-1.41zM18 9a1 1 0 010 2h-1a1 1 0 010-2h1zm-2.78 6.22a1 1 0 00-1.41 0l-.71.71a1 1 0 001.41 1.41l.71-.71a1 1 0 000-1.41zM10 16a1 1 0 011 1v1a1 1 0 01-2 0v-1a1 1 0 011-1zm-5.22-1.78a1 1 0 000-1.41l-.71-.71a1 1 0 00-1.41 1.41l.71.71a1 1 0 001.41 0zM4 11a1 1 0 010-2H3a1 1 0 010 2h1zm2.78-6.22a1 1 0 001.41 0l.71-.71A1 1 0 007.49 2.66l-.71.71a1 1 0 000 1.41zM10 7a3 3 0 100 6 3 3 0 000-6z" />
          </svg>
          <svg v-else class="h-4 w-4" viewBox="0 0 20 20" fill="currentColor">
            <path d="M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z" />
          </svg>
        </button>

        <button
          type="button"
          class="ui-btn-ghost !px-2 !py-1 text-xs font-medium"
          @click="toggleLang"
        >
          {{ locale === 'zh-CN' ? 'EN' : '中' }}
        </button>

        <div ref="userMenuRef" class="relative border-l border-line pl-2">
          <button
            type="button"
            class="ui-btn-ghost flex items-center gap-1.5 !px-2.5 !py-1.5 text-xs"
            :aria-expanded="userMenuOpen"
            @click.stop="toggleUserMenu"
          >
            <span class="max-w-[7rem] truncate font-medium text-fg">
              {{ session.username || t('common.notLoggedIn') }}
            </span>
            <svg
              class="h-3 w-3 shrink-0 text-muted transition-transform"
              :class="{ 'rotate-180': userMenuOpen }"
              viewBox="0 0 20 20"
              fill="currentColor"
            >
              <path fill-rule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clip-rule="evenodd" />
            </svg>
          </button>
          <div
            v-show="userMenuOpen"
            class="user-menu absolute right-0 top-full z-50 mt-1.5 min-w-[10rem] overflow-hidden rounded-lg border border-line bg-panel py-1 shadow-lg"
          >
            <button
              type="button"
              class="user-menu-item"
              @click="goAccount"
            >
              {{ t('nav.account') }}
            </button>
            <button
              type="button"
              class="user-menu-item"
              @click="logout"
            >
              {{ t('common.logout') }}
            </button>
          </div>
        </div>
      </div>
    </header>

    <main class="mx-auto w-full max-w-screen-2xl flex-1 px-5 py-8 sm:px-8">
      <router-view />
    </main>
  </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.15s ease;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
