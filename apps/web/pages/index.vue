<script setup lang="ts">
import type { FlowRun, FlowTimelineEvent } from '~/types/flow'
import type { OperationalFact } from '~/types/ui'

const api = useFlowApi()

const { data, pending, error, refresh } = await useAsyncData('flow-dashboard', async () => {
  const [workflows, runs] = await Promise.all([api.getWorkflows(), api.getRuns()])
  return { workflows, runs }
})

const runs = computed(() => data.value?.runs ?? [])
const workflows = computed(() => data.value?.workflows ?? [])

const latestEvent = (run: FlowRun): FlowTimelineEvent | null => run.timeline[run.timeline.length - 1] ?? null

const eventTimeMs = (run: FlowRun) => {
  const event = latestEvent(run)
  if (event?.at) return new Date(event.at).getTime()
  if (run.createdAt) return new Date(run.createdAt).getTime()
  return 0
}

const latestEventText = (run: FlowRun) => {
  const event = latestEvent(run)
  if (!event) return 'No recorded timeline event yet.'
  return `${event.type} · ${new Date(event.at).toLocaleString()}`
}

const statusTone = (status: string) => {
  if (status === 'running' || status === 'completed') return 'rf-badge rf-badge--ok'
  if (status.includes('approval') || status === 'changes_requested' || status === 'pending') return 'rf-badge rf-badge--warn'
  if (status === 'failed' || status === 'cancelled' || status === 'rejected') return 'rf-badge rf-badge--danger'
  return 'rf-badge'
}

const attentionTone = (status: string) => {
  if (status === 'failed') return 'text-rose-200'
  if (status === 'changes_requested' || status === 'waiting_for_approval') return 'text-amber-200'
  if (status === 'pending') return 'text-cyan-200'
  return 'text-[color:var(--rf-muted)]'
}

const attentionLabel = (run: FlowRun) => {
  if (run.status === 'waiting_for_approval') return 'Approval gate is blocking progress.'
  if (run.status === 'changes_requested') return 'Rework requested. Approval gate can be resumed when ready.'
  if (run.status === 'failed') return 'Execution failed and needs intervention.'
  if (run.status === 'pending') return 'Run is staged and can be advanced by an operator.'
  return 'No immediate operator intervention required.'
}

const runsByRecentChange = computed(() => [...runs.value].sort((a, b) => eventTimeMs(b) - eventTimeMs(a)))
const attentionRuns = computed(() => runsByRecentChange.value.filter((run) => ['waiting_for_approval', 'changes_requested', 'failed', 'pending'].includes(run.status)))
const liveRuns = computed(() => runsByRecentChange.value.filter((run) => ['running', 'pending'].includes(run.status)))
const recentChangedRuns = computed(() => runsByRecentChange.value.slice(0, 8))
const failedRuns = computed(() => attentionRuns.value.filter((run) => run.status === 'failed'))
const approvalBlockedRuns = computed(() => attentionRuns.value.filter((run) => run.status === 'waiting_for_approval'))
const reworkRuns = computed(() => attentionRuns.value.filter((run) => run.status === 'changes_requested'))
const stagedRuns = computed(() => attentionRuns.value.filter((run) => run.status === 'pending'))

const activeCount = computed(() => runs.value.filter((run) => run.status === 'running').length)
const approvalCount = computed(() => approvalBlockedRuns.value.length)
const reworkCount = computed(() => reworkRuns.value.length)
const stalledCount = computed(() => runs.value.filter((run) => ['failed', 'cancelled'].includes(run.status)).length)
const quietCount = computed(() => runs.value.filter((run) => ['completed', 'cancelled'].includes(run.status)).length)

const cockpitHeadline = computed(() => {
  if (failedRuns.value.length > 0) return 'Failures exist right now. Clear those before pretending the rest of the board is healthy.'
  if (approvalBlockedRuns.value.length > 0) return 'Human decisions are the main live constraint right now.'
  if (reworkRuns.value.length > 0) return 'Requested changes are stacking up and deserve explicit follow-through.'
  if (stagedRuns.value.length > 0) return 'There are staged runs ready for an operator to push forward.'
  if (activeCount.value > 0) return 'Execution is live. Watch the missions that are already in motion before launching more.'
  return 'No immediate operator fire is visible. Use the quiet to inspect package readiness and launch deliberately.'
})

const cockpitFacts = computed<OperationalFact[]>(() => [
  { label: 'Failed', value: String(failedRuns.value.length), tone: failedRuns.value.length ? 'danger' : 'ok' },
  { label: 'Approval blocked', value: String(approvalBlockedRuns.value.length), tone: approvalBlockedRuns.value.length ? 'warn' : 'ok' },
  { label: 'Rework', value: String(reworkRuns.value.length), tone: reworkRuns.value.length ? 'warn' : 'default' },
  { label: 'Staged', value: String(stagedRuns.value.length), tone: stagedRuns.value.length ? 'warn' : 'default' }
])

