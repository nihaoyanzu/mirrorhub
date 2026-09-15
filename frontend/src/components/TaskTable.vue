<script setup lang="ts">
import EmptyState from '@/components/EmptyState.vue'
import ModalDialog from '@/components/ModalDialog.vue'
import PaginationBar from '@/components/PaginationBar.vue'
import { usePagination } from '@/composables/usePagination'
import { fmtBytes, fmtDurationMs } from '@/lib/format'
import { prioClass, prioKey, statusClass, statusKey } from '@/lib/status'
import { computed, ref, toRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { QueueTask } from '@/types/api'

const { t, locale } = useI18n()

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
  cancelMany: [ids: string[]]
}>()

const tasksRef = toRef(props, 'tasks')
const { page, size, total, pageCount, slice, go } = usePagination(tasksRef, props.pageSize)

const selected = ref<Set<string>>(new Set())
const cancelling = ref(false)
const showCancelModal = ref(false)
const headerCheck = ref<HTMLInputElement | null>(null)

const effectiveEmptyTitle = computed(() => props.emptyTitle || t('common.noData'))
const effectiveEmptyDesc = computed(() => props.emptyDescription || undefined)

function canCancel(task: QueueTask) {
  if (task.status !== 'running' && task.status !== 'queued') return false
  if (props.showPriority) return task.priority === 'P2' || task.priority === 'P1'
  return true
}

function taskTitle(task: QueueTask) {
  return task.label || task.url || '—'
}

function taskSub(task: QueueTask) {
  const label = task.label || ''
  const detail = task.detail || ''
  if (detail && detail !== label) return detail
  return ''
}

function progressPct(task: QueueTask): number | null {
  const totalBytes = Number(task.bytes_total || 0)
  if (totalBytes <= 0) return null
  if (task.status !== 'running' && task.status !== 'queued') {
    if (task.status === 'done') return 100
    return null
  }
  const done = Number(task.bytes_done || 0)
  return Math.min(100, Math.max(0, Math.round((done / totalBytes) * 100)))
}

function progressText(task: QueueTask) {
  const pct = progressPct(task)
  if (pct == null) return ''
  return t('queue.progress', {
    done: fmtBytes(Number(task.bytes_done || 0)),
    total: fmtBytes(Number(task.bytes_total || 0)),
    pct,
  })
}

const cancellableOnPage = computed(() => slice.value.filter(canCancel))
const selectedCancellable = computed(() =>
  props.tasks.filter((task) => selected.value.has(task.id) && canCancel(task)),
)

const allPageSelected = computed(() => {
  const list = cancellableOnPage.value
  return list.length > 0 && list.every((task) => selected.value.has(task.id))
})

const somePageSelected = computed(() => {
  const list = cancellableOnPage.value
  return list.some((task) => selected.value.has(task.id)) && !allPageSelected.value
})

watch(
  () => props.tasks.map((task) => task.id).join('\0'),
  () => {
    const alive = new Set(props.tasks.map((task) => task.id))
    selected.value = new Set([...selected.value].filter((id) => alive.has(id)))
  },
)

watch([somePageSelected, headerCheck], () => {
  if (headerCheck.value) headerCheck.value.indeterminate = somePageSelected.value
})

function toggleOne(id: string, on: boolean) {
  const next = new Set(selected.value)
  if (on) next.add(id)
  else next.delete(id)
  selected.value = next
}

function togglePage(on: boolean) {
  const next = new Set(selected.value)
  for (const task of cancellableOnPage.value) {
    if (on) next.add(task.id)
    else next.delete(task.id)
  }
  selected.value = next
}

function onHeaderCheck(ev: Event) {
  const el = ev.target as HTMLInputElement
  togglePage(el.checked)
}

function requestCancelSelected() {
  if (!selectedCancellable.value.length) return
  showCancelModal.value = true
}

async function confirmCancelSelected() {
  const ids = selectedCancellable.value.map((task) => task.id)
  if (!ids.length) return
  cancelling.value = true
  try {
    emit('cancelMany', ids)
    selected.value = new Set()
  } finally {
    cancelling.value = false
    showCancelModal.value = false
  }
}
</script>

