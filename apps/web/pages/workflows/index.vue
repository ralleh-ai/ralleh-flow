<script setup lang="ts">
const api = useFlowApi()

const { data, pending, error, refresh } = await useAsyncData('flow-workflows', async () => {
  const [workflows, runs] = await Promise.all([api.getWorkflows(), api.getRuns()])

  const runCounts = runs.reduce<Record<string, number>>((acc, run) => {
    acc[run.workflowId] = (acc[run.workflowId] || 0) + 1
    return acc
  }, {})

  return { workflows, runCounts }
})

const workflows = computed(() => data.value?.workflows ?? [])
</script>

<template>
  <div class="space-y-6">
    <section class="rf-card">
      <div class="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Workflow catalog</div>
          <h1 class="mt-2 text-3xl font-semibold">Operational packages</h1>
          <p class="mt-2 text-sm text-[color:var(--rf-muted)]">Choose a workflow, review readiness, then inspect or launch runs.</p>
        </div>
        <button class="rf-button" :disabled="pending" @click="refresh()">Refresh catalog</button>
      </div>
    </section>

    <section v-if="pending" class="grid gap-4 xl:grid-cols-2">
      <div v-for="n in 4" :key="n" class="rf-card">
        <div class="rf-skeleton h-5 w-1/2 rounded" />
        <div class="rf-skeleton mt-3 h-4 w-full rounded" />
        <div class="rf-skeleton mt-2 h-4 w-3/4 rounded" />
      </div>
    </section>

    <section v-else-if="error" class="rf-card border-rose-400/30 bg-rose-500/10 text-rose-100">
      Could not load workflows. {{ error.message }}
    </section>

    <section v-else-if="workflows.length === 0" class="rf-card text-sm text-[color:var(--rf-muted)]">
      No workflow definitions discovered under <code>/workflows</code> yet.
    </section>

    <section v-else class="grid gap-4 xl:grid-cols-2">
      <article v-for="workflow in workflows" :key="workflow.id" class="rf-card">
        <div class="flex items-start justify-between gap-4">
          <div>
            <h2 class="text-xl font-semibold">{{ workflow.name || workflow.id || 'Unnamed workflow' }}</h2>
            <p class="mt-2 text-sm text-[color:var(--rf-muted)]">{{ workflow.description || 'No description provided.' }}</p>
          </div>
          <span class="rf-badge">v{{ workflow.version || 'n/a' }}</span>
        </div>

        <dl class="mt-5 space-y-2 text-sm">
          <div class="flex items-center justify-between gap-4">
            <dt class="text-[color:var(--rf-muted)]">Workflow ID</dt>
            <dd class="font-mono text-xs">{{ workflow.id || 'unknown' }}</dd>
          </div>
          <div class="flex items-center justify-between gap-4">
            <dt class="text-[color:var(--rf-muted)]">Observed runs</dt>
            <dd>{{ data?.runCounts?.[workflow.id] || 0 }}</dd>
          </div>
        </dl>

        <div class="mt-5 flex items-center justify-between text-sm text-[color:var(--rf-muted)]">
          <span class="truncate">{{ workflow.path }}</span>
          <NuxtLink :to="`/workflows/${workflow.id}`" class="rf-button">Open</NuxtLink>
        </div>
      </article>
    </section>
  </div>
</template>
