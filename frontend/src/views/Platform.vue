<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import PageHeader from '@/components/PageHeader.vue'
import ModalDialog from '@/components/ModalDialog.vue'
import { api } from '@/api/client'
import { useToastStore } from '@/stores/toast'
import type { AccessTestCheck, AppConfig } from '@/types/api'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const toast = useToastStore()
const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const dirty = ref(false)
const tab = ref<'access' | 'download' | 'prefetch' | 'cache'>('access')
const showClearModal = ref(false)
const clearing = ref(false)
const testChecks = ref<AccessTestCheck[] | null>(null)

const form = reactive({
  enabled: true,
  upstream: 'https://mirrors.aliyun.com/pypi',
  file_upstream: 'https://mirrors.aliyun.com/pypi',
  metadata_upstream: '',
  concurrency: 16,
  chunk_size: 5242880,
  min_size: 102400,
  idle_quota_ratio: 0.3,
  on_interactive: 'pause',
  resume_on_idle: true,
  artifact_mode: 'portable',
  extra_wheel_tags: '',
  target_python: '3.10\n3.11\n3.12',
  target_platforms: ['linux'] as string[],
  max_depth: 5,
  max_packages: 200,
  small_file_boost_enabled: true,
  small_file_boost_kb: 512,
  index_ttl_seconds: 604800,
  package_ttl_seconds: 0,
  max_size_gb: 100,
})

/** 系统级字段：探测时只读带入，不在本页编辑 */
const systemNet = reactive({
  upstream_proxy: '',
})

const tabs = computed(() => [
  { id: 'access' as const, label: t('platform.tabAccess') },
  { id: 'download' as const, label: t('platform.tabDownload') },
  { id: 'prefetch' as const, label: t('platform.tabPrefetch') },
  { id: 'cache' as const, label: t('platform.tabCache') },
])

const platformOptions = [
  { id: 'linux', labelKey: 'platform.platLinux' },
  { id: 'linux-arm', labelKey: 'platform.platLinuxArm' },
  { id: 'win32', labelKey: 'platform.platWin' },
  { id: 'win-arm', labelKey: 'platform.platWinArm' },
  { id: 'darwin', labelKey: 'platform.platDarwin' },
  { id: 'darwin-arm', labelKey: 'platform.platDarwinArm' },
] as const

function normalizePlatforms(raw: unknown, legacy?: string): string[] {
  const allowed = new Set<string>(platformOptions.map((o) => o.id))
  const out: string[] = []
  const push = (v: string) => {
    const id = String(v || '').trim().toLowerCase()
    if (!id || !allowed.has(id)) return
    if (!out.includes(id)) out.push(id)
  }
  if (Array.isArray(raw)) {
    for (const v of raw) push(String(v))
  }
  if (out.length === 0 && legacy) push(legacy)
  return out.length ? out : ['linux']
}

function togglePlatform(id: string) {
  const i = form.target_platforms.indexOf(id)
  if (i >= 0) {
    if (form.target_platforms.length <= 1) return
    form.target_platforms.splice(i, 1)
  } else {
    form.target_platforms.push(id)
  }
  markDirty()
}

function syncTabFromRoute() {
  const q = String(route.query.tab || '')
  if (q === 'rate') {
    tab.value = 'download'
    return
  }
  if (q === 'access' || q === 'download' || q === 'prefetch' || q === 'cache') {
    tab.value = q
  }
}

function setTab(id: typeof tab.value) {
  tab.value = id
  router.replace({ query: { ...route.query, tab: id } })
}

watch(() => route.query.tab, syncTabFromRoute)

function markDirty() {
  dirty.value = true
}

