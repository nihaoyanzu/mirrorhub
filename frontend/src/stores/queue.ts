import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { api } from '@/api/client'
import type { QueueTask } from '@/types/api'

export const useQueueStore = defineStore('queue', () => {
  const tasks = ref<QueueTask[]>([])
  const loading = ref(false)
  const auto = ref(true)
  let timer: number | undefined

  const runningCount = computed(() => tasks.value.filter((t) => t.status === 'running').length)

  async function load(silent = false) {
    if (!silent) loading.value = true
    try {
      const res = await api.getQueue()
      tasks.value = res.tasks || []
    } catch {
      // silent
    } finally {
      if (!silent) loading.value = false
    }
  }

  async function cancel(id: string) {
    await api.cancelPrefetch(id)
    await load(true)
  }

  async function cancelMany(ids: string[]) {
    const uniq = [...new Set(ids.filter(Boolean))]
    for (const id of uniq) {
      await api.cancelPrefetch(id)
    }
    await load(true)
  }

  async function clearFinished() {
    const res = await api.clearQueue()
    await load(true)
    return res.cleared ?? 0
  }

  function startPolling(intervalMs = 3000) {
    stopPolling()
    auto.value = true
    load()
    timer = window.setInterval(() => {
      if (auto.value) load(true)
    }, intervalMs)
  }

  function stopPolling() {
    if (timer) {
      clearInterval(timer)
      timer = undefined
    }
  }

  return {
    tasks,
    loading,
    auto,
    runningCount,
    load,
    cancel,
    cancelMany,
    clearFinished,
    startPolling,
    stopPolling,
  }
})
