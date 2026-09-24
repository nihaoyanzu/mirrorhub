<script setup lang="ts">
defineProps<{
  title: string
  description?: string
  eyebrow?: string
}>()
</script>

<template>
  <div class="page-header flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
    <div class="min-w-0">
      <p v-if="eyebrow" class="mb-1 text-xs font-semibold uppercase tracking-[0.08em] text-muted">
        {{ eyebrow }}
      </p>
      <h1 class="text-2xl font-semibold tracking-tight text-fg">{{ title }}</h1>
      <p v-if="description" class="mt-1.5 text-base text-muted">{{ description }}</p>
    </div>
    <div v-if="$slots.actions" class="flex shrink-0 flex-wrap items-center gap-2">
      <slot name="actions" />
    </div>
  </div>
</template>
