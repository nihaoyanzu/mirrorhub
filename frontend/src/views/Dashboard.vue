<script setup lang="ts">
import { computed, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import StatTile from '@/components/StatTile.vue'
import EmptyState from '@/components/EmptyState.vue'
import DonutChart from '@/components/DonutChart.vue'
import { useStatsStore } from '@/stores/stats'
import { fmtBytes, fmtRate, hitRate } from '@/lib/format'

const { t } = useI18n()
const statsStore = useStatsStore()
const stats = computed(() => statsStore.data)

onUnmounted(() => statsStore.stopPolling())

const upstreamBps = computed(() => fmtRate(Number(stats.value?.traffic?.upstream_bps ?? 0)))
const downstreamBps = computed(() => fmtRate(Number(stats.value?.traffic?.downstream_bps ?? 0)))
const interactiveActive = computed(() => stats.value?.interactive_active ?? 0)
const resumeCount = computed(() => stats.value?.resume_count ?? 0)
const hitRateStr = computed(() => {
  const c = stats.value?.cache
  return hitRate(Number(c?.hits ?? 0), Number(c?.misses ?? 0))
})
const capacityCenter = computed(() => {
  const used = Number(stats.value?.cache?.total_size ?? 0)
  const max = Number(stats.value?.cache?.max_size ?? 0)
  if (!max) return '—'
  return `${Math.min(100, (used / max) * 100).toFixed(0)}%`
})

const cacheSlices = computed(() => {
  const used = Number(stats.value?.cache?.total_size ?? 0)
  const max = Number(stats.value?.cache?.max_size ?? 0)
  const free = max > used ? max - used : 0
  return [
    { label: t('dashboard.used'), value: used, color: 'var(--color-accent)' },
    { label: t('dashboard.freeCapacity'), value: free, color: 'var(--color-line)' },
  ]
})

const hitSlices = computed(() => [
  { label: t('dashboard.hit'), value: Number(stats.value?.cache?.hits ?? 0), color: 'var(--color-ok)' },
  { label: t('dashboard.miss'), value: Number(stats.value?.cache?.misses ?? 0), color: 'var(--color-warn)' },
])

const limiters = computed(() => stats.value?.rate_limiters ?? [])
</script>

<template>
  <div>
    <div class="mb-6 grid grid-cols-2 gap-3 sm:grid-cols-4">
      <StatTile :label="t('dashboard.pull')" :value="upstreamBps" icon="pull" />
      <StatTile :label="t('dashboard.share')" :value="downstreamBps" icon="share" />
      <StatTile :label="t('dashboard.p0Interactive')" :value="String(interactiveActive)" tone="warn" />
      <StatTile :label="t('dashboard.p1Resume')" :value="String(resumeCount)" tone="ok" />
    </div>

    <div v-if="stats" class="mb-6 grid gap-4 lg:grid-cols-2">
      <section class="ui-panel p-4">
        <div class="mb-3 flex items-center justify-between">
          <h2 class="text-sm font-medium text-fg">{{ t('dashboard.capacity') }}</h2>
          <span
            v-if="stats.cache?.water_crit || stats.cache?.water_warn"
            class="h-2 w-2 rounded-full"
            :class="stats.cache?.water_crit ? 'bg-danger' : 'bg-warn'"
            :title="stats.cache?.water_crit ? t('cache.critical') : t('cache.warning')"
          />
        </div>
        <DonutChart
          :slices="cacheSlices"
          value-format="bytes"
          :center-label="t('dashboard.usage')"
          :center-value="capacityCenter"
        />
        <div class="mt-4 grid grid-cols-3 gap-2 border-t border-line pt-3 text-center">
          <div>
            <div class="text-xs uppercase tracking-wide text-muted">{{ t('dashboard.entries') }}</div>
            <div class="mt-1 font-mono text-base text-fg">{{ stats.cache?.entries ?? 0 }}</div>
          </div>
          <div>
            <div class="text-xs uppercase tracking-wide text-muted">{{ t('dashboard.usage') }}</div>
            <div class="mt-1 font-mono text-base text-fg">{{ fmtBytes(stats.cache?.total_size ?? 0) }}</div>
          </div>
          <div>
            <div class="text-xs uppercase tracking-wide text-muted">{{ t('dashboard.disk') }}</div>
            <div class="mt-1 font-mono text-base text-fg">
              {{ stats.cache?.disk_free_ok ? fmtBytes(stats.cache?.disk_free_bytes ?? 0) : t('dashboard.unknown') }}
            </div>
          </div>
        </div>
      </section>
      <section class="ui-panel p-4">
        <h2 class="mb-3 text-sm font-medium text-fg">{{ t('dashboard.cacheHit') }}</h2>
        <DonutChart
          :slices="hitSlices"
          :center-label="t('dashboard.hitRate')"
          :center-value="hitRateStr"
        />
      </section>
    </div>

    <section class="ui-panel overflow-x-auto">
      <div class="flex flex-wrap items-center justify-between gap-2 border-b border-line px-4 py-2.5">
        <h2 class="text-sm font-medium text-fg">{{ t('dashboard.rateLimit') }}</h2>
        <RouterLink class="ui-btn-ghost !py-1 text-xs" to="/settings?tab=rate">
          {{ t('dashboard.openSettings') }}
        </RouterLink>
      </div>
      <div v-if="limiters.length" class="overflow-x-auto">
        <table class="ui-table">
          <thead>
            <tr>
              <th>{{ t('dashboard.platform') }}</th>
              <th>{{ t('dashboard.bandwidthMode') }}</th>
              <th class="text-right">{{ t('platform.prefetchMbps') }}</th>
              <th class="text-right">{{ t('dashboard.tasks') }}</th>
              <th class="text-right">{{ t('dashboard.boost') }}</th>
              <th class="text-right">{{ t('dashboard.connections') }}</th>
              <th>{{ t('dashboard.prefetchStatus') }}</th>
              <th class="text-right">P50 {{ t('dashboard.wait') }}</th>
              <th class="text-right">P95 {{ t('dashboard.wait') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(r, i) in limiters" :key="i">
              <td class="font-medium">{{ r.platform }}</td>
              <td>
                <span class="text-xs" :class="r.bandwidth_limited ? 'text-warn' : 'text-ok'">
                  {{ r.bandwidth_limited ? t('dashboard.rateLimited') : t('dashboard.fullSpeed') }}
                </span>
              </td>
              <td class="text-right font-mono text-xs">
                {{ r.bandwidth_limited ? (r.effective_bandwidth_mbps ?? r.bandwidth_mbps) : '∞' }}
              </td>
              <td class="text-right font-mono text-xs">{{ r.active_tasks }}</td>
              <td class="text-right font-mono text-xs">{{ r.active_boost }}/{{ r.boost_slots }}</td>
              <td class="text-right font-mono text-xs">{{ r.active_conns }}</td>
              <td>
                <span class="text-xs" :class="r.prefetch_paused ? 'text-warn' : 'text-ok'">
                  {{ r.prefetch_paused ? t('dashboard.paused') : t('dashboard.running') }}
                </span>
              </td>
              <td class="text-right font-mono text-xs">{{ r.rate_limit_wait_p50 }} ms</td>
              <td class="text-right font-mono text-xs">{{ r.rate_limit_wait_p95 }} ms</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-else>
        <EmptyState :title="t('dashboard.noRateLimit')" />
      </div>
    </section>
  </div>
</template>
