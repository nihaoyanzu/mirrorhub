<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'
import PaginationBar from '@/components/PaginationBar.vue'
import ModalDialog from '@/components/ModalDialog.vue'
import { api } from '@/api/client'
import { usePagination } from '@/composables/usePagination'
import { fmtBytes, fmtTime } from '@/lib/format'
import { useToastStore } from '@/stores/toast'
import type { PackageSummary, PackageDetail } from '@/types/api'

const { t } = useI18n()
const toast = useToastStore()
const loading = ref(false)
const detailLoading = ref(false)
const q = ref('')
const showDeleteModal = ref(false)
const pendingDeleteKey = ref('')
const packages = ref<PackageSummary[]>([])
const matchTotal = ref(0)
const listPage = ref(1)
const listSize = ref(20)
const catalogSize = ref(0)
const catalogReady = ref(false)
const catalogRefreshing = ref(false)
const catalogError = ref('')
const selected = ref('')
const detail = ref<PackageDetail | null>(null)
const showUpstream = ref(true)

let catalogPoll: number | undefined

const listPageCount = computed(() => Math.max(1, Math.ceil(matchTotal.value / listSize.value) || 1))

const catalogHint = computed(() => {
  if (catalogRefreshing.value && catalogSize.value > 0) {
    return t('packages.updating') + ` ${catalogSize.value}`
  }
  if (catalogRefreshing.value) return t('packages.indexPulling')
  if (catalogReady.value && catalogSize.value > 0) return t('packages.packagesCount', { n: catalogSize.value })
  if (catalogError.value) return t('packages.indexUnavailable')
  return ''
})

const filteredHint = computed(() => {
  if (!q.value.trim()) return catalogHint.value
  if (matchTotal.value <= 0 && !packages.value.length) return catalogHint.value
  const match = t('packages.matchCount', { n: matchTotal.value })
  return catalogHint.value ? `${match} · ${catalogHint.value}` : match
})

const emptyTitle = computed(() => {
  if (loading.value) return t('common.loading')
  if (!catalogReady.value && catalogRefreshing.value) return t('packages.indexPreparing')
  if (q.value.trim()) return t('packages.noMatch')
  return t('packages.inputName')
})

const emptyDesc = computed(() => {
  if (loading.value) return ''
  if (!catalogReady.value && catalogRefreshing.value) {
    return catalogSize.value > 0 ? t('packages.loadedCount', { n: catalogSize.value }) : t('packages.firstPull')
  }
  if (catalogError.value) return catalogError.value
  if (q.value.trim()) return ''
  return t('packages.localMark')
})

function applyCatalogMeta(res: {
  catalog_size?: number
  catalog_ready?: boolean
  catalog_refreshing?: boolean
  catalog_error?: string
}) {
  catalogSize.value = res.catalog_size || 0
  catalogReady.value = !!res.catalog_ready
  catalogRefreshing.value = !!res.catalog_refreshing
  catalogError.value = res.catalog_error || ''
}

function syncCatalogPoll() {
  window.clearInterval(catalogPoll)
  if (catalogRefreshing.value || !catalogReady.value) {
    catalogPoll = window.setInterval(() => loadList(true), 2000)
  }
}

async function loadList(silent = false) {
  const query = q.value.trim()
  if (!silent) loading.value = true
  try {
    const res = await api.listPackages({
      q: query || undefined,
      page: listPage.value,
      page_size: listSize.value,
    })
    packages.value = res.packages || []
    matchTotal.value = res.total || 0
    if (res.page && res.page > 0) listPage.value = res.page
    if (res.page_size && res.page_size > 0) listSize.value = res.page_size
    applyCatalogMeta(res)
    if (listPage.value > listPageCount.value) {
      listPage.value = listPageCount.value
    }
    if (selected.value && query && matchTotal.value === 0) {
      selected.value = ''
      detail.value = null
    }
    syncCatalogPoll()
  } catch (e: any) {
    if (!silent) toast.err(e.message || t('packages.searchFailed'))
  } finally {
    loading.value = false
  }
}

function goList(p: number) {
  const next = Math.min(Math.max(1, p), listPageCount.value)
  if (next === listPage.value) return
  listPage.value = next
  loadList()
}

