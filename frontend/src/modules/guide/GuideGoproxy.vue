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
    key: 'goproxyGlobal',
    title: t('guide.goproxyGlobal'),
    text: `go env -w GOPROXY=${props.baseURL},direct\n# 可选：私有模块仍走直连\n# go env -w GOPRIVATE=*.example.com\n# 完全离线（仅用已缓存）\n# go env -w GOPROXY=${props.baseURL},off`,
  },
  {
    key: 'goproxyLocal',
    title: t('guide.goproxyLocal'),
    text: `# 当前终端\nexport GOPROXY=${props.baseURL},direct\n# 可选：export GOPRIVATE=*.example.com\n\n# 单次命令\nGOPROXY=${props.baseURL},direct go mod download\n\n# 完全离线（仅用已缓存）\n# export GOPROXY=${props.baseURL},off`,
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
