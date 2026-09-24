<script setup lang="ts">
import type { GuideSnippet } from './types'

defineProps<{
  snippets: GuideSnippet[]
  copied: string
  copyLabel: string
  copiedLabel: string
}>()

const emit = defineEmits<{
  copy: [key: string, text: string]
}>()
</script>

<template>
  <div class="grid gap-3 p-4 sm:p-5">
    <article
      v-for="item in snippets"
      :key="item.key"
      class="rounded-xl border border-line bg-bg/40 p-3 sm:p-4"
    >
      <div class="mb-2 flex items-center justify-between gap-2">
        <h3 class="text-sm font-medium text-fg">{{ item.title }}</h3>
        <button
          v-if="!item.hideCopy"
          type="button"
          class="ui-btn-ghost !px-2 !py-1 text-xs"
          @click="emit('copy', item.key, item.text)"
        >
          {{ copied === item.key ? copiedLabel : copyLabel }}
        </button>
      </div>
      <pre
        class="overflow-x-auto whitespace-pre-wrap break-all font-mono text-xs leading-relaxed text-muted"
      >{{ item.text }}</pre>
    </article>
  </div>
</template>
