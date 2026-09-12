<script setup lang="ts">
import EmptyState from '@/components/EmptyState.vue'
import PaginationBar from '@/components/PaginationBar.vue'
import { usePagination } from '@/composables/usePagination'
import { fmtTime } from '@/lib/format'
import { prioClass, prioKey, statusClass, statusKey } from '@/lib/status'
import { computed, toRef } from 'vue'
import { useI18n } from 'vue-i18n'
import type { QueueTask } from '@/types/api'

const { t } = useI18n()

const props = withDefaults(
  defineProps<{
    tasks: QueueTask[]
    loading?: boolean
    showPriority?: boolean
    emptyTitle?: string
    emptyDescription?: string
    pageSize?: number
  }>(),
  {
    loading: false,
    showPriority: true,
    emptyTitle: '',
    emptyDescription: '',
    pageSize: 15,
  },
)

const emit = defineEmits<{
  cancel: [id: string]
}>()

const tasksRef = toRef(props, 'tasks')
const { page, size, total, pageCount, slice, go } = usePagination(tasksRef, props.pageSize)

const effectiveEmptyTitle = computed(() => props.emptyTitle || t('common.noData'))
const effectiveEmptyDesc = computed(() => props.emptyDescription || undefined)

function canCancel(task: QueueTask) {
  if (task.status !== 'running' && task.status !== 'queued') return false
  if (props.showPriority) return task.priority === 'P2' || task.priority === 'P1'
  return true
}
</script>

<template>
  <div>
    <div class="overflow-x-auto">
      <table v-if="slice.length" class="ui-table">
        <thead>
          <tr>
            <th v-if="showPriority">{{ t('status.p0Interactive').split(' ')[0] }}</th>
            <th>{{ t('queue.running') }}</th>
            <th>{{ t('dashboard.platform') }}</th>
            <th>{{ t('dashboard.path') }}</th>
            <th>{{ t('dashboard.wait') }}</th>
            <th>{{ t('dashboard.time') }}</th>
            <th class="w-24"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="task in slice" :key="task.id">
            <td v-if="showPriority">
              <span :class="prioClass(task.priority)">{{ t(prioKey(task.priority)) }}</span>
            </td>
            <td>
              <span :class="statusClass(task.status)">{{ t(statusKey(task.status)) }}</span>
            </td>
            <td>{{ task.platform }}</td>
            <td class="max-w-[420px]">
              <div class="truncate font-mono text-xs text-muted" :title="task.url">{{ task.url }}</div>
              <div v-if="task.error" class="mt-1 truncate text-xs text-danger" :title="task.error">
                {{ task.error }}
              </div>
            </td>
            <td class="whitespace-nowrap font-mono text-xs text-muted">
              {{ task.wait_ms != null ? `${task.wait_ms} ms` : '-' }}
            </td>
            <td class="whitespace-nowrap text-xs text-muted">{{ fmtTime(task.updated_at) }}</td>
            <td>
              <button
                v-if="canCancel(task)"
                class="ui-btn-danger !px-2 !py-1 text-xs"
                @click="emit('cancel', task.id)"
              >
                {{ t('queue.cancelled') }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
      <EmptyState
        v-else
        :title="loading ? t('common.loading') : effectiveEmptyTitle"
        :description="effectiveEmptyDesc"
      />
    </div>
    <PaginationBar
      :page="page"
      :page-count="pageCount"
      :total="total"
      :page-size="size"
      @update:page="go"
    />
  </div>
</template>
