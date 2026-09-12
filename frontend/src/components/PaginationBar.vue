<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = withDefaults(
  defineProps<{
    page: number
    pageCount: number
    total: number
    pageSize?: number
  }>(),
  { pageSize: 15 },
)

const emit = defineEmits<{
  'update:page': [n: number]
}>()

const hint = computed(() => {
  if (props.total <= 0) return t('pagination.total', { n: 0 })
  const start = (props.page - 1) * props.pageSize + 1
  const end = Math.min(props.page * props.pageSize, props.total)
  return t('pagination.range', { start, end, total: props.total })
})

function go(p: number) {
  if (p < 1 || p > props.pageCount || p === props.page) return
  emit('update:page', p)
}
</script>

<template>
  <div
    v-if="total > 0"
    class="flex flex-wrap items-center justify-between gap-3 border-t border-line px-4 py-3.5 text-sm text-muted"
  >
    <span>{{ hint }}</span>
    <div v-if="pageCount > 1" class="flex items-center gap-1.5">
      <button class="ui-btn !px-2 !py-1 text-xs" :disabled="page <= 1" @click="go(page - 1)">
        {{ t('pagination.prev') }}
      </button>
      <span class="min-w-[4.5rem] text-center font-mono text-fg">{{ page }} / {{ pageCount }}</span>
      <button
        class="ui-btn !px-2 !py-1 text-xs"
        :disabled="page >= pageCount"
        @click="go(page + 1)"
      >
        {{ t('pagination.next') }}
      </button>
    </div>
  </div>
</template>
