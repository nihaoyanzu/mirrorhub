<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

defineProps<{
  visible: boolean
  title?: string
  description?: string
  confirmText?: string
  cancelText?: string
  danger?: boolean
  /** 仅关闭按钮（结果展示等） */
  hideConfirm?: boolean
  wide?: boolean
}>()

const emit = defineEmits<{
  confirm: []
  cancel: []
  'update:visible': [value: boolean]
}>()

function onCancel() {
  emit('update:visible', false)
  emit('cancel')
}

function onConfirm() {
  emit('update:visible', false)
  emit('confirm')
}
</script>

<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="visible"
        class="fixed inset-0 z-[100] flex items-center justify-center p-4"
        @keydown.escape="onCancel"
      >
        <div class="absolute inset-0 bg-black/60 backdrop-blur-sm" @click="onCancel" />

        <div
          class="ui-panel relative w-full p-6 shadow-2xl"
          :class="wide ? 'max-w-xl' : 'max-w-md'"
        >
          <h3 v-if="title" class="mb-2 text-lg font-semibold text-fg">{{ title }}</h3>
          <p v-if="description" class="mb-4 text-sm text-muted">{{ description }}</p>
          <slot />
          <div class="mt-6 flex justify-end gap-3">
            <button class="ui-btn" @click="onCancel">
              {{ cancelText || (hideConfirm ? t('common.close') : t('common.cancel')) }}
            </button>
            <button
              v-if="!hideConfirm"
              :class="danger ? 'ui-btn-danger' : 'ui-btn-primary'"
              @click="onConfirm"
            >
              {{ confirmText || t('common.confirm') }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}
.modal-enter-active .ui-panel,
.modal-leave-active .ui-panel {
  transition: transform 0.2s ease, opacity 0.2s ease;
}
.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
.modal-enter-from .ui-panel,
.modal-leave-to .ui-panel {
  transform: scale(0.95);
  opacity: 0;
}
</style>
