<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import GuideSnippetList from './GuideSnippetList.vue'
import type { GuideSnippet } from './types'

const props = defineProps<{
  baseURL: string
  displayHost: string
  copied: string
}>()

const emit = defineEmits<{
  copy: [key: string, text: string]
}>()

const { t } = useI18n()

const snippets = computed<GuideSnippet[]>(() => [
  {
    key: 'dockerDaemon',
    title: t('guide.dockerDaemon'),
    text: `{\n  "registry-mirrors": ["${props.baseURL}"],\n  "insecure-registries": ["${props.displayHost}"]\n}`,
  },
  {
    key: 'dockerPull',
    title: t('guide.dockerPull'),
    text: `# 重启 dockerd 后正常 pull，经 MirrorHub 自动灌缓存\ndocker pull nginx:1.27`,
  },
  {
    key: 'dockerOffline',
    title: t('guide.dockerOffline'),
    text: t('guide.dockerOfflineBody'),
    hideCopy: true,
  },
])
</script>

<template>
  <section class="ui-panel overflow-hidden">
    <div class="border-b border-line p-4 sm:p-5">
      <p class="text-xs text-muted">{{ t('guide.dockerHint') }}</p>
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