<template>
  <div>
    <div
      v-if="selectedCancellable.length"
      class="flex flex-wrap items-center gap-2 border-b border-line px-4 py-2.5"
    >
      <span class="ui-badge-muted">{{ t('queue.selected', { n: selectedCancellable.length }) }}</span>
      <button
        type="button"
        class="ui-btn-danger !px-3 !py-1.5 text-xs"
        :disabled="cancelling"
        @click="requestCancelSelected"
      >
        {{ t('queue.cancelSelected') }}
      </button>
      <button type="button" class="ui-btn-ghost !px-2 !py-1 text-xs" @click="selected = new Set()">
        {{ t('queue.clearSelection') }}
      </button>
    </div>

    <div class="overflow-x-auto">
      <table v-if="slice.length" class="ui-table">
        <thead>
          <tr>
            <th class="w-10">
              <input
                ref="headerCheck"
                type="checkbox"
                class="accent-accent"
                :checked="allPageSelected"
                :disabled="!cancellableOnPage.length"
                :title="t('queue.selectAllPage')"
                @change="onHeaderCheck"
              />
            </th>
            <th v-if="showPriority">{{ t('status.p0Interactive').split(' ')[0] }}</th>
            <th>{{ t('queue.status') }}</th>
            <th>{{ t('dashboard.platform') }}</th>
            <th>{{ t('queue.task') }}</th>
            <th>{{ t('queue.wait') }}</th>
            <th>{{ t('queue.download') }}</th>
            <th class="w-24">{{ t('queue.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="task in slice" :key="task.id">
            <td>
              <input
                v-if="canCancel(task)"
                type="checkbox"
                class="accent-accent"
                :checked="selected.has(task.id)"
                @change="toggleOne(task.id, ($event.target as HTMLInputElement).checked)"
              />
            </td>
            <td v-if="showPriority">
              <span :class="prioClass(task.priority)">{{ t(prioKey(task.priority)) }}</span>
            </td>
            <td>
              <span :class="statusClass(task.status)">{{ t(statusKey(task.status)) }}</span>
            </td>
            <td>{{ task.platform }}</td>
            <td class="min-w-[220px] max-w-[420px]">
              <div class="truncate text-sm text-fg" :title="taskTitle(task)">{{ taskTitle(task) }}</div>
              <div v-if="taskSub(task)" class="mt-0.5 truncate font-mono text-xs text-muted" :title="taskSub(task)">
                {{ taskSub(task) }}
              </div>
              <div v-if="progressPct(task) != null" class="mt-1.5">
                <div class="h-1.5 overflow-hidden rounded bg-line">
                  <div
                    class="h-full rounded bg-accent transition-[width] duration-200"
                    :style="{ width: `${progressPct(task)}%` }"
                  />
                </div>
                <div class="mt-0.5 font-mono text-xs text-muted">{{ progressText(task) }}</div>
              </div>
              <div v-if="task.error" class="mt-1 truncate text-xs text-danger" :title="task.error">
                {{ task.error }}
              </div>
            </td>
            <td class="whitespace-nowrap font-mono text-xs text-muted">
              {{ fmtDurationMs(task.wait_ms, locale) }}
            </td>
            <td class="whitespace-nowrap font-mono text-xs text-muted">
              {{
                task.status === 'queued'
                  ? '-'
                  : fmtDurationMs(task.elapsed_ms, locale)
              }}
            </td>
            <td>
              <button
                v-if="canCancel(task)"
                type="button"
                class="ui-btn-danger !px-2 !py-1 text-xs"
                @click="emit('cancel', task.id)"
              >
                {{ t('queue.cancelAction') }}
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

    <ModalDialog
      v-model:visible="showCancelModal"
      :title="t('queue.cancelSelected')"
      :description="t('queue.cancelSelectedConfirm', { n: selectedCancellable.length })"
      :confirm-text="t('queue.cancelSelected')"
      danger
      @confirm="confirmCancelSelected"
      @cancel="showCancelModal = false"
    />
  </div>
</template>
