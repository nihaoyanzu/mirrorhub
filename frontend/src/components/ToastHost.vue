<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useToastStore } from '@/stores/toast'

const { t } = useI18n()
const toast = useToastStore()
</script>

<template>
  <div class="pointer-events-none fixed right-4 top-4 z-[200] flex w-[min(360px,calc(100vw-2rem))] flex-col gap-2">
    <div
      v-for="item in toast.items"
      :key="item.id"
      class="pointer-events-auto flex items-start gap-3 rounded-lg border px-4 py-3.5 text-base shadow-lg backdrop-blur-md transition"
      :class="{
        'border-ok/30 bg-panel/95 text-ok': item.kind === 'ok',
        'border-danger/30 bg-panel/95 text-danger': item.kind === 'err',
        'border-line bg-panel/95 text-fg': item.kind === 'info',
      }"
    >
      <span class="min-w-0 flex-1 leading-snug">{{ item.message }}</span>
      <button class="ui-btn-ghost !px-1.5 !py-0.5 text-xs" @click="toast.dismiss(item.id)">{{ t('common.close') }}</button>
    </div>
  </div>
</template>
