<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

defineProps<{
  title?: string
  description?: string
}>()
</script>

<template>
  <div class="flex flex-col items-center justify-center gap-2 px-6 py-14 text-center">
    <div class="text-base font-medium text-fg">{{ title || t('common.noData') }}</div>
    <p v-if="description" class="max-w-sm text-base text-muted">{{ description }}</p>
    <div v-if="$slots.default" class="mt-2">
      <slot />
    </div>
  </div>
</template>