const cockpitBullets = computed(() => {
  if (failedRuns.value.length > 0) {
    return [
      'Open the failed runs first and inspect the mission timeline before advancing anything else.',
      'Treat failure as a truth surface, not just a status badge.',
      'If a run failed after human review or dispatch, check that handoff path before relaunching.'
    ]
  }

  if (approvalBlockedRuns.value.length > 0) {
    return [
      'The governance queue is currently the fastest way to change system state.',
      'Use each run mission view to validate context before making an approval decision.',
      'Do not approve from memory; inspect the live mission first.'
    ]
  }

  if (reworkRuns.value.length > 0) {
    return [
      'Changes were explicitly requested, so completion pressure is not the same as readiness.',
      'Use the mission timeline to confirm the requested rework actually happened.',
      'Resume only when reopening the same gate is the deliberate next move.'
    ]
  }

  if (stagedRuns.value.length > 0) {
    return [
      'These runs are ready for operator advancement, not silently self-starting.',
      'Check package readiness if the run depends on external prompts, files, or documents.',
      'Advance one with intent rather than treating pending as harmless backlog.'
    ]
  }

  return [
    'No urgent intervention signal is visible right now.',
    'Use this calm state to validate packages, inspect readiness pressure, and launch deliberately.',
    'The cockpit remains a triage surface, not a fake control plane.'
  ]
})

const cards = computed(() => [
  {
    title: 'Active operations',
    value: String(activeCount.value),
    hint: activeCount.value ? 'Live execution in progress' : 'No runs actively executing'
  },
  {
    title: 'Approval pressure',
    value: String(approvalCount.value),
    hint: approvalCount.value ? 'Human decisions are blocking progress' : 'No live approval gates waiting'
  },
  {
    title: 'Rework queue',
    value: String(reworkCount.value),
    hint: reworkCount.value ? 'Runs paused for requested changes' : 'No runs paused for rework'
  },
  {
    title: 'Stalled operations',
    value: String(stalledCount.value),
    hint: stalledCount.value ? 'Failures or cancellations need review' : 'No stalled runs right now'
  }
])
</script>

