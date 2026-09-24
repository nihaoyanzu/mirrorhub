<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useToastStore, type ToastKind } from '@/stores/toast'

const { t } = useI18n()
const toast = useToastStore()

function kindLabel(kind: ToastKind): string {
  if (kind === 'ok') return t('toast.ok')
  if (kind === 'err') return t('toast.err')
  return t('toast.info')
}
</script>

<template>
  <div
    class="pointer-events-none fixed right-4 top-4 z-[200] flex w-[min(22rem,calc(100vw-2rem))] flex-col gap-2.5"
  >
    <div
      v-for="item in toast.items"
      :key="item.id"
      class="toast-item pointer-events-auto relative overflow-hidden text-fg"
      :class="'toast-item--' + item.kind"
    >
      <span class="toast-wash" aria-hidden="true" />
      <div class="relative flex items-start gap-3 px-3.5 py-3">
        <span class="toast-icon" aria-hidden="true">
          <svg v-if="item.kind === 'ok'" viewBox="0 0 20 20" class="h-4 w-4" fill="none">
            <path
              d="M5.6 10.3l2.7 2.7 6.1-6.4"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linecap="round"
              stroke-linejoin="round"
            />
          </svg>
          <svg v-else-if="item.kind === 'err'" viewBox="0 0 20 20" class="h-4 w-4" fill="none">
            <path
              d="M10 6.1v5M10 14.2h.01"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linecap="round"
            />
          </svg>
          <svg v-else viewBox="0 0 20 20" class="h-4 w-4" fill="none">
            <path
              d="M10 9.1v4.4M10 6.5h.01"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linecap="round"
            />
          </svg>
        </span>
        <div class="min-w-0 flex-1 pt-0.5">
          <div class="toast-kicker">{{ kindLabel(item.kind) }}</div>
          <div class="toast-msg">{{ item.message }}</div>
        </div>
        <button
          type="button"
          class="ui-btn-ghost !h-7 !w-7 shrink-0 !p-0 opacity-60 hover:opacity-100"
          :aria-label="t('common.close')"
          @click="toast.dismiss(item.id)"
        >
          <svg viewBox="0 0 20 20" fill="none" aria-hidden="true">
            <path
              d="M6 6l8 8M14 6l-8 8"
              stroke="currentColor"
              stroke-width="1.6"
              stroke-linecap="round"
            />
          </svg>
        </button>
      </div>
      <div class="toast-countdown" aria-hidden="true">
        <span class="toast-countdown-bar" :style="{ animationDuration: item.ms + 'ms' }" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.toast-item {
  --toast-accent: var(--color-accent);
  --toast-glow: color-mix(in srgb, var(--color-accent) 24%, transparent);
  border-radius: 0.75rem;
  border: 1px solid color-mix(in srgb, var(--color-line) 80%, transparent);
  background: color-mix(in srgb, var(--color-panel) 88%, transparent);
  backdrop-filter: blur(16px);
  box-shadow: var(--shadow-toast);
  animation: toast-in 0.42s cubic-bezier(0.16, 1, 0.3, 1);
}

.toast-item--ok {
  --toast-accent: var(--color-ok);
  --toast-glow: color-mix(in srgb, var(--color-ok) 26%, transparent);
}

.toast-item--err {
  --toast-accent: var(--color-danger);
  --toast-glow: color-mix(in srgb, var(--color-danger) 26%, transparent);
}

.toast-item--info {
  --toast-accent: var(--color-accent);
  --toast-glow: color-mix(in srgb, var(--color-accent) 26%, transparent);
}

.toast-wash {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: radial-gradient(180px 110px at 0% 0%, var(--toast-glow), transparent 72%);
}

.toast-icon {
  display: flex;
  height: 2.1rem;
  width: 2.1rem;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  border-radius: 0.6rem;
  color: var(--toast-accent);
  background: color-mix(in srgb, var(--toast-accent) 16%, transparent);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--toast-accent) 32%, transparent);
}

.toast-kicker {
  font-size: 0.68rem;
  font-weight: 600;
  letter-spacing: 0.08em;
  color: var(--toast-accent);
}

.toast-msg {
  margin-top: 0.1rem;
  font-size: 0.875rem;
  line-height: 1.45;
  color: var(--color-fg);
}

.toast-countdown {
  height: 2px;
  width: 100%;
  background: color-mix(in srgb, var(--color-line) 55%, transparent);
}

.toast-countdown-bar {
  display: block;
  height: 100%;
  width: 100%;
  background: linear-gradient(90deg, color-mix(in srgb, var(--toast-accent) 55%, transparent), var(--toast-accent));
  transform-origin: left center;
  animation-name: toast-countdown;
  animation-timing-function: linear;
  animation-fill-mode: forwards;
}

@keyframes toast-in {
  from {
    opacity: 0;
    transform: translateX(18px);
  }
  to {
    opacity: 1;
    transform: translateX(0);
  }
}

@keyframes toast-countdown {
  from {
    transform: scaleX(1);
  }
  to {
    transform: scaleX(0);
  }
}
</style>