const files = computed(() => (detail.value?.files || []))
const {
  page: filePage,
  size: fileSize,
  total: fileTotal,
  pageCount: filePageCount,
  slice: fileSlice,
  go: goFile,
  reset: resetFiles,
} = usePagination(files, 12)

const versions = computed(() => {
  const d = detail.value
  if (!d) return [] as string[]
  return (d.upstream?.versions || d.index_versions || d.versions || [])
})
const {
  page: verPage,
  size: verSize,
  total: verTotal,
  pageCount: verPageCount,
  slice: verSlice,
  go: goVer,
  reset: resetVers,
} = usePagination(versions, 36)

async function openPackage(name: string) {
  selected.value = name
  detailLoading.value = true
  resetFiles()
  resetVers()
  try {
    detail.value = await api.getPackage(name, showUpstream.value)
  } catch (e: any) {
    toast.err(e.message || t('packages.loadDetailFailed'))
    detail.value = null
  } finally {
    detailLoading.value = false
  }
}

function requestRemoveEntry(key: string) {
  pendingDeleteKey.value = key
  showDeleteModal.value = true
}

async function confirmDelete() {
  try {
    await api.deletePackageEntry(pendingDeleteKey.value)
    toast.ok(t('packages.deleted'))
    await loadList()
    if (selected.value) await openPackage(selected.value)
  } catch (e: any) {
    toast.err(e.message || t('packages.deleteFailed'))
  } finally {
    showDeleteModal.value = false
    pendingDeleteKey.value = ''
  }
}

async function requestPrefetch(version: string) {
  if (!selected.value || !version) return
  const spec = `${selected.value}==${version}`
  try {
    await api.prefetch([spec])
    toast.ok(t('packages.submittedPrefetch', { name: spec }))
  } catch (e: any) {
    toast.err(e.message || t('prefetch.submitFailed'))
  }
}

let searchTimer: number | undefined
watch(q, () => {
  window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => {
    listPage.value = 1
    loadList()
  }, 280)
})
watch(showUpstream, () => {
  if (selected.value) openPackage(selected.value)
})

onMounted(() => loadList())
onUnmounted(() => {
  window.clearTimeout(searchTimer)
  window.clearInterval(catalogPoll)
})
</script>

