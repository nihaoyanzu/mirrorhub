<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { api } from '@/api/client'
import { useSessionStore } from '@/stores/session'
import { useToastStore } from '@/stores/toast'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const session = useSessionStore()
const toast = useToastStore()
const loading = ref(false)
const saving = ref(false)
const dirty = ref(false)
const pwdSaving = ref(false)
const tab = ref<'network' | 'rate' | 'concurrency' | 'account'>('network')

const form = reactive({
  upstream_proxy: '',
  bandwidth_mbps: 0,
  max_concurrent: 20,
  max_connections: 80,
  windows: [] as { start: string; end: string; bandwidth_mbps: number }[],
  idle_quota_ratio: 0.3,
  on_interactive: 'pause',
  resume_on_idle: true,
  small_file_boost_enabled: true,
  small_file_boost_kb: 512,
  // 预取制品字段：系统页不编辑，保存时原样带回，避免冲掉模块设置
  artifact_mode: 'portable',
  extra_wheel_tags: [] as string[],
  target_python: [] as string[],
  target_platform: 'linux',
  max_depth: 5,
  max_packages: 200,
})

type RateWindowRow = { start: string; end: string; bandwidth_mbps: number }

const pwd = reactive({
  old_password: '',
  new_password: '',
  confirm: '',
})

const tabs = computed(() => [
  { id: 'network' as const, label: t('system.tabNetwork') },
  { id: 'rate' as const, label: t('system.tabRate') },
  { id: 'concurrency' as const, label: t('system.tabConcurrency') },
  { id: 'account' as const, label: t('system.tabAccount') },
])

function syncTabFromRoute() {
  const q = String(route.query.tab || '')
  if (q === 'network' || q === 'rate' || q === 'concurrency' || q === 'account') tab.value = q
}

function setTab(id: typeof tab.value) {
  tab.value = id
  router.replace({ query: { ...route.query, tab: id } })
}

watch(() => route.query.tab, syncTabFromRoute)

function markDirty() {
  dirty.value = true
}

function padHM(hm: string) {
  const m = String(hm || '').trim().match(/^(\d{1,2}):(\d{2})/)
  if (!m) return '00:00'
  return `${String(Number(m[1])).padStart(2, '0')}:${m[2]}`
}

function normalizeWindows(wins: RateWindowRow[]) {
  return wins
    .map((w) => ({
      start: padHM(w.start),
      end: padHM(w.end),
      bandwidth_mbps: Math.max(0, Number(w.bandwidth_mbps) || 0),
    }))
    .filter((w) => w.start && w.end)
}

function addWindow() {
  form.windows.push({ start: '09:00', end: '18:00', bandwidth_mbps: 20 })
  markDirty()
}

function removeWindow(i: number) {
  form.windows.splice(i, 1)
  markDirty()
}

function applyPreset(kind: 'off' | 'always' | 'work_limit' | 'offwork_limit') {
  if (kind === 'off') {
    form.bandwidth_mbps = 0
    form.windows = []
  } else if (kind === 'always') {
    form.bandwidth_mbps = form.bandwidth_mbps > 0 ? form.bandwidth_mbps : 20
    form.windows = []
  } else if (kind === 'work_limit') {
    form.bandwidth_mbps = 0
    form.windows = [{ start: '09:00', end: '18:00', bandwidth_mbps: 20 }]
  } else {
    form.bandwidth_mbps = form.bandwidth_mbps > 0 ? form.bandwidth_mbps : 20
    form.windows = [{ start: '09:00', end: '18:00', bandwidth_mbps: 0 }]
  }
  markDirty()
}

