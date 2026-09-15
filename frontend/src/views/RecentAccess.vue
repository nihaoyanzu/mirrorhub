<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import EmptyState from '@/components/EmptyState.vue'
import PaginationBar from '@/components/PaginationBar.vue'
import { usePagination } from '@/composables/usePagination'
import { useStatsStore } from '@/stores/stats'
import { fmtBytes, fmtTime } from '@/lib/format'
import { cacheClass, cacheKey } from '@/lib/status'

const { t } = useI18n()
const statsStore = useStatsStore()
const recent = computed(() => statsStore.data?.traffic?.recent ?? [])

const {
  page,
  size: pageSize,
  total,
  pageCount,
  slice,
  go,
} = usePagination(recent, 20)
</script>

<template>
  <div>
    <section class="ui-panel overflow-hidden">
      <div v-if="total" class="overflow-x-auto">
        <table class="ui-table">
          <thead>
            <tr>
              <th>{{ t('access.time') }}</th>
              <th>{{ t('access.ip') }}</th>
              <th>{{ t('access.method') }}</th>
              <th>{{ t('access.path') }}</th>
              <th>{{ t('access.cacheStatus') }}</th>
              <th class="text-right">{{ t('access.delivered') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(r, i) in slice" :key="`${r.at}-${r.path}-${i}`">
              <td class="whitespace-nowrap text-sm text-muted">{{ fmtTime(r.at) }}</td>
              <td class="font-mono text-sm">{{ r.ip }}</td>
              <td class="font-mono text-sm">{{ r.method }}</td>
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
      <EmptyState v-else :title="t('access.noAccess')" />
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
