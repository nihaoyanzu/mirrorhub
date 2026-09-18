<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import AppShell from './AppShell.vue'
import ToastHost from '@/components/ToastHost.vue'

const route = useRoute()
const routeReady = computed(() => route.matched.length > 0)
const isPublic = computed(() => !!route.meta.public)
</script>

<template>
  <template v-if="routeReady">
    <RouterView v-if="isPublic" />
    <AppShell v-else />
  </template>
  <ToastHost />
</template>
