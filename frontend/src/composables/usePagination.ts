import { computed, ref, watch, type Ref } from 'vue'

/** 客户端分页；source 变化时不强制回第 1 页（便于自动刷新），可用 reset() 或 pageCount 钳制 */
export function usePagination<T>(source: Ref<T[]>, pageSize = 15) {
  const page = ref(1)
  const size = ref(pageSize)

  const total = computed(() => source.value.length)
  const pageCount = computed(() => Math.max(1, Math.ceil(total.value / size.value) || 1))

  watch(pageCount, (n) => {
    if (page.value > n) page.value = n
  })

  const slice = computed(() => {
    const start = (page.value - 1) * size.value
    return source.value.slice(start, start + size.value)
  })

  function go(p: number) {
    page.value = Math.min(Math.max(1, p), pageCount.value)
  }

  function reset() {
    page.value = 1
  }

  return { page, size, total, pageCount, slice, go, reset }
}
