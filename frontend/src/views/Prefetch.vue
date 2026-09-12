<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import PageHeader from '@/components/PageHeader.vue'
import TaskTable from '@/components/TaskTable.vue'
import { api } from '@/api/client'
import { useToastStore } from '@/stores/toast'
import { useQueueStore } from '@/stores/queue'
import type { QueueTask } from '@/types/api'

const { t } = useI18n()
const toast = useToastStore()
const queueStore = useQueueStore()

const text = ref('')
const submitting = ref(false)
const lastSkipped = ref<string[]>([])

const prefetchTasks = computed(() =>
  queueStore.tasks.filter((t: QueueTask) => t.priority === 'P2' || t.priority === 'P1'),
)

const lineHint = computed(() => {
  const n = text.value.split(/\r?\n/).filter((l) => l.trim() && !l.trim().startsWith('#')).length
  return n > 1 ? t('prefetch.lines', { n }) : ''
})

async function submit() {
  const content = text.value.trim()
  if (!content) {
    toast.err(t('prefetch.emptyInput'))
    return
  }
  submitting.value = true
  lastSkipped.value = []
  try {
    const res = await api.prefetch([], content)
    lastSkipped.value = res.skipped || []
    const skipTip = lastSkipped.value.length ? t('prefetch.skipped', { n: lastSkipped.value.length }) : ''
    toast.ok(t('prefetch.submitted', { n: res.enqueued }) + skipTip)
    text.value = ''
    await queueStore.load(true)
  } catch (e: any) {
    toast.err(e.message || t('prefetch.submitFailed'))
  } finally {
    submitting.value = false
  }
}

async function cancel(id: string) {
  try {
    await queueStore.cancel(id)
    toast.ok(t('queue.cancelledOk'))
    await queueStore.load(true)
  } catch (e: any) {
    toast.err(e.message || t('queue.cancelFailed'))
  }
}

onMounted(() => queueStore.startPolling(4000))
onUnmounted(() => queueStore.stopPolling())
</script>

<template>
  <div>
    <PageHeader :title="t('prefetch.title')" :description="t('prefetch.subtitle')">
      <template #actions>
        <RouterLink class="ui-btn" to="/platform?tab=prefetch">{{ t('prefetch.settingsLink') }}</RouterLink>
      </template>
    </PageHeader>

    <section class="ui-panel mb-5 p-4">
      <div class="mb-2 flex items-center justify-between gap-2">
        <span class="ui-section-title !mb-0">{{ t('prefetch.submitDeps') }}</span>
        <span v-if="lineHint" class="ui-badge-muted">{{ lineHint }}</span>
      </div>
      <p class="mb-3 text-xs text-muted">{{ t('prefetch.hint') }}</p>
      <form class="space-y-3" @submit.prevent="submit">
        <textarea
          v-model="text"
          rows="6"
          class="ui-input font-mono text-sm"
          :placeholder="t('prefetch.placeholder')"
        />
        <div class="flex flex-wrap gap-2">
          <button type="submit" class="ui-btn-primary" :disabled="submitting">
            {{ submitting ? t('prefetch.submitting') : t('common.submit') }}
          </button>
          <button type="button" class="ui-btn" :disabled="!text" @click="text = ''">{{ t('common.clear') }}</button>
        </div>
        <p v-if="lastSkipped.length" class="text-xs text-muted">
          {{ t('prefetch.skippedLabel') }}{{ lastSkipped.slice(0, 5).join(' · ') }}
          <span v-if="lastSkipped.length > 5"> 等 {{ lastSkipped.length }} 项</span>
        </p>
      </form>
    </section>

    <section class="ui-panel overflow-hidden">
      <div class="border-b border-line px-4 py-2.5">
        <h2 class="ui-section-title !mb-0">{{ t('prefetch.taskList') }}</h2>
      </div>
      <TaskTable
        :tasks="prefetchTasks"
        :loading="queueStore.loading"
        :show-priority="true"
        :empty-title="t('prefetch.noTasks')"
        @cancel="cancel"
      />
    </section>
  </div>
</template>
