<script setup lang="ts">
import type { FlowRun, FlowTimelineEvent, FlowWorkflow } from '~/types/flow'

const api = useFlowApi()

type WorkflowMissionSummary = {
  workflow: FlowWorkflow
  runCount: number
  liveCount: number
  attentionCount: number
  completedCount: number
  lastChangedAtMs: number
  lastChangedLabel: string
  missionLabel: string
  nextAction: string
}

const { data, pending, error, refresh } = await useAsyncData('flow-workflow-catalog', async () => {
  const [workflows, runs] = await Promise.all([api.getWorkflows(), api.getRuns()])
  return { workflows, runs }
})

const workflows = computed(() => data.value?.workflows ?? [])
const runs = computed(() => data.value?.runs ?? [])

const latestEvent = (run: FlowRun): FlowTimelineEvent | null => run.timeline[run.timeline.length - 1] ?? null

const runLastChangedMs = (run: FlowRun) => {
  const event = latestEvent(run)
  if (event?.at) return new Date(event.at).getTime()
  if (run.createdAt) return new Date(run.createdAt).getTime()
  return 0
}

const describeWorkflowMission = (workflowRuns: FlowRun[]) => {
  if (workflowRuns.some((run) => run.status === 'waiting_for_approval')) {
    return 'Human approval is currently the deciding factor for this package.'
  }
  if (workflowRuns.some((run) => run.status === 'changes_requested')) {
    return 'Rework was requested before this package can move forward again.'
  }
  if (workflowRuns.some((run) => run.status === 'failed')) {
    return 'A recent execution stalled and needs investigation.'
  }
  if (workflowRuns.some((run) => run.status === 'running')) {
    return 'This package has live execution in motion.'
  }
  if (workflowRuns.some((run) => run.status === 'pending')) {
    return 'This package has a staged run ready to advance.'
  }
  if (workflowRuns.some((run) => run.status === 'completed')) {
    return 'This package has completed runs in its operational history.'
  }
  return 'No runs recorded yet. Validate the package, dry-run it, then launch the first mission deliberately.'
}

const describeNextAction = (workflowRuns: FlowRun[]) => {
  if (workflowRuns.some((run) => run.status === 'waiting_for_approval')) return 'Review blocked approvals'
  if (workflowRuns.some((run) => run.status === 'changes_requested')) return 'Resume after rework'
  if (workflowRuns.some((run) => run.status === 'failed')) return 'Inspect the stalled run'
  if (workflowRuns.some((run) => run.status === 'pending')) return 'Advance the staged run'
  if (workflowRuns.some((run) => run.status === 'running')) return 'Monitor the live mission'
  return 'Open package, validate, and launch'
}

const workflowSummaries = computed<WorkflowMissionSummary[]>(() => {
  return workflows.value
    .map((workflow) => {
      const workflowRuns = runs.value.filter((run) => run.workflowId === workflow.id)
      const liveCount = workflowRuns.filter((run) => ['running', 'pending'].includes(run.status)).length
      const attentionCount = workflowRuns.filter((run) => ['waiting_for_approval', 'changes_requested', 'failed', 'pending'].includes(run.status)).length
      const completedCount = workflowRuns.filter((run) => run.status === 'completed').length
      const lastChangedAtMs = workflowRuns.reduce((max, run) => Math.max(max, runLastChangedMs(run)), 0)
      const lastChangedLabel = lastChangedAtMs
        ? new Date(lastChangedAtMs).toLocaleString()
        : 'No activity recorded yet'

      return {
        workflow,
        runCount: workflowRuns.length,
        liveCount,
        attentionCount,
        completedCount,
        lastChangedAtMs,
        lastChangedLabel,
        missionLabel: describeWorkflowMission(workflowRuns),
        nextAction: describeNextAction(workflowRuns)
      }
    })
    .sort((a, b) => {
      if (b.attentionCount !== a.attentionCount) return b.attentionCount - a.attentionCount
      if (b.liveCount !== a.liveCount) return b.liveCount - a.liveCount
      if (b.lastChangedAtMs !== a.lastChangedAtMs) return b.lastChangedAtMs - a.lastChangedAtMs
      return a.workflow.name.localeCompare(b.workflow.name)
    })
})

const workflowsNeedingAttention = computed(() => workflowSummaries.value.filter((workflow) => workflow.attentionCount > 0))
const livePackages = computed(() => workflowSummaries.value.filter((workflow) => workflow.liveCount > 0))
const dormantPackages = computed(() => workflowSummaries.value.filter((workflow) => workflow.runCount === 0))

const cards = computed(() => [
  {
    title: 'Operational packages',
    value: String(workflows.value.length),
    hint: workflows.value.length ? 'Workflow definitions available for command' : 'No workflow packages discovered'
  },
  {
    title: 'Needs attention',
    value: String(workflowsNeedingAttention.value.length),
    hint: workflowsNeedingAttention.value.length ? 'Packages with blocked, failed, or staged work' : 'No workflow package currently asking for intervention'
  },
  {
    title: 'Live packages',
    value: String(livePackages.value.length),
    hint: livePackages.value.length ? 'Packages with active or staged missions' : 'No package is live right now'
  },
  {
    title: 'Dormant packages',
    value: String(dormantPackages.value.length),
    hint: dormantPackages.value.length ? 'Definitions with no execution history yet' : 'Every package has at least one recorded run'
  }
])
</script>

