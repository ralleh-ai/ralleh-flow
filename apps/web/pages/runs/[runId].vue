<script setup lang="ts">
const route = useRoute()
const api = useFlowApi()

const runId = computed(() => String(route.params.runId || ''))

const loadError = ref('')
const advanceError = ref('')
const stepActionError = ref('')
const advancing = ref(false)
const dispatching = ref(false)
const completing = ref(false)
const failing = ref(false)

const { data: run, pending, refresh } = useAsyncData(
  () => `flow-run-${runId.value}`,
  async () => {
    if (!runId.value) {
      loadError.value = 'Missing run id'
      return null
    }

    try {
      loadError.value = ''
      return await api.getRun(runId.value)
    } catch (err: any) {
      loadError.value = err?.data?.error || err?.message || 'Could not load run'
      return null
    }
  },
  {
    watch: [runId],
    default: () => null
  }
)

const timeline = computed(() => run.value?.timeline ?? [])
const steps = computed(() => run.value?.steps ?? [])
const activeStep = computed(() => steps.value.find((step) => step.status === 'running') || null)
const approvalEvents = computed(() => timeline.value.filter((event) => event.type.includes('approval')))
const handoffs = computed(() => run.value?.handoffs ?? [])

const statusTone = computed(() => {
  const status = run.value?.status || ''
  if (status === 'running') return 'rf-badge rf-badge--ok'
  if (status.includes('approval')) return 'rf-badge rf-badge--warn'
  if (status === 'failed' || status === 'cancelled') return 'rf-badge rf-badge--danger'
  return 'rf-badge'
})

const canAdvance = computed(() => run.value?.status === 'pending')
const activeHandoff = computed(() => handoffs.value.find((handoff) => handoff.stepId === activeStep.value?.stepId) || null)
const canDispatchStep = computed(() => !!activeStep.value && run.value?.status === 'running' && activeHandoff.value?.status === 'claimed')
const canCompleteStep = computed(() => !!activeStep.value && run.value?.status === 'running')

const advanceRun = async () => {
  if (!run.value || !canAdvance.value || advancing.value) {
    return
  }

  try {
    advancing.value = true
    advanceError.value = ''
    run.value = await api.advanceRun(run.value.id)
  } catch (err: any) {
    advanceError.value = err?.data?.error || err?.message || 'Could not advance run'
  } finally {
    advancing.value = false
  }
}

const dispatchStep = async () => {
  if (!run.value || !canDispatchStep.value || dispatching.value) {
    return
  }

  try {
    dispatching.value = true
    stepActionError.value = ''
    run.value = await api.dispatchRunStep(run.value.id, {
      sessionId: `session:${run.value.id}:${activeStep.value?.stepId || 'step'}`,
      note: 'Dispatch binding recorded pending real OpenClaw session integration'
    })
  } catch (err: any) {
    stepActionError.value = err?.data?.error || err?.message || 'Could not dispatch step'
  } finally {
    dispatching.value = false
  }
}

const completeStep = async () => {
  if (!run.value || !canCompleteStep.value || completing.value) {
    return
  }

  try {
    completing.value = true
    stepActionError.value = ''
    run.value = await api.completeRunStep(run.value.id, {
      summary: `Completed ${activeStep.value?.stepId || 'step'} checkpoint`,
      checkpointLabel: activeStep.value?.stepId || 'step-checkpoint'
    })
  } catch (err: any) {
    stepActionError.value = err?.data?.error || err?.message || 'Could not complete step'
  } finally {
    completing.value = false
  }
}

const failStep = async () => {
  if (!run.value || !canCompleteStep.value || failing.value) {
    return
  }

  try {
    failing.value = true
    stepActionError.value = ''
    run.value = await api.failRunStep(run.value.id, {
      reason: 'Operator marked active step failed pending execution integration'
    })
  } catch (err: any) {
    stepActionError.value = err?.data?.error || err?.message || 'Could not fail step'
  } finally {
    failing.value = false
  }
}

</script>

