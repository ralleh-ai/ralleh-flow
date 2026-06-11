<script setup lang="ts">
import type { FlowRun } from '~/types/flow'

const api = useFlowApi()

const { data, pending, error, refresh } = await useAsyncData('flow-dashboard', async () => {
  const [workflows, runs] = await Promise.all([api.getWorkflows(), api.getRuns()])
  return { workflows, runs }
})

const runs = computed(() => data.value?.runs ?? [])
const workflowCount = computed(() => data.value?.workflows.length ?? 0)

const runningCount = computed(() => runs.value.filter((run) => run.status === 'running').length)
const approvalCount = computed(() => runs.value.filter((run) => run.status.includes('approval')).length)
const stalledCount = computed(() => runs.value.filter((run) => ['failed', 'cancelled'].includes(run.status)).length)

const cards = computed(() => [
  { title: 'Active runs', value: String(runningCount.value), hint: `${runs.value.length} tracked runs` },
  { title: 'Pending approvals', value: String(approvalCount.value), hint: approvalCount.value ? 'Human gate needed' : 'No blocked approvals' },
  { title: 'Workflow catalog', value: String(workflowCount.value), hint: 'Reusable orchestration packages' },
  { title: 'Stalled runs', value: String(stalledCount.value), hint: stalledCount.value ? 'Needs operator action' : 'No failed/cancelled runs' }
])

const latestEvent = (run: FlowRun) => run.timeline[run.timeline.length - 1]

const latestEventText = (run: FlowRun) => {
  const event = latestEvent(run)
  if (!event) return '—'
  return `${event.type} · ${new Date(event.at).toLocaleTimeString()}`
}

const statusTone = (status: string) => {
  if (status === 'running') return 'rf-badge rf-badge--ok'
  if (status.includes('approval')) return 'rf-badge rf-badge--warn'
  if (status === 'failed' || status === 'cancelled') return 'rf-badge rf-badge--danger'
  return 'rf-badge'
}
</script>

<template>
  <div class="space-y-6">
    <section class="rf-card">
      <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Operations cockpit</p>
      <div class="mt-3 flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 class="text-2xl font-semibold md:text-3xl">Execution truth for workflows, approvals, and assets</h1>
          <p class="mt-2 text-sm text-[color:var(--rf-muted)]">This view reflects only what the API currently knows. No optimistic placeholders.</p>
        </div>
        <button class="rf-button" :disabled="pending" @click="refresh()">Refresh data</button>
      </div>
    </section>

    <section class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
      <StatusCard v-for="card in cards" :key="card.title" v-bind="card" />
    </section>

    <section class="grid gap-6 xl:grid-cols-[minmax(0,1.7fr)_minmax(320px,1fr)]">
      <div class="rf-card min-w-0">
        <div class="flex items-center justify-between gap-4 border-b border-[color:var(--rf-border)] pb-3">
          <div>
            <h2 class="text-lg font-semibold">Run explorer</h2>
            <p class="text-sm text-[color:var(--rf-muted)]">Live run state, current step, and latest execution event.</p>
          </div>
          <NuxtLink to="/workflows" class="rf-button">Explore workflows</NuxtLink>
        </div>

        <div v-if="pending" class="mt-4 space-y-3">
          <div v-for="n in 4" :key="n" class="rf-skeleton h-12 rounded-xl" />
        </div>

        <div v-else-if="error" class="mt-4 rounded-2xl border border-rose-400/30 bg-rose-500/10 p-4 text-sm text-rose-100">
          Could not load dashboard data. {{ error.message }}
        </div>

        <div v-else-if="runs.length === 0" class="mt-4 rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-6 text-sm text-[color:var(--rf-muted)]">
          No runs yet. Create the first run from a workflow to populate operations.
        </div>

        <div v-else class="mt-4 overflow-x-auto">
          <table class="min-w-full text-sm">
            <thead class="text-left text-[color:var(--rf-muted)]">
              <tr>
                <th class="py-2 pr-4">Run</th>
                <th class="py-2 pr-4">Workflow</th>
                <th class="py-2 pr-4">State</th>
                <th class="py-2 pr-4">Current step</th>
                <th class="py-2">Latest event</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="run in runs" :key="run.id" class="border-t border-[color:var(--rf-border)]/70">
                <td class="py-3 pr-4 font-medium"><NuxtLink :to="`/runs/${run.id}`" class="hover:text-cyan-300">{{ run.id }}</NuxtLink></td>
                <td class="py-3 pr-4">{{ run.workflowId }}</td>
                <td class="py-3 pr-4"><span :class="statusTone(run.status)">{{ run.status }}</span></td>
                <td class="py-3 pr-4">{{ run.currentStep || '—' }}</td>
                <td class="py-3 text-xs text-[color:var(--rf-muted)]">{{ latestEventText(run) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <aside class="space-y-6">
        <section class="rf-card">
          <h2 class="text-lg font-semibold">Approvals pressure</h2>
          <p class="mt-2 text-sm text-[color:var(--rf-muted)]">
            {{ approvalCount ? `${approvalCount} run(s) are waiting on human approval.` : 'No runs currently blocked on approval gates.' }}
          </p>
          <NuxtLink to="/approvals" class="mt-4 inline-flex text-sm text-cyan-200 hover:text-cyan-100">Open approvals inbox →</NuxtLink>
        </section>

        <section class="rf-card">
          <h2 class="text-lg font-semibold">Asset readiness</h2>
          <p class="mt-2 text-sm text-[color:var(--rf-muted)]">
            Asset-level provenance isn’t returned by current API endpoints yet.
            This panel stays explicit so operators don’t infer missing truth.
          </p>
          <NuxtLink to="/assets" class="mt-4 inline-flex text-sm text-cyan-200 hover:text-cyan-100">Inspect asset registry →</NuxtLink>
        </section>
      </aside>
    </section>
  </div>
</template>
