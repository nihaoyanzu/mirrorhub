<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import EmptyState from '@/components/EmptyState.vue'
import PaginationBar from '@/components/PaginationBar.vue'
import { usePagination } from '@/composables/usePagination'
import { useStatsStore } from '@/stores/stats'
import { fmtBytes, fmtTime } from '@/lib/format'
import { cacheClass, cacheKey } from '@/lib/status'
import { MODULE_BY_ID } from '@/modules/registry'

const { t } = useI18n()
const statsStore = useStatsStore()
const recent = computed(() => statsStore.data?.traffic?.recent ?? [])

const platformFilter = ref('')

const platformOptions = computed(() => {
  const set = new Set<string>()
  for (const r of recent.value) {
    const p = String(r.platform || '').trim()
    if (p) set.add(p)
  }
  return [...set].sort((a, b) => a.localeCompare(b))
})

watch(platformOptions, (opts) => {
  if (platformFilter.value && !opts.includes(platformFilter.value)) {
    platformFilter.value = ''
  }
})

const filtered = computed(() => {
  const id = platformFilter.value
  if (!id) return recent.value
  return recent.value.filter((r) => String(r.platform || '').trim() === id)
})

const {
  page,
  size: pageSize,
  total,
  pageCount,
  slice,
  go,
  reset,
} = usePagination(filtered, 20)

watch(platformFilter, () => reset())

function platformLabel(id: string) {
  return MODULE_BY_ID[id]?.guideTitle || id
}
</script>

<template>
  <div>
    <section class="ui-panel overflow-hidden">
      <div class="overflow-x-auto">
        <table class="ui-table">
          <thead>
            <tr>
              <th>{{ t('access.time') }}</th>
              <th>{{ t('access.ip') }}</th>
              <th>
                <select
                  v-model="platformFilter"
                  class="ui-input !w-auto max-w-[9rem] min-w-[6.5rem] py-1 text-xs font-medium"
                  :aria-label="t('access.platform')"
                >
                  <option value="">{{ t('access.allPlatforms') }}</option>
                  <option v-for="id in platformOptions" :key="id" :value="id">
                    {{ platformLabel(id) }}
                  </option>
                </select>
              </th>
              <th>{{ t('access.path') }}</th>
              <th>{{ t('access.cacheStatus') }}</th>
              <th class="text-right">{{ t('access.delivered') }}</th>
            </tr>
          </thead>
          <tbody v-if="total">
            <tr v-for="(r, i) in slice" :key="`${r.at}-${r.path}-${i}`">
              <td class="whitespace-nowrap text-sm text-muted">{{ fmtTime(r.at) }}</td>
              <td class="font-mono text-sm">{{ r.ip }}</td>
              <td class="font-mono text-sm">{{ platformLabel(r.platform) || '—' }}</td>
              <td class="max-w-[480px] truncate font-mono text-sm text-muted" :title="r.path">
                {{ r.path }}
              </td>
              <td>
                <span :class="cacheClass(r.cache)">{{ t(cacheKey(r.cache)) }}</span>
              </td>
              <td class="text-right font-mono text-sm">{{ fmtBytes(r.bytes) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <EmptyState v-if="!total" :title="t('access.noAccess')" />
      <PaginationBar
        :page="page"
        :page-count="pageCount"
        :total="total"
        :page-size="pageSize"
        @update:page="go"
      />
    </section>
  </div>
</template>
