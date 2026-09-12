import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api } from '@/api/client'
import type { SystemStats } from '@/types/api'

export const useStatsStore = defineStore('stats', () => {
  const data = ref<SystemStats | null>(null)
  const loading = ref(false)
  const error = ref('')
  let timer: number | undefined
  const auto = ref(true)

  async function load(silent = false) {
    if (!silent) loading.value = true
    try {
      data.value = await api.getStats()
      error.value = ''
    } catch (e: any) {
      if (!silent) error.value = e.message || 'Failed to load stats'
    } finally {
      if (!silent) loading.value = false
    }
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

  function setAuto(v: boolean) {
    auto.value = v
  }

  return { data, loading, error, auto, load, startPolling, stopPolling, setAuto }
})
