import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { api } from '@/api/client'
import { MODULES, MODULE_BY_ID } from '@/modules/registry'

export const useModulesStore = defineStore('modules', () => {
  /** moduleId → enabled；未加载时按 guideDefaultEnabled 兜底 */
  const enabledMap = ref<Record<string, boolean>>(
    Object.fromEntries(MODULES.map((d) => [d.id, d.guideDefaultEnabled])),
  )
  const loaded = ref(false)

  const enabledIds = computed(() => MODULES.filter((d) => enabledMap.value[d.id]).map((d) => d.id))

  function isEnabled(id: string): boolean {
    return !!enabledMap.value[id]
  }

  async function refresh() {
    try {
      const cfg = await api.getConfig()
      const next: Record<string, boolean> = {}
      for (const d of MODULES) {
        const p = cfg.platforms?.[d.id]
        next[d.id] = p ? !!p.enabled : d.guideDefaultEnabled
      }
      enabledMap.value = next
    } catch {
      /* 保持上次 / 兜底 */
    } finally {
      loaded.value = true
    }
  }

  function firstCatalogModule(): string | null {
    for (const d of MODULES) {
      if (d.nav.catalog && enabledMap.value[d.id]) return d.id
    }
    return null
  }

  function firstPrefetchModule(): string | null {
    for (const d of MODULES) {
      if (d.nav.prefetch && enabledMap.value[d.id]) return d.id
    }
    return null
  }

  function catalogMode(id: string) {
    return MODULE_BY_ID[id]?.catalogMode || 'local'
  }

  return {
    enabledMap,
    enabledIds,
    loaded,
    isEnabled,
    refresh,
    firstCatalogModule,
    firstPrefetchModule,
    catalogMode,
  }
})
