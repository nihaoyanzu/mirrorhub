import { defineStore } from 'pinia'
import { ref } from 'vue'
import { api, clearToken, getToken } from '@/api/client'

export const useSessionStore = defineStore('session', () => {
  const username = ref('')
  const loaded = ref(false)

  async function refresh() {
    if (!getToken()) {
      username.value = ''
      loaded.value = true
      return
    }
    try {
      const me = await api.me()
      username.value = me.username
    } catch {
      username.value = ''
      clearToken()
    } finally {
      loaded.value = true
    }
  }

  function reset() {
    username.value = ''
    clearToken()
  }

  return { username, loaded, refresh, reset }
})