<template>
  <div class="space-y-6">
    <section class="rf-card" data-testid="dashboard-command-view">
      <div class="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Operations cockpit</p>
          <h1 class="mt-3 text-2xl font-semibold md:text-3xl">Command view for live workflow operations</h1>
          <p class="mt-3 max-w-4xl text-sm text-[color:var(--rf-muted)]">
            Start with what needs intervention, then what is active, then what changed. This screen stays anchored to current API truth instead of pretending to know more than the runtime exposes.
          </p>
          <div class="mt-4 flex flex-wrap gap-x-4 gap-y-1 text-xs text-[color:var(--rf-muted)]">
            <span>{{ runs.length }} tracked runs</span>
            <span>{{ workflows.length }} workflows</span>
            <span>{{ quietCount }} quiet outcomes</span>
          </div>
        </div>
        <div class="flex flex-wrap gap-3">
          <NuxtLink to="/approvals" class="rf-button" data-testid="dashboard-open-governance">Open governance queue</NuxtLink>
          <button class="rf-button" :disabled="pending" @click="refresh()">Refresh cockpit</button>
        </div>
      </div>
    </section>

    <section class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
      <DashboardStatusCard v-for="card in cards" :key="card.title" v-bind="card" />
    </section>

    <AttentionContextCard
      title="Immediate command intent"
      :summary="cockpitHeadline"
      :facts="cockpitFacts"
      :bullets="cockpitBullets"
      :links="[
        { label: 'Open governance queue', to: '/approvals' },
        { label: 'Open resource intelligence', to: '/assets' }
      ]"
    />

    <section v-if="error" class="rf-card border-rose-400/30 bg-rose-500/10 text-rose-100">
      Could not load cockpit data. {{ error.message }}
    </section>

    <section v-else class="grid gap-6 xl:grid-cols-[minmax(0,1.25fr)_minmax(0,1.1fr)_320px]">
      <section class="rf-card min-w-0">
        <div class="border-b border-[color:var(--rf-border)] pb-3">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Needs attention</div>
          <h2 class="mt-2 text-xl font-semibold">Operator triage queue</h2>
          <p class="mt-2 text-sm text-[color:var(--rf-muted)]">Runs that are blocked, staged, or failed — the places a human decision changes the system.</p>
        </div>

        <div v-if="pending" class="mt-4 space-y-3">
          <div v-for="n in 4" :key="n" class="rf-skeleton h-24 rounded-2xl" />
        </div>

        <div v-else-if="attentionRuns.length === 0" class="mt-4 rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-6 text-sm text-[color:var(--rf-muted)]">
          Nothing is currently asking for operator intervention.
        </div>

        <div v-else class="mt-4 space-y-3">
          <NuxtLink
            v-for="run in attentionRuns"
            :key="run.id"
            :to="`/runs/${run.id}`"
            data-testid="dashboard-triage-run"
            :data-run-id="run.id"
            class="block rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4 transition hover:border-cyan-400/40 hover:bg-black/20"
          >
            <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <strong class="truncate">{{ run.id }}</strong>
                  <span :class="statusTone(run.status)">{{ run.status }}</span>
                </div>
                <p class="mt-2 text-sm text-white/90">{{ attentionLabel(run) }}</p>
                <div class="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-[color:var(--rf-muted)]">
                  <span>Workflow {{ run.workflowId }}</span>
                  <span>Current step {{ run.currentStep || '—' }}</span>
                </div>
              </div>
              <div class="text-left md:max-w-[18rem] md:text-right">
                <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Latest change</div>
                <p class="mt-2 text-xs" :class="attentionTone(run.status)">{{ latestEventText(run) }}</p>
              </div>
            </div>
          </NuxtLink>
        </div>
      </section>

      <section class="rf-card min-w-0">
        <div class="border-b border-[color:var(--rf-border)] pb-3">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Active work</div>
          <h2 class="mt-2 text-xl font-semibold">Live operations</h2>
          <p class="mt-2 text-sm text-[color:var(--rf-muted)]">Runs currently executing or staged for immediate advancement.</p>
        </div>

        <div v-if="pending" class="mt-4 space-y-3">
          <div v-for="n in 4" :key="n" class="rf-skeleton h-20 rounded-2xl" />
        </div>

        <div v-else-if="liveRuns.length === 0" class="mt-4 rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-6 text-sm text-[color:var(--rf-muted)]">
          No live or staged runs right now.
        </div>

        <div v-else class="mt-4 space-y-3">
          <NuxtLink
            v-for="run in liveRuns"
            :key="run.id"
            :to="`/runs/${run.id}`"
            data-testid="dashboard-live-run"
            :data-run-id="run.id"
            class="block rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4 transition hover:border-cyan-400/40 hover:bg-black/20"
          >
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <strong class="truncate">{{ run.id }}</strong>
                  <span :class="statusTone(run.status)">{{ run.status }}</span>
                </div>
                <div class="mt-2 text-sm text-[color:var(--rf-muted)]">{{ run.workflowId }} · {{ run.currentStep || 'No current step recorded' }}</div>
                <div class="mt-3 text-xs text-[color:var(--rf-muted)]">{{ latestEventText(run) }}</div>
              </div>
            </div>
          </NuxtLink>
        </div>
      </section>

      <aside class="space-y-6">
        <section class="rf-card">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Recent change</div>
          <h2 class="mt-2 text-lg font-semibold">Activity pulse</h2>
          <p class="mt-2 text-sm text-[color:var(--rf-muted)]" v-if="recentChangedRuns.length === 0">
            No run activity has been recorded yet.
          </p>
          <ul v-else class="mt-4 space-y-3 text-sm text-[color:var(--rf-muted)]">
            <li v-for="run in recentChangedRuns" :key="run.id" class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-3">
              <div class="flex items-center justify-between gap-3">
                <NuxtLink :to="`/runs/${run.id}`" class="font-medium text-white/90 hover:text-cyan-300">{{ run.id }}</NuxtLink>
                <span :class="statusTone(run.status)">{{ run.status }}</span>
              </div>
              <p class="mt-2 text-xs">{{ latestEventText(run) }}</p>
            </li>
          </ul>
        </section>

        <section class="rf-card">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Truth boundaries</div>
          <h2 class="mt-2 text-lg font-semibold">What this cockpit does not fake</h2>
          <ul class="mt-4 space-y-3 text-sm text-[color:var(--rf-muted)]">
            <li>Asset provenance is still handled on the asset registry page, not synthesized here.</li>
            <li>Git trust remains run-specific and stays on the run mission view where branch and worktree are explicit.</li>
            <li>Approval rationale and decision context live in the governance queue and each run detail page.</li>
          </ul>
          <div class="mt-4 flex flex-col gap-2 text-sm">
            <NuxtLink to="/workflows" class="text-cyan-200 hover:text-cyan-100">Open workflow catalog →</NuxtLink>
            <NuxtLink to="/assets" class="text-cyan-200 hover:text-cyan-100">Open asset registry →</NuxtLink>
          </div>
        </section>
      </aside>
    </section>
  </div>
</template>
