<script setup lang="ts">
import FlowTopBar from '~/components/layout/FlowTopBar.vue'

const route = useRoute()
const config = useRuntimeConfig()

const items = [
  {
    label: 'Cockpit',
    to: '/',
    detail: 'What needs attention right now'
  },
  {
    label: 'Workflows',
    to: '/workflows',
    detail: 'Operational packages and launch posture'
  },
  {
    label: 'Approvals',
    to: '/approvals',
    detail: 'Governance queue and intervention moments'
  },
  {
    label: 'Assets',
    to: '/assets',
    detail: 'Resource intelligence and provenance stance'
  }
]

const isActive = (to: string) => {
  if (to === '/') return route.path === '/'
  return route.path === to || route.path.startsWith(`${to}/`)
}

const appName = computed(() => config.public.appName || 'Ralleh Flow')
</script>

<template>
  <div class="min-h-screen bg-[color:var(--rf-bg)] text-[color:var(--rf-text)]">
    <div class="mx-auto flex min-h-screen max-w-[1600px] flex-col lg:flex-row">
      <aside class="border-b border-[color:var(--rf-border)] bg-[color:var(--rf-surface)] px-4 py-4 lg:min-h-screen lg:w-80 lg:border-b-0 lg:border-r lg:px-5 lg:py-5">
        <div class="rounded-[1.75rem] border border-[color:var(--rf-border)] bg-black/10 p-5">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Mission control shell</div>
          <h1 class="mt-2 text-2xl font-semibold">{{ appName }}</h1>
          <p class="mt-3 text-sm text-[color:var(--rf-muted)]">
            Command surface for autonomous work: attention first, live operations second, history only when it changes the next move.
          </p>
        </div>

        <nav class="mt-5 space-y-3">
          <NuxtLink
            v-for="item in items"
            :key="item.to"
            :to="item.to"
            class="rf-nav-link"
            :class="isActive(item.to) ? 'rf-nav-link--active' : ''"
          >
            <div class="flex items-center justify-between gap-3">
              <span class="font-medium">{{ item.label }}</span>
              <span v-if="isActive(item.to)" class="rf-badge">active</span>
            </div>
            <p class="mt-2 text-sm text-[color:var(--rf-muted)]">{{ item.detail }}</p>
          </NuxtLink>
        </nav>

        <section class="mt-5 rounded-[1.5rem] border border-[color:var(--rf-border)] bg-black/10 p-4">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Shell doctrine</div>
          <ul class="mt-3 space-y-3 text-sm text-[color:var(--rf-muted)]">
            <li>Use the cockpit to triage.</li>
            <li>Use workflow packages to launch deliberately.</li>
            <li>Use mission views when Git, worker, and approval truth matter.</li>
          </ul>
        </section>
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
