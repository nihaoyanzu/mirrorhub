<script setup lang="ts">
defineProps<{
  label: string
  value: string | number
  hint?: string
  tone?: 'default' | 'ok' | 'warn' | 'danger'
  /** 与顶栏一致的流量方向图标 */
  icon?: 'pull' | 'share'
}>()
</script>

<template>
  <div class="ui-panel relative overflow-hidden p-5">
    <div
      class="pointer-events-none absolute inset-x-0 top-0 h-0.5"
      :class="{
        'bg-line': !tone || tone === 'default',
        'bg-ok': tone === 'ok',
        'bg-warn': tone === 'warn',
        'bg-danger': tone === 'danger',
      }"
    />
    <div class="flex items-center gap-1.5 text-sm font-medium uppercase tracking-wide text-muted">
      <span>{{ label }}</span>
      <svg
        v-if="icon === 'pull'"
        class="h-4 w-4 text-accent"
        viewBox="0 0 20 20"
        fill="currentColor"
        aria-hidden="true"
      >
        <path d="M10 3a1 1 0 011 1v7.586l2.293-2.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L10 11.586V4a1 1 0 011-1z" />
        <path d="M4 15a1 1 0 011-1h10a1 1 0 110 2H5a1 1 0 01-1-1z" />
      </svg>
      <svg
        v-else-if="icon === 'share'"
        class="h-4 w-4 text-ok"
        viewBox="0 0 20 20"
        fill="currentColor"
        aria-hidden="true"
      >
        <path d="M10 17a1 1 0 01-1-1V9.414l-2.293 2.293a1 1 0 01-1.414-1.414l4-4a1 1 0 011.414 0l4 4a1 1 0 01-1.414 1.414L10 9.414V16a1 1 0 01-1 1z" />
        <path d="M4 5a1 1 0 011-1h10a1 1 0 110 2H5a1 1 0 01-1-1z" />
      </svg>
    </div>
    <div class="mt-2.5 font-mono text-3xl tracking-tight text-fg">{{ value }}</div>
    <div v-if="hint" class="mt-1.5 text-sm text-muted">{{ hint }}</div>
  </div>
</template>