async function load() {
  loading.value = true
  try {
    const cfg = await api.getConfig()
    form.upstream_proxy = cfg.server?.upstream_proxy || ''
    const rl = cfg.rate_limit
    form.bandwidth_mbps = rl?.bandwidth_mbps ?? 0
    form.max_concurrent = rl?.max_concurrent ?? 20
    form.max_connections = rl?.max_connections ?? 80
    form.windows = normalizeWindows(
      (rl?.windows || []).map((w) => ({
        start: w.start,
        end: w.end,
        bandwidth_mbps: w.bandwidth_mbps ?? 0,
      })),
    )
    const sch = cfg.scheduler
    if (sch) {
      form.idle_quota_ratio = sch.prefetch?.idle_quota_ratio ?? 0.3
      form.on_interactive = sch.prefetch?.on_interactive || 'pause'
      form.resume_on_idle = sch.prefetch?.resume_on_idle !== false
      form.small_file_boost_enabled = sch.small_file_boost?.enabled !== false
      form.small_file_boost_kb = sch.small_file_boost?.max_size_kb ?? 512
      form.artifact_mode = sch.prefetch?.artifact_mode || 'portable'
      form.extra_wheel_tags = sch.prefetch?.extra_wheel_tags || []
      form.target_python = Array.isArray(sch.prefetch?.target_python)
        ? sch.prefetch.target_python
        : []
      form.target_platform = sch.prefetch?.target_platform || 'linux'
      form.max_depth = sch.prefetch?.max_depth ?? 5
      form.max_packages = sch.prefetch?.max_packages ?? 200
    }
    dirty.value = false
  } catch (e: any) {
    toast.err(e.message || t('system.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    if (tab.value === 'network') {
      await api.putConfig({
        upstream_proxy: form.upstream_proxy,
      })
    } else if (tab.value === 'rate' || tab.value === 'concurrency') {
      // 两页共享 rate_limit / scheduler，保存时整包带回，避免互相冲掉
      await api.putConfig({
        rate_limit: {
          bandwidth_mbps: form.bandwidth_mbps,
          max_concurrent: form.max_concurrent,
          max_connections: form.max_connections,
          windows: normalizeWindows(form.windows),
        },
        scheduler: {
          prefetch: {
            idle_quota_ratio: form.idle_quota_ratio,
            on_interactive: form.on_interactive,
            resume_on_idle: form.resume_on_idle,
            artifact_mode: form.artifact_mode,
            extra_wheel_tags: form.extra_wheel_tags,
            target_python: form.target_python,
            target_platform: form.target_platform,
            max_depth: form.max_depth,
            max_packages: form.max_packages,
          },
          small_file_boost: {
            enabled: form.small_file_boost_enabled,
            max_size_kb: form.small_file_boost_kb,
          },
        },
      })
    }
    dirty.value = false
    toast.ok(t('system.saved'))
  } catch (e: any) {
    toast.err(e.message || t('system.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function changePassword() {
  if (!pwd.old_password || !pwd.new_password) {
    toast.err(t('system.pwdRequired'))
    return
  }
  if (pwd.new_password !== pwd.confirm) {
    toast.err(t('system.pwdMismatch'))
    return
  }
  pwdSaving.value = true
  try {
    await api.changePassword(pwd.old_password, pwd.new_password)
    pwd.old_password = ''
    pwd.new_password = ''
    pwd.confirm = ''
    toast.ok(t('system.pwdChanged'))
  } catch (e: any) {
    toast.err(e.message || t('system.pwdFailed'))
  } finally {
    pwdSaving.value = false
  }
}

async function logout() {
  try {
    await api.logout()
  } catch {
    /* ignore */
  }
  session.reset()
  toast.info(t('toast.loggedOut'))
  await router.replace('/login')
}

onMounted(() => {
  syncTabFromRoute()
  load()
})
</script>

<template>
  <div>
    <div class="mb-4 flex flex-wrap items-center gap-3">
      <div class="flex min-w-0 flex-1 flex-wrap gap-1 rounded-xl border border-line bg-panel/60 p-1">
        <button
          v-for="item in tabs"
          :key="item.id"
          type="button"
          class="ui-tab"
          :class="{ 'ui-tab-active': tab === item.id }"
          @click="setTab(item.id)"
        >
          {{ item.label }}
        </button>
      </div>
      <div class="flex shrink-0 flex-wrap items-center gap-2">
        <template v-if="tab === 'network' || tab === 'rate' || tab === 'concurrency'">
          <span v-if="dirty" class="ui-badge-warn">{{ t('common.unsaved') }}</span>
          <button class="ui-btn" :disabled="loading || saving" @click="load">{{ t('system.reload') }}</button>
          <button class="ui-btn-primary" :disabled="loading || saving" @click="save">
            {{ saving ? t('system.saving') : t('common.save') }}
          </button>
        </template>
        <template v-else>
          <button type="button" class="ui-btn" @click="logout">{{ t('common.logout') }}</button>
          <button
            type="button"
            class="ui-btn-primary"
            :disabled="pwdSaving"
            @click="changePassword"
          >
            {{ pwdSaving ? t('system.pwdSaving') : t('system.changePassword') }}
          </button>
        </template>
      </div>
    </div>

    <section
      v-if="tab === 'network'"
      class="ui-panel space-y-5 p-5"
      @input="markDirty"
      @change="markDirty"
    >
      <form class="grid gap-4 sm:grid-cols-2" autocomplete="off" @submit.prevent>
        <div class="sm:col-span-2">
          <label class="ui-label">{{ t('system.upstreamProxy') }}</label>
          <input
            v-model="form.upstream_proxy"
            class="ui-input"
            name="mirrorhub-upstream-proxy"
            type="text"
            inputmode="url"
            autocomplete="off"
            data-1p-ignore
            data-lpignore="true"
            placeholder="socks5://127.0.0.1:1080"
          />
        </div>
      </form>
    </section>

    <section
      v-else-if="tab === 'rate'"
      class="ui-panel space-y-6 p-5"
      @input="markDirty"
      @change="markDirty"
    >
      <div class="flex flex-wrap items-center gap-2">
        <span class="shrink-0 text-xs font-medium text-muted">{{ t('system.presets') }}</span>
        <button type="button" class="ui-btn-ghost !py-1 text-xs" @click="applyPreset('off')">
          {{ t('system.presetOff') }}
        </button>
        <button type="button" class="ui-btn-ghost !py-1 text-xs" @click="applyPreset('always')">
          {{ t('system.presetAlways') }}
        </button>
        <button type="button" class="ui-btn-ghost !py-1 text-xs" @click="applyPreset('work_limit')">
          {{ t('system.presetWorkLimit') }}
        </button>
        <button type="button" class="ui-btn-ghost !py-1 text-xs" @click="applyPreset('offwork_limit')">
          {{ t('system.presetOffworkLimit') }}
        </button>
      </div>

      <div>
        <h3 class="ui-section-title">{{ t('system.sectionDefaultBandwidth') }}</h3>
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="ui-label">{{ t('system.defaultMbps') }}</label>
            <input
              v-model.number="form.bandwidth_mbps"
              type="number"
              min="0"
              step="0.1"
              class="ui-input"
            />
          </div>
        </div>
      </div>

      <div>
        <div class="mb-3 flex items-end justify-between gap-3">
          <h3 class="ui-section-title !mb-0">{{ t('system.sectionRateWindows') }}</h3>
          <button type="button" class="ui-btn !py-1.5 text-xs" @click="addWindow">
            {{ t('system.addWindow') }}
          </button>
        </div>

        <div v-if="!form.windows.length" class="rounded-md border border-dashed border-line px-4 py-6 text-center text-sm text-muted">
          {{ t('system.noWindows') }}
        </div>

        <div v-else class="space-y-2">
          <div
            v-for="(w, i) in form.windows"
            :key="i"
            class="grid grid-cols-1 items-end gap-3 rounded-md border border-line px-3 py-3 sm:grid-cols-[1fr_auto_1fr_1fr_auto]"
          >
            <div>
              <label class="ui-label">{{ t('system.windowStart') }}</label>
              <input v-model="w.start" type="time" class="ui-input" />
            </div>
            <span class="hidden pb-2.5 text-muted sm:block">→</span>
            <div>
              <label class="ui-label">{{ t('system.windowEnd') }}</label>
              <input v-model="w.end" type="time" class="ui-input" />
            </div>
            <div>
              <label class="ui-label">{{ t('system.windowMbps') }}</label>
              <input
                v-model.number="w.bandwidth_mbps"
                type="number"
                min="0"
                step="0.1"
                class="ui-input"
              />
            </div>
            <button
              type="button"
              class="ui-btn-ghost !px-2 !py-2 text-xs text-muted hover:text-danger"
              :title="t('common.delete')"
              @click="removeWindow(i)"
            >
              {{ t('common.delete') }}
            </button>
          </div>
        </div>
      </div>
    </section>

    <section
      v-else-if="tab === 'concurrency'"
      class="ui-panel space-y-6 p-5"
      @input="markDirty"
      @change="markDirty"
    >
      <div>
        <h3 class="ui-section-title">{{ t('system.sectionConcurrency') }}</h3>
        <div class="grid gap-4 sm:grid-cols-3">
          <div>
            <label class="ui-label">{{ t('platform.concurrency') }}</label>
            <input v-model.number="form.max_concurrent" type="number" min="1" class="ui-input" />
          </div>
          <div>
            <label class="ui-label">{{ t('platform.rangeConnections') }}</label>
            <input v-model.number="form.max_connections" type="number" min="1" class="ui-input" />
          </div>
          <div>
            <label class="ui-label">{{ t('platform.idleQuota') }}</label>
            <input
              v-model.number="form.idle_quota_ratio"
              type="number"
              min="0.05"
              max="1"
              step="0.05"
              class="ui-input"
            />
          </div>
        </div>
      </div>

      <div>
        <h3 class="ui-section-title">{{ t('platform.sectionInteractive') }}</h3>
        <div class="grid gap-4 sm:grid-cols-3">
          <div>
            <label class="ui-label">{{ t('platform.onInteractive') }}</label>
            <select v-model="form.on_interactive" class="ui-input">
              <option value="pause">{{ t('platform.pause') }}</option>
              <option value="continue">{{ t('platform.continueAction') }}</option>
            </select>
          </div>
          <div>
            <label class="ui-label">{{ t('platform.smallFileKB') }}</label>
            <input v-model.number="form.small_file_boost_kb" type="number" min="1" class="ui-input" />
          </div>
          <div class="flex flex-col justify-end gap-2 pb-1">
            <label class="flex items-center gap-2 text-sm">
              <input v-model="form.resume_on_idle" type="checkbox" class="accent-accent" />
              {{ t('platform.resumeOnIdle') }}
            </label>
            <label class="flex items-center gap-2 text-sm">
              <input v-model="form.small_file_boost_enabled" type="checkbox" class="accent-accent" />
              {{ t('platform.smallFileBoost') }}
            </label>
          </div>
        </div>
      </div>
    </section>

    <section v-else-if="tab === 'account'" class="ui-panel space-y-5 p-5">
      <p class="text-xs text-muted">
        {{ t('system.accountHint', { name: session.username || '—' }) }}
      </p>
      <form class="grid gap-4 sm:grid-cols-3" autocomplete="off" @submit.prevent="changePassword">
        <div>
          <label class="ui-label">{{ t('system.oldPassword') }}</label>
          <input
            v-model="pwd.old_password"
            type="password"
            class="ui-input"
            name="mirrorhub-current-password"
            autocomplete="current-password"
          />
        </div>
        <div>
          <label class="ui-label">{{ t('system.newPassword') }}</label>
          <input
            v-model="pwd.new_password"
            type="password"
            class="ui-input"
            name="mirrorhub-new-password"
            autocomplete="new-password"
          />
        </div>
        <div>
          <label class="ui-label">{{ t('system.confirmPassword') }}</label>
          <input
            v-model="pwd.confirm"
            type="password"
            class="ui-input"
            name="mirrorhub-confirm-password"
            autocomplete="new-password"
          />
        </div>
      </form>
    </section>
  </div>
</template>
