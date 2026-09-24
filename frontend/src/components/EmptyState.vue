<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

defineProps<{
  title?: string
  description?: string
}>()
</script>

<template>
  <div class="flex flex-col items-center justify-center gap-2 px-6 py-16 text-center">
    <div class="mb-1 h-10 w-10 rounded-full border border-dashed border-line bg-panel-2/50" />
    <div class="text-base font-medium text-fg">{{ title || t('common.noData') }}</div>
    <p v-if="description" class="max-w-sm text-sm text-muted">{{ description }}</p>
    <div v-if="$slots.default" class="mt-3">
      <slot />
    </div>
  </div>
</template>
