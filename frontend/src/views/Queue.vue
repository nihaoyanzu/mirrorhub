<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import PageHeader from '@/components/PageHeader.vue'
import TaskTable from '@/components/TaskTable.vue'
import { useQueueStore } from '@/stores/queue'
import { useToastStore } from '@/stores/toast'
import type { QueueTask } from '@/types/api'

const { t } = useI18n()
const toast = useToastStore()
const queueStore = useQueueStore()

const filter = defineModel<'all' | 'running' | 'done' | 'error' | 'cancelled'>('filter', { default: 'all' })

const filters = computed(() => [
  { id: 'all' as const, label: t('queue.all') },
  { id: 'running' as const, label: t('queue.running') },
  { id: 'done' as const, label: t('queue.done') },
  { id: 'error' as const, label: t('queue.error') },
  { id: 'cancelled' as const, label: t('queue.cancelled') },
])

const filtered = computed(() => {
  if (filter.value === 'all') return queueStore.tasks
  return queueStore.tasks.filter((t: QueueTask) => t.status === filter.value)
})

onMounted(() => queueStore.startPolling(3000))
onUnmounted(() => queueStore.stopPolling())

async function cancel(id: string) {
  try {
    await queueStore.cancel(id)
    toast.ok(t('queue.cancelledOk'))
  } catch (e: any) {
    toast.err(e.message || t('queue.cancelFailed'))
  }
}

async function clearFinished() {
  try {
    const n = await queueStore.clearFinished()
    toast.ok(t('queue.clearedOk', { n }))
  } catch (e: any) {
    toast.err(e.message || t('queue.clearFailed'))
  }
}
</script>

<template>
  <div>
    <PageHeader :title="t('queue.title')" :description="t('queue.subtitle')">
      <template #actions>
        <span class="ui-badge-muted">{{ t('queue.running') }} {{ queueStore.runningCount }}</span>
        <button type="button" class="ui-btn" @click="clearFinished">{{ t('queue.clearFinished') }}</button>
        <span class="h-4 w-px bg-line" />
        <button
          v-for="f in filters"
          :key="f.id"
          type="button"
          class="ui-chip"
          :class="{ 'ui-chip-active': filter === f.id }"
          @click="filter = f.id"
        >
          {{ f.label }}
        </button>
      </template>
    </PageHeader>

    <section class="ui-panel overflow-hidden">
      <TaskTable
        :key="filter"
        :tasks="filtered"
        :loading="queueStore.loading"
        :show-priority="true"
        :empty-title="t('queue.queueIdle')"
        @cancel="cancel"
      />
    </section>
  </div>
</template>