<template>
  <div class="space-y-6">
    <section class="rf-card">
      <div class="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <NuxtLink to="/" class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)] hover:text-cyan-200">Run explorer</NuxtLink>
          <h1 class="mt-2 text-3xl font-semibold">{{ runId }}</h1>
          <p class="mt-2 text-sm text-[color:var(--rf-muted)]">Detailed execution state, approvals, assets, and timeline truth.</p>
        </div>
        <div class="flex gap-3">
          <button class="rf-button" :disabled="pending || advancing" @click="refresh()">Refresh run</button>
          <button class="rf-button" :disabled="!canAdvance || advancing || pending" @click="advanceRun()">
            {{ advancing ? 'Advancing…' : 'Advance once' }}
          </button>
        </div>
      </div>
      <p v-if="advanceError" class="mt-3 text-sm text-rose-200">{{ advanceError }}</p>
      <p v-if="stepActionError" class="mt-2 text-sm text-rose-200">{{ stepActionError }}</p>
    </section>

    <section v-if="pending" class="grid gap-6 xl:grid-cols-[280px_minmax(0,1fr)]">
      <div class="rf-card space-y-3">
        <div class="rf-skeleton h-5 w-2/3 rounded" />
        <div class="rf-skeleton h-4 w-full rounded" />
        <div class="rf-skeleton h-4 w-4/5 rounded" />
      </div>
      <div class="rf-card space-y-3">
        <div v-for="n in 5" :key="n" class="rf-skeleton h-12 rounded-xl" />
      </div>
    </section>

    <section v-else-if="loadError" class="rf-card border-rose-400/30 bg-rose-500/10 text-rose-100">
      Could not load run {{ runId }}. {{ loadError }}
    </section>

    <section v-else-if="!run" class="rf-card text-sm text-[color:var(--rf-muted)]">
      Run {{ runId }} was not found.
    </section>

    <section v-else class="grid gap-6 xl:grid-cols-[280px_minmax(0,1fr)_minmax(260px,320px)]">
      <aside class="rf-card xl:sticky xl:top-24 xl:self-start">
        <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Execution truth</div>
        <h2 class="mt-2 text-lg font-semibold">Context</h2>
        <dl class="mt-4 space-y-3 text-sm">
          <div class="flex items-center justify-between gap-4">
            <dt class="text-[color:var(--rf-muted)]">Status</dt>
            <dd><span :class="statusTone">{{ run.status }}</span></dd>
          </div>
          <div class="flex items-center justify-between gap-4">
            <dt class="text-[color:var(--rf-muted)]">Workflow</dt>
            <dd>{{ run.workflowId }}</dd>
          </div>
          <div class="flex items-center justify-between gap-4">
            <dt class="text-[color:var(--rf-muted)]">Current step</dt>
            <dd>{{ run.currentStep || '—' }}</dd>
          </div>
          <div>
            <dt class="text-[color:var(--rf-muted)]">Operator control</dt>
            <dd class="mt-1 text-xs text-[color:var(--rf-muted)]">
              <span v-if="canAdvance">This run is pending and can be advanced once into running state.</span>
              <span v-else>Manual advance is only available while the run is pending.</span>
            </dd>
          </div>
          <div>
            <dt class="text-[color:var(--rf-muted)]">Step controls</dt>
            <dd class="mt-2 flex flex-wrap gap-2">
              <button class="rf-button" :disabled="!canDispatchStep || dispatching || completing || failing" @click="dispatchStep()">{{ dispatching ? 'Dispatching…' : 'Dispatch step' }}</button>
              <button class="rf-button" :disabled="!canCompleteStep || dispatching || completing || failing" @click="completeStep()">{{ completing ? 'Completing…' : 'Complete step' }}</button>
              <button class="rf-button" :disabled="!canCompleteStep || completing || failing" @click="failStep()">{{ failing ? 'Failing…' : 'Fail step' }}</button>
            </dd>
          </div>
          <div>
            <dt class="text-[color:var(--rf-muted)]">Active step claim</dt>
            <dd class="mt-1 text-xs text-[color:var(--rf-muted)]">
              <span v-if="activeStep">{{ activeStep.stepId }} claimed by {{ activeStep.workerId }}</span>
              <span v-else>No active step claim recorded yet.</span>
            </dd>
          </div>
          <div>
            <dt class="text-[color:var(--rf-muted)]">Branch</dt>
            <dd class="mt-1 break-all font-mono text-xs">{{ run.branch }}</dd>
          </div>
          <div>
            <dt class="text-[color:var(--rf-muted)]">Worktree</dt>
            <dd class="mt-1 break-all font-mono text-xs">{{ run.worktreePath }}</dd>
          </div>
        </dl>
      </aside>

      <section class="rf-card min-w-0">
        <div class="border-b border-[color:var(--rf-border)] pb-3">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Timeline</div>
          <h2 class="mt-2 text-xl font-semibold">Execution events</h2>
        </div>

        <div v-if="timeline.length === 0" class="mt-4 rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4 text-sm text-[color:var(--rf-muted)]">
          No timeline events recorded yet.
        </div>

        <div v-else class="mt-5 space-y-4">
          <div v-for="event in timeline" :key="`${event.at}-${event.type}`" class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
            <div class="flex items-center justify-between gap-4">
              <strong>{{ event.type }}</strong>
              <span class="text-xs text-[color:var(--rf-muted)]">{{ new Date(event.at).toLocaleString() }}</span>
            </div>
            <p class="mt-2 text-sm text-[color:var(--rf-muted)]">{{ event.detail }}</p>
          </div>
        </div>
      </section>

      <aside class="space-y-6">
        <section class="rf-card">
          <h2 class="text-lg font-semibold">Active steps</h2>
          <p class="mt-2 text-sm text-[color:var(--rf-muted)]" v-if="steps.length === 0">
            No persisted step records yet.
          </p>
          <ul v-else class="mt-3 space-y-3 text-sm text-[color:var(--rf-muted)]">
            <li v-for="step in steps" :key="`${step.stepId}-${step.startedAt}`" class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-3">
              <div class="flex items-center justify-between gap-3">
                <strong>{{ step.stepId }}</strong>
                <span class="rf-badge" :class="step.status === 'running' ? 'rf-badge--ok' : ''">{{ step.status }}</span>
              </div>
              <p class="mt-2 text-xs">Worker: {{ step.workerId }}</p>
              <p class="mt-1 text-xs">Started: {{ new Date(step.startedAt).toLocaleString() }}</p>
              <p class="mt-1 text-xs" v-if="step.kind || step.agent">Kind: {{ step.kind || '—' }}<span v-if="step.agent"> · Agent: {{ step.agent }}</span></p>
            </li>
          </ul>
        </section>

        <section class="rf-card">
          <h2 class="text-lg font-semibold">Approvals</h2>
          <p class="mt-2 text-sm text-[color:var(--rf-muted)]" v-if="approvalEvents.length === 0">
            No approval events reported by current API timeline.
          </p>
          <ul v-else class="mt-3 space-y-2 text-sm text-[color:var(--rf-muted)]">
            <li v-for="event in approvalEvents" :key="event.at">{{ event.type }} · {{ event.detail }}</li>
          </ul>
        </section>

        <section class="rf-card">
          <h2 class="text-lg font-semibold">Dispatch handoffs</h2>
          <p class="mt-2 text-sm text-[color:var(--rf-muted)]" v-if="handoffs.length === 0">
            No persisted handoff records yet.
          </p>
          <ul v-else class="mt-3 space-y-3 text-sm text-[color:var(--rf-muted)]">
            <li v-for="handoff in handoffs" :key="`${handoff.stepId}-${handoff.createdAt}`" class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-3">
              <div class="flex items-center justify-between gap-3">
                <strong>{{ handoff.stepId }}</strong>
                <span class="rf-badge" :class="handoff.status === 'claimed' ? 'rf-badge--ok' : ''">{{ handoff.status }}</span>
              </div>
              <p class="mt-2 text-xs">Worker: {{ handoff.workerId }}</p>
              <p class="mt-1 text-xs">Kind: {{ handoff.kind || '—' }}<span v-if="handoff.agent"> · Agent: {{ handoff.agent }}</span></p>
              <p class="mt-1 text-xs">Created: {{ new Date(handoff.createdAt).toLocaleString() }}</p>
              <p class="mt-1 text-xs" v-if="handoff.dispatchAttemptAt">Dispatch attempt: {{ new Date(handoff.dispatchAttemptAt).toLocaleString() }}</p>
              <p class="mt-1 text-xs" v-if="handoff.sessionId">Session: {{ handoff.sessionId }}</p>
            </li>
          </ul>
        </section>

        <section class="rf-card">
          <h2 class="text-lg font-semibold">Assets & artifacts</h2>
          <p class="mt-2 text-sm text-[color:var(--rf-muted)]">
            Asset manifests are not exposed from current endpoints yet. This panel will activate once API returns artifact references.
          </p>
        </section>
      </aside>
    </section>
  </div>
</template>
