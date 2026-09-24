<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter, RouterLink } from 'vue-router'
import { api, setToken } from '@/api/client'
import { useSessionStore } from '@/stores/session'
import { useToastStore } from '@/stores/toast'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const session = useSessionStore()
const toast = useToastStore()

const username = ref('admin')
const password = ref('')
const showPwd = ref(false)
const error = ref('')
const loading = ref(false)

async function submit() {
  error.value = ''
  loading.value = true
  try {
    const res = await api.login(username.value.trim(), password.value)
    setToken(res.token)
    await session.refresh()
    toast.ok(t('login.welcome', { name: res.username }))
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/dashboard'
    await router.replace(redirect && redirect !== '/' ? redirect : '/dashboard')
  } catch (e: any) {
    error.value = e.message || t('login.loginFailed')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="relative flex min-h-screen items-center justify-center overflow-hidden px-4">
    <div class="pointer-events-none absolute inset-0">
      <div class="absolute -left-24 top-16 h-72 w-72 rounded-full bg-accent/10 blur-3xl" />
      <div
        class="absolute -right-16 bottom-10 h-80 w-80 rounded-full blur-3xl"
        style="background: var(--bg-glow-b)"
      />
    </div>

    <div class="relative w-full max-w-sm">
      <div class="mb-6 text-center">
        <div class="mb-3 flex justify-center">
          <img src="/favicon.svg" alt="MirrorHub" class="h-12 w-12" width="48" height="48" />
        </div>
        <div class="text-4xl font-semibold tracking-wide">
          mirror<span class="text-accent">hub</span>
        </div>
        <p class="mt-2 text-sm uppercase tracking-[0.18em] text-muted">{{ t('login.tagline') }}</p>
      </div>

      <form class="ui-panel p-7 sm:p-8" @submit.prevent="submit">
        <div class="space-y-4">
          <div>
            <label class="ui-label">{{ t('login.username') }}</label>
            <input v-model="username" class="ui-input" autocomplete="username" />
          </div>
          <div>
            <label class="ui-label">{{ t('login.password') }}</label>
            <div class="relative">
              <input
                v-model="password"
                class="ui-input pr-14"
                :type="showPwd ? 'text' : 'password'"
                autocomplete="current-password"
              />
              <button
                type="button"
                class="absolute right-2 top-1/2 -translate-y-1/2 ui-btn-ghost !py-1 text-xs"
                @click="showPwd = !showPwd"
              >
                {{ showPwd ? t('login.hide') : t('login.show') }}
              </button>
            </div>
          </div>
        </div>

        <button type="submit" class="ui-btn-primary mt-5 w-full" :disabled="loading || !password">
          {{ loading ? t('login.loggingIn') : t('login.loginBtn') }}
        </button>
        <p class="mt-4 text-center text-xs text-muted">
          <RouterLink class="text-accent hover:underline" to="/">{{ t('guide.title') }}</RouterLink>
        </p>
        <p v-if="error" class="mt-3 text-sm text-danger">{{ error }}</p>
      </form>
    </div>
  </div>
</template>