async function load() {
  loading.value = true
  try {
    const cfg: AppConfig = await api.getConfig()
    systemNet.upstream_proxy = cfg.server?.upstream_proxy || ''
    const p = cfg.platforms?.pypi
    if (p) {
      form.enabled = !!p.enabled
      form.upstream = p.upstream || ''
      form.file_upstream = p.file_upstream || ''
      form.metadata_upstream = p.metadata_upstream || ''
      form.concurrency = p.download?.concurrency ?? 16
      form.chunk_size = p.download?.chunk_size ?? 5242880
      form.min_size = p.download?.min_size ?? 102400
    }
    if (cfg.cache) {
      form.index_ttl_seconds = cfg.cache.index_ttl_seconds
      form.package_ttl_seconds = cfg.cache.package_ttl_seconds
      form.max_size_gb = cfg.cache.max_size_gb ?? 100
    }
    const sch = cfg.scheduler
    if (sch) {
      form.idle_quota_ratio = sch.prefetch?.idle_quota_ratio ?? 0.3
      form.on_interactive = sch.prefetch?.on_interactive || 'pause'
      form.resume_on_idle = sch.prefetch?.resume_on_idle !== false
      form.artifact_mode = sch.prefetch?.artifact_mode || 'portable'
      form.extra_wheel_tags = (sch.prefetch?.extra_wheel_tags || []).join('\n')
      const tp = sch.prefetch?.target_python
      form.target_python = Array.isArray(tp) ? tp.join('\n') : tp || '3.10\n3.11\n3.12'
      form.target_platforms = normalizePlatforms(sch.prefetch?.target_platforms, sch.prefetch?.target_platform)
      form.max_depth = sch.prefetch?.max_depth ?? 5
      form.max_packages = sch.prefetch?.max_packages ?? 200
      form.small_file_boost_enabled = sch.small_file_boost?.enabled !== false
      form.small_file_boost_kb = sch.small_file_boost?.max_size_kb ?? 512
    }
    dirty.value = false
  } catch (e: any) {
    toast.err(e.message || t('platform.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    await api.putConfig({
      cache: {
        max_size_gb: form.max_size_gb,
        index_ttl_seconds: form.index_ttl_seconds,
        package_ttl_seconds: form.package_ttl_seconds,
      },
      scheduler: {
        prefetch: {
          idle_quota_ratio: form.idle_quota_ratio,
          on_interactive: form.on_interactive,
          resume_on_idle: form.resume_on_idle,
          artifact_mode: form.artifact_mode,
          extra_wheel_tags: form.extra_wheel_tags
            .split(/[\n,]+/)
            .map((s) => s.trim())
            .filter(Boolean),
          target_python: form.target_python
            .split(/[\n,]+/)
            .map((s) => s.trim())
            .filter(Boolean),
          target_platforms: form.target_platforms.length ? [...form.target_platforms] : ['linux'],
          target_platform: form.target_platforms[0] || 'linux',
          max_depth: form.max_depth,
          max_packages: form.max_packages,
        },
        small_file_boost: {
          enabled: form.small_file_boost_enabled,
          max_size_kb: form.small_file_boost_kb,
        },
      },
      platforms: {
        pypi: {
          enabled: form.enabled,
          upstream: form.upstream,
          file_upstream: form.file_upstream,
          metadata_upstream: form.metadata_upstream,
          download: {
            concurrency: form.concurrency,
            chunk_size: form.chunk_size,
            min_size: form.min_size,
          },
        },
      },
    })
    dirty.value = false
    toast.ok(t('platform.saved'))
  } catch (e: any) {
    toast.err(e.message || t('platform.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function clearCache() {
  clearing.value = true
  try {
    await api.clearCache()
    toast.ok(t('cache.cleared'))
  } catch (e: any) {
    toast.err(e.message || t('cache.clearFailed'))
  } finally {
    clearing.value = false
    showClearModal.value = false
  }
}

async function testAccess() {
  testing.value = true
  testChecks.value = null
  try {
    const res = await api.testAccess({
      upstream_proxy: systemNet.upstream_proxy,
      upstream: form.upstream,
      file_upstream: form.file_upstream,
      metadata_upstream: form.metadata_upstream,
    })
    testChecks.value = res.checks || []
    if (res.ok) toast.ok(t('platform.testOk'))
    else toast.err(t('platform.testFailed'))
  } catch (e: any) {
    toast.err(e.message || t('platform.testFailed'))
  } finally {
    testing.value = false
  }
}

function checkLabel(name: string) {
  const map: Record<string, string> = {
    index_upstream: t('platform.indexUpstream'),
    file_upstream: t('platform.fileUpstream'),
    metadata_upstream: t('platform.metadataUpstream'),
    upstream_proxy: t('system.upstreamProxy'),
  }
  return map[name] || name
}

onMounted(() => {
  syncTabFromRoute()
  load()
})
</script>

<template>
  <div>
    <PageHeader :title="t('platform.title')" :description="t('platform.subtitle')">
      <template #actions>
        <span v-if="dirty" class="ui-badge-warn">{{ t('common.unsaved') }}</span>
        <button class="ui-btn" :disabled="loading || saving" @click="load">{{ t('platform.reload') }}</button>
        <button class="ui-btn-primary" :disabled="loading || saving" @click="save">
          {{ saving ? t('platform.saving') : t('common.save') }}
        </button>
      </template>
    </PageHeader>

    <div class="mb-4 flex flex-wrap gap-1 rounded-xl border border-line bg-panel/60 p-1">
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

    <div class="space-y-4" @input="markDirty" @change="markDirty">
      <section v-show="tab === 'access'" class="ui-panel p-5">
        <div class="mb-5">
          <label class="flex items-center gap-3 text-sm font-medium text-fg">
            <span class="ui-switch">
              <input v-model="form.enabled" type="checkbox" />
              <span class="ui-switch-track" aria-hidden="true" />
              <span class="ui-switch-thumb" aria-hidden="true" />
            </span>
            {{ t('platform.enablePyPI') }}
          </label>
        </div>

        <h3 class="ui-section-title">{{ t('platform.sectionUpstream') }}</h3>
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="ui-label">{{ t('platform.indexUpstream') }}</label>
            <input v-model="form.upstream" class="ui-input" />
          </div>
          <div>
            <label class="ui-label">{{ t('platform.fileUpstream') }}</label>
            <input v-model="form.file_upstream" class="ui-input" />
          </div>
          <div class="sm:col-span-2">
            <label class="ui-label">
              {{ t('platform.metadataUpstream') }}
              <span class="group relative ml-1 inline-flex cursor-help items-center">
                <svg class="h-3.5 w-3.5 text-muted/60" viewBox="0 0 20 20" fill="currentColor">
                  <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd" />
                </svg>
                <span class="pointer-events-none absolute bottom-full left-1/2 z-50 mb-2 w-64 -translate-x-1/2 rounded-lg border border-line bg-panel px-3 py-2 text-xs text-muted opacity-0 shadow-lg transition-opacity group-hover:opacity-100">
                  {{ t('platform.metadataHint') }}
                </span>
              </span>
            </label>
            <input
              v-model="form.metadata_upstream"
              class="ui-input"
              placeholder="PEP 658；留空回退到包文件上游"
            />
          </div>
        </div>

        <div class="mt-5 flex flex-wrap items-center gap-3 border-t border-line pt-4">
          <button type="button" class="ui-btn" :disabled="loading || testing" @click.stop="testAccess">
            {{ testing ? t('platform.testing') : t('platform.testAccess') }}
          </button>
        </div>

        <ul v-if="testChecks?.length" class="mt-4 space-y-2">
          <li v-for="(c, i) in testChecks" :key="i" class="rounded-lg border border-line px-3 py-2 text-sm">
            <div class="flex flex-wrap items-center gap-2">
              <span
                class="inline-block h-2 w-2 rounded-full"
                :class="c.skipped ? 'bg-muted' : c.ok ? 'bg-ok' : 'bg-danger'"
              />
              <span class="font-medium text-fg">{{ checkLabel(c.name) }}</span>
              <span v-if="c.skipped" class="text-xs text-muted">{{ t('platform.testSkipped') }}</span>
              <span v-else-if="c.ok" class="text-xs text-ok">{{ t('platform.testPass') }}</span>
              <span v-else class="text-xs text-danger">{{ t('platform.testFail') }}</span>
              <span v-if="c.ms" class="ml-auto font-mono text-xs text-muted">{{ c.ms }}ms</span>
            </div>
            <p class="mt-1 break-all font-mono text-xs text-muted">{{ c.detail }}</p>
          </li>
        </ul>
      </section>

      <section v-show="tab === 'download'" class="ui-panel space-y-6 p-5">
        <div>
          <h3 class="ui-section-title">{{ t('platform.sectionDownload') }}</h3>
          <div class="grid gap-4 sm:grid-cols-3">
            <div>
              <label class="ui-label">{{ t('platform.chunkConcurrency') }}</label>
              <input v-model.number="form.concurrency" type="number" min="1" class="ui-input" />
            </div>
            <div>
              <label class="ui-label">{{ t('platform.chunkSize') }}</label>
              <input v-model.number="form.chunk_size" type="number" min="1" class="ui-input" />
            </div>
            <div>
              <label class="ui-label">{{ t('platform.parallelMinSize') }}</label>
              <input v-model.number="form.min_size" type="number" min="0" class="ui-input" />
            </div>
          </div>
        </div>
      </section>

      <section v-show="tab === 'prefetch'" class="ui-panel p-5">
        <div class="grid gap-4 sm:grid-cols-3">
          <div>
            <label class="ui-label">{{ t('platform.artifactMode') }}</label>
            <select v-model="form.artifact_mode" class="ui-input">
              <option value="portable">portable</option>
              <option value="all">all</option>
            </select>
          </div>
          <div class="sm:col-span-2">
            <label class="ui-label">{{ t('platform.minPython') }}</label>
            <textarea
              v-model="form.target_python"
              rows="2"
              class="ui-input font-mono text-sm"
              placeholder="3.10&#10;3.11&#10;3.12"
            />
          </div>
          <div class="sm:col-span-3">
            <label class="ui-label">{{ t('platform.targetPlatform') }}</label>
            <p class="mb-2 text-xs text-muted">{{ t('platform.targetPlatformHint') }}</p>
            <div class="flex flex-wrap gap-2">
              <button
                v-for="opt in platformOptions"
                :key="opt.id"
                type="button"
                class="ui-chip"
                :class="{ 'ui-chip-active': form.target_platforms.includes(opt.id) }"
                @click="togglePlatform(opt.id)"
              >
                {{ t(opt.labelKey) }}
              </button>
            </div>
          </div>
          <div>
            <label class="ui-label">{{ t('platform.depDepth') }}</label>
            <input v-model.number="form.max_depth" type="number" min="0" max="20" class="ui-input" />
          </div>
          <div>
            <label class="ui-label">{{ t('platform.maxPackages') }}</label>
            <input v-model.number="form.max_packages" type="number" min="1" max="2000" class="ui-input" />
          </div>
          <div class="sm:col-span-3">
            <label class="ui-label">{{ t('platform.extraWheelTags') }}</label>
            <textarea
              v-model="form.extra_wheel_tags"
              rows="2"
              class="ui-input font-mono text-sm"
              placeholder="manylinux2014_x86_64"
            />
          </div>
        </div>
      </section>

      <section v-show="tab === 'cache'" class="ui-panel p-5">
        <div class="grid gap-4 sm:grid-cols-3">
          <div>
            <label class="ui-label">{{ t('platform.cacheGB') }}</label>
            <input v-model.number="form.max_size_gb" type="number" min="1" class="ui-input" />
          </div>
          <div>
            <label class="ui-label">{{ t('platform.indexTTL') }}</label>
            <input v-model.number="form.index_ttl_seconds" type="number" min="1" class="ui-input" />
            <p class="mt-1 text-xs text-muted">{{ t('platform.indexTTLHint') }}</p>
          </div>
          <div>
            <label class="ui-label">{{ t('platform.packageTTL') }}</label>
            <input v-model.number="form.package_ttl_seconds" type="number" min="0" class="ui-input" />
            <p class="mt-1 text-xs text-muted">{{ t('platform.packageTTLHint') }}</p>
          </div>
        </div>
        <div class="mt-6 border-t border-danger/20 pt-5">
          <h3 class="mb-2 text-sm font-medium text-danger">{{ t('platform.dangerZone') }}</h3>
          <p class="mb-4 text-xs text-muted">{{ t('platform.clearCacheDesc') }}</p>
          <button class="ui-btn-danger" :disabled="clearing" @click="showClearModal = true">
            {{ t('platform.clearCache') }}
          </button>
        </div>
      </section>
    </div>

    <ModalDialog
      v-model:visible="showClearModal"
      :title="t('platform.clearCache')"
      :description="t('platform.clearCacheConfirm')"
      danger
      @cancel="showClearModal = false"
      @confirm="clearCache"
    />
  </div>
</template>
