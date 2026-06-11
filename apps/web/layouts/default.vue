<script setup lang="ts">
import FlowTopBar from '~/components/layout/FlowTopBar.vue'

const route = useRoute()
const items = [
  { label: 'Dashboard', to: '/' },
  { label: 'Workflows', to: '/workflows' },
  { label: 'Approvals', to: '/approvals' },
  { label: 'Assets', to: '/assets' }
]
</script>

<template>
  <div class="min-h-screen bg-[color:var(--rf-bg)] text-[color:var(--rf-text)]">
    <div class="mx-auto flex min-h-screen max-w-[1600px] flex-col lg:flex-row">
      <aside class="border-b border-[color:var(--rf-border)] bg-[color:var(--rf-surface)] px-4 py-4 lg:min-h-screen lg:w-72 lg:border-b-0 lg:border-r">
        <div class="mb-6">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Ralleh</div>
          <h1 class="mt-2 text-2xl font-semibold">Flow</h1>
          <p class="mt-2 text-sm text-[color:var(--rf-muted)]">Git-native orchestration control room</p>
        </div>
        <nav class="space-y-2">
          <NuxtLink
            v-for="item in items"
            :key="item.to"
            :to="item.to"
            class="block rounded-xl border px-3 py-2 text-sm transition"
            :class="route.path === item.to ? 'border-cyan-400/40 bg-cyan-500/10 text-white' : 'border-transparent text-[color:var(--rf-muted)] hover:border-[color:var(--rf-border)] hover:bg-white/5 hover:text-white'"
          >
            {{ item.label }}
          </NuxtLink>
        </nav>
      </aside>

      <div class="flex min-w-0 flex-1 flex-col">
        <FlowTopBar />
        <main class="flex-1 px-4 py-4 lg:px-6 lg:py-6">
          <slot />
        </main>
      </div>
    </div>
  </div>
</template>
