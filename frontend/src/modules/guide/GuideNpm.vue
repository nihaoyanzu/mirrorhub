<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import GuideSnippetList from './GuideSnippetList.vue'
import type { GuideSnippet } from './types'

const props = defineProps<{
  registryURL: string
  copied: string
}>()

const emit = defineEmits<{
  copy: [key: string, text: string]
}>()

const { t } = useI18n()
const npmToolTab = ref<'npm' | 'pnpm' | 'yarn'>('npm')

const npmSnippets = computed<GuideSnippet[]>(() => [
  {
    key: 'npmOnce',
    title: t('guide.npmOnce'),
    text: `npm config set registry ${props.registryURL}`,
  },
  {
    key: 'npmConfig',
    title: t('guide.npmConfig'),
    text: `# .npmrc\nregistry=${props.registryURL}`,
  },
])

const pnpmSnippets = computed<GuideSnippet[]>(() => [
  {
    key: 'pnpmConfig',
    title: t('guide.pnpmConfig'),
    text: `pnpm config set registry ${props.registryURL}`,
  },
])

const yarnSnippets = computed<GuideSnippet[]>(() => [
  {
    key: 'yarnConfig',
    title: t('guide.yarnConfig'),
    text: `yarn config set registry ${props.registryURL}`,
  },
])

const snippets = computed(() => {
  if (npmToolTab.value === 'pnpm') return pnpmSnippets.value
  if (npmToolTab.value === 'yarn') return yarnSnippets.value
  return npmSnippets.value
})
</script>

<template>
  <section class="ui-panel overflow-hidden">
    <div class="flex flex-wrap items-center justify-between gap-3 border-b border-line p-4 sm:p-5">
      <p class="text-xs text-muted">{{ t('guide.npmHint') }}</p>
      <div class="flex gap-1 rounded-xl border border-line bg-bg/60 p-1">
        <button
          type="button"
          class="ui-chip"
          :class="{ 'ui-chip-active': npmToolTab === 'npm' }"
          @click="npmToolTab = 'npm'"
        >
          npm
        </button>
        <button
          type="button"
          class="ui-chip"
          :class="{ 'ui-chip-active': npmToolTab === 'pnpm' }"
          @click="npmToolTab = 'pnpm'"
        >
          pnpm
        </button>
        <button
          type="button"
          class="ui-chip"
          :class="{ 'ui-chip-active': npmToolTab === 'yarn' }"
          @click="npmToolTab = 'yarn'"
        >
          yarn
        </button>
      </div>
    </div>
    <GuideSnippetList
      :snippets="snippets"
      :copied="copied"
      :copy-label="t('guide.copy')"
      :copied-label="t('guide.copied')"
      @copy="(k, text) => emit('copy', k, text)"
    />
  </section>
</template>