<template>
  <div class="space-y-6">
    <section class="rf-card">
      <div class="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
        <div>
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Workflow command surface</div>
          <h1 class="mt-2 text-3xl font-semibold">Operational packages</h1>
          <p class="mt-3 max-w-4xl text-sm text-[color:var(--rf-muted)]">
            Workflows are command packages, not static definitions. This catalog answers which packages are active, which ones are blocked, and where an operator should look next.
          </p>
        </div>
        <div class="flex flex-wrap gap-3">
          <NuxtLink to="/" class="rf-button">Open cockpit</NuxtLink>
          <button class="rf-button" :disabled="pending" @click="refresh()">Refresh catalog</button>
        </div>
      </div>
    </section>

    <section class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
      <DashboardStatusCard v-for="card in cards" :key="card.title" v-bind="card" />
    </section>

    <section v-if="pending" class="grid gap-4 xl:grid-cols-[minmax(0,1.5fr)_340px]">
      <div class="space-y-3">
        <div v-for="n in 4" :key="n" class="rf-skeleton h-32 rounded-2xl" />
      </div>
      <div class="rf-skeleton h-80 rounded-2xl" />
    </section>

    <section v-else-if="error" class="rf-card border-rose-400/30 bg-rose-500/10 text-rose-100">
      Could not load workflows. {{ error.message }}
    </section>

    <section v-else-if="workflowSummaries.length === 0" class="rf-card text-sm text-[color:var(--rf-muted)]">
      No workflow definitions discovered under <code>/workflows</code> yet.
    </section>

    <section v-else class="grid gap-6 xl:grid-cols-[minmax(0,1.5fr)_340px]">
      <section class="space-y-3 min-w-0">
        <article
          v-for="summary in workflowSummaries"
          :key="summary.workflow.id"
          class="rf-card"
        >
          <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <h2 class="text-xl font-semibold">{{ summary.workflow.name || summary.workflow.id || 'Unnamed workflow' }}</h2>
                <span class="rf-badge">v{{ summary.workflow.version || 'n/a' }}</span>
                <span v-if="summary.attentionCount" class="rf-badge rf-badge--warn">{{ summary.attentionCount }} attention</span>
                <span v-else-if="summary.liveCount" class="rf-badge rf-badge--ok">live</span>
              </div>
              <p class="mt-2 text-sm text-[color:var(--rf-muted)]">{{ summary.workflow.description || 'No description provided.' }}</p>
              <p class="mt-3 text-sm text-white/90">{{ summary.missionLabel }}</p>
            </div>
            <div class="lg:max-w-[16rem] lg:text-right">
              <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Next best action</div>
              <p class="mt-2 text-sm text-cyan-200">{{ summary.nextAction }}</p>
            </div>
          </div>

          <div class="mt-5 grid gap-3 md:grid-cols-4">
            <div class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-3">
              <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Observed runs</div>
              <div class="mt-2 text-2xl font-semibold">{{ summary.runCount }}</div>
            </div>
            <div class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-3">
              <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Live</div>
              <div class="mt-2 text-2xl font-semibold">{{ summary.liveCount }}</div>
            </div>
            <div class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-3">
              <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Completed</div>
              <div class="mt-2 text-2xl font-semibold">{{ summary.completedCount }}</div>
            </div>
            <div class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-3">
              <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Last change</div>
              <div class="mt-2 text-sm font-medium">{{ summary.lastChangedLabel }}</div>
            </div>
          </div>

          <div class="mt-5 flex flex-col gap-3 border-t border-[color:var(--rf-border)] pt-4 md:flex-row md:items-center md:justify-between">
            <div class="min-w-0 text-xs text-[color:var(--rf-muted)]">
              <span class="font-mono">{{ summary.workflow.id }}</span>
              <span class="mx-2">•</span>
              <span class="truncate">{{ summary.workflow.path }}</span>
            </div>
            <NuxtLink :to="`/workflows/${summary.workflow.id}`" class="rf-button">Open operational package</NuxtLink>
          </div>
        </article>
      </section>

      <aside class="space-y-6">
        <section class="rf-card">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Catalog doctrine</div>
          <h2 class="mt-2 text-lg font-semibold">How to read this page</h2>
          <ul class="mt-4 space-y-3 text-sm text-[color:var(--rf-muted)]">
            <li>Use attention counts to find where an operator decision changes the system.</li>
            <li>Use live counts to find packages that are already in motion.</li>
            <li>Open the package detail when you need readiness checks, launch controls, or run history.</li>
          </ul>
        </section>

        <section class="rf-card">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Truth boundaries</div>
          <h2 class="mt-2 text-lg font-semibold">What this catalog does not fake</h2>
          <ul class="mt-4 space-y-3 text-sm text-[color:var(--rf-muted)]">
            <li>Readiness is validated from each package page, not guessed here.</li>
            <li>Git trust remains mission-specific and stays on the run mission view.</li>
            <li>Agent transcript theater is intentionally absent; packages summarize operational pressure, not chat logs.</li>
          </ul>
        </section>
      </aside>
    </section>
  </div>
</template>
