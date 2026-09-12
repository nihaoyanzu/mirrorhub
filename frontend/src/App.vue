<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import AppShell from './AppShell.vue'
import ToastHost from '@/components/ToastHost.vue'

const route = useRoute()
const isPublic = computed(() => !!route.meta.public)
</script>

<template>
  <RouterView v-if="isPublic" />
  <AppShell v-else />
  <ToastHost />
</template>
