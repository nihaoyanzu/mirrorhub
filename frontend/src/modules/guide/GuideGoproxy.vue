<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import GuideSnippetList from './GuideSnippetList.vue'
import type { GuideSnippet } from './types'

const props = defineProps<{
  baseURL: string
  copied: string
}>()

const emit = defineEmits<{
  copy: [key: string, text: string]
}>()

const { t } = useI18n()

const snippets = computed<GuideSnippet[]>(() => [
  {
    key: 'goproxyEnv',
    title: t('guide.goproxyEnv'),
    text: `export GOPROXY=${props.baseURL},direct\n# 可选：私有模块仍走直连\n# export GOPRIVATE=*.example.com`,
  },
  {
    key: 'goproxyOffline',
    title: t('guide.goproxyOffline'),
    text: `# 完全离线（仅用已缓存；勿加 ,direct）\nexport GOPROXY=${props.baseURL},off\ngo mod download`,
  },
  {
    key: 'goproxyNote',
    title: t('guide.goproxyNote'),
    text: t('guide.goproxyNoteBody'),
    hideCopy: true,
  },
])
</script>

<template>
  <section class="ui-panel overflow-hidden">
    <div class="border-b border-line p-4 sm:p-5">
      <p class="text-xs text-muted">{{ t('guide.goproxyHint') }}</p>
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
