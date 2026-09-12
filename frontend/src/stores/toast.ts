import { defineStore } from 'pinia'
import { ref } from 'vue'

export type ToastKind = 'ok' | 'err' | 'info'

export interface ToastItem {
  id: number
  kind: ToastKind
  message: string
}

export const useToastStore = defineStore('toast', () => {
  const items = ref<ToastItem[]>([])
  let seq = 1

  function push(message: string, kind: ToastKind = 'info', ms = 2800) {
    const id = seq++
    items.value.push({ id, kind, message })
    window.setTimeout(() => dismiss(id), ms)
  }

  function dismiss(id: number) {
    items.value = items.value.filter((t) => t.id !== id)
  }

  function ok(message: string) {
    push(message, 'ok')
  }

  function err(message: string) {
    push(message, 'err', 4000)
  }

  function info(message: string) {
    push(message, 'info')
  }

  return { items, push, dismiss, ok, err, info }
})