<template>
  <div>
    <PageHeader :title="t('packages.title')" :description="t('packages.subtitle')">
      <template #actions>
        <span v-if="filteredHint" class="ui-badge-muted">{{ filteredHint }}</span>
      </template>
    </PageHeader>

    <div class="mb-4">
      <input
        v-model="q"
        class="ui-input w-full"
        :placeholder="t('packages.placeholder')"
        autofocus
      />
    </div>

    <div class="grid gap-4 lg:grid-cols-[minmax(280px,340px)_1fr]">
      <section class="ui-panel flex max-h-[75vh] flex-col overflow-hidden">
        <div class="ui-section-title shrink-0 border-b border-line px-4 py-2.5 !mb-0">{{ t('packages.results') }}</div>
        <div class="min-h-0 flex-1 overflow-y-auto">
          <button
            v-for="p in packages"
            :key="p.name"
            class="flex w-full flex-col gap-1 border-b border-line/60 px-4 py-2.5 text-left transition hover:bg-panel-2/50"
            :class="selected === p.name ? 'bg-accent/10' : ''"
            @click="openPackage(p.name)"
          >
            <div class="flex items-center justify-between gap-2">
              <span class="font-medium text-fg">{{ p.name }}</span>
              <span v-if="p.cached" class="ui-badge-ok">{{ p.file_count }}</span>
              <span v-else-if="p.has_index" class="ui-badge">{{ t('packages.versions') }}</span>
              <span v-else class="text-xs text-muted">—</span>
            </div>
            <div v-if="p.cached" class="truncate text-xs text-muted">
              {{ (p.versions || []).slice(0, 3).join(' · ') }}
              <span v-if="(p.versions || []).length > 3"> …</span>
            </div>
          </button>
          <EmptyState
            v-if="!packages.length"
            :title="emptyTitle"
            :description="emptyDesc || undefined"
          />
        </div>
        <PaginationBar
          class="shrink-0"
          :page="listPage"
          :page-count="listPageCount"
          :total="matchTotal"
          :page-size="listSize"
          @update:page="goList"
        />
      </section>

      <section class="ui-panel overflow-hidden">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-line px-4 py-2.5">
          <h2 class="ui-section-title !mb-0">{{ selected || t('packages.detail') }}</h2>
          <label class="flex items-center gap-2 text-xs text-muted">
            <input v-model="showUpstream" type="checkbox" class="accent-accent" />
            {{ t('packages.upstreamVersions') }}
          </label>
        </div>

        <div v-if="!selected" class="p-2">
          <EmptyState :title="t('packages.selectLeft')" />
        </div>
        <div v-else-if="detailLoading" class="p-6 text-sm text-muted">{{ t('common.loading') }}</div>
        <div v-else-if="detail" class="space-y-4 p-4">
          <div class="flex flex-wrap gap-2 text-xs">
            <span class="ui-badge-muted">{{ t('packages.files') }} {{ detail.package_count ?? 0 }}</span>
            <span class="ui-badge-muted">{{ fmtBytes(detail.total_size || 0) }}</span>
            <span class="ui-badge">{{ t('packages.local') }} {{ (detail.versions || []).length }}</span>
            <span v-if="detail.upstream" class="ui-badge-ok">
              {{ t('packages.upstream') }} {{ (detail.upstream.versions || []).length }}
            </span>
          </div>

          <div>
            <h3 class="ui-section-title">{{ t('packages.versions') }}</h3>
            <p v-if="detail.upstream_error" class="mb-2 text-sm text-danger">{{ detail.upstream_error }}</p>
            <div v-if="verSlice.length" class="flex flex-wrap gap-1.5">
              <button
                v-for="v in verSlice"
                :key="v"
                type="button"
                class="ui-chip font-mono"
                :class="(detail.versions || []).includes(v) ? 'ui-chip-active !text-ok ring-ok/30' : ''"
                :title="t('packages.prefetchVersion')"
                @click="requestPrefetch(v)"
              >
                {{ v }}
                <span v-if="(detail.versions || []).includes(v)" class="opacity-70">✓</span>
              </button>
            </div>
            <p v-else class="text-xs text-muted">{{ t('packages.noVersions') }}</p>
            <PaginationBar
              :page="verPage"
              :page-count="verPageCount"
              :total="verTotal"
              :page-size="verSize"
              @update:page="goVer"
            />
          </div>

          <div>
            <h3 class="ui-section-title">{{ t('packages.localFiles') }}</h3>
            <div class="overflow-x-auto">
              <table v-if="fileSlice.length" class="ui-table">
                <thead>
                  <tr>
                    <th>{{ t('packages.type') }}</th>
                    <th>{{ t('packages.version') }}</th>
                    <th>{{ t('packages.fileName') }}</th>
                    <th>{{ t('packages.size') }}</th>
                    <th>{{ t('packages.access') }}</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="f in fileSlice" :key="f.key">
                    <td><span class="ui-badge-muted">{{ f.type || f.kind }}</span></td>
                    <td class="font-mono text-xs">{{ f.version || '-' }}</td>
                    <td class="max-w-[280px]">
                      <div class="truncate font-mono text-xs" :title="f.source_url || f.filename">
                        {{ f.filename || f.source_url || f.key }}
                      </div>
                    </td>
                    <td class="whitespace-nowrap font-mono text-xs">{{ fmtBytes(f.size || 0) }}</td>
                    <td class="whitespace-nowrap text-xs text-muted">{{ fmtTime(f.last_access) }}</td>
                    <td>
                      <button class="ui-btn-danger !px-2 !py-1 text-xs" @click="requestRemoveEntry(f.key)">
                        {{ t('common.delete') }}
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
              <p v-else class="text-xs text-muted">{{ t('packages.noLocalCache') }}</p>
            </div>
            <PaginationBar
              :page="filePage"
              :page-count="filePageCount"
              :total="fileTotal"
              :page-size="fileSize"
              @update:page="goFile"
            />
          </div>
        </div>
      </section>
    </div>

    <!-- 删除确认弹窗 -->
    <ModalDialog
      v-model:visible="showDeleteModal"
      :title="t('common.delete')"
      :description="t('packages.deleteConfirm')"
      :confirm-text="t('common.delete')"
      danger
      @confirm="confirmDelete"
      @cancel="showDeleteModal = false"
    />
  </div>
</template>
