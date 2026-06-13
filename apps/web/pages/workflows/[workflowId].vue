<script setup lang="ts">
import type { FlowDryRunResult, FlowRun, FlowTimelineEvent, FlowValidationResult, FlowWorkflowDetail, FlowWorkflowStep } from '~/types/flow'
import type { OperationalFact } from '~/types/ui'
import AttentionContextCard from '~/components/operations/AttentionContextCard.vue'
import OperationalFactGrid from '~/components/operations/OperationalFactGrid.vue'

const route = useRoute()
const api = useFlowApi()

const workflowId = computed(() => String(route.params.workflowId || ''))
const dependencyKeywords = ['asset', 'file', 'path', 'url', 'uri', 'image', 'doc', 'document', 'prompt', 'template', 'input', 'source', 'artifact']

const loadError = ref('')
const submitPending = ref(false)
const submitError = ref('')
const createdRun = ref<FlowRun | null>(null)
const validationPending = ref(false)
const validationError = ref('')
const validationResult = ref<FlowValidationResult | null>(null)
const dryRunPending = ref(false)
const dryRunError = ref('')
const dryRunResult = ref<FlowDryRunResult | null>(null)
const variableValues = reactive<Record<string, string>>({})

type WorkflowPagePayload = {
  workflow: FlowWorkflowDetail | null
  runs: FlowRun[]
}

const { data, pending, refresh } = useAsyncData<WorkflowPagePayload>(
  () => `flow-workflow-${workflowId.value}`,
  async () => {
    if (!workflowId.value) {
      loadError.value = 'Missing workflow id'
      return { workflow: null, runs: [] }
    }

    try {
      loadError.value = ''
      const [workflow, runs] = await Promise.all([
        api.getWorkflow(workflowId.value),
        api.getRuns()
      ])
      return { workflow, runs }
    } catch (err: any) {
      loadError.value = err?.data?.error || err?.message || 'Could not load workflow'
      return { workflow: null, runs: [] }
    }
  },
  {
    watch: [workflowId],
    default: () => ({ workflow: null, runs: [] })
  }
)

const workflow = computed(() => data.value?.workflow ?? null)
const allRuns = computed(() => data.value?.runs ?? [])
const workflowRuns = computed(() => allRuns.value.filter((run) => run.workflowId === workflow.value?.id))
const variables = computed(() => workflow.value?.variables ?? [])
const steps = computed(() => workflow.value?.steps ?? [])

const latestEvent = (run: FlowRun): FlowTimelineEvent | null => run.timeline[run.timeline.length - 1] ?? null
const runLastChangedMs = (run: FlowRun) => {
  const event = latestEvent(run)
  if (event?.at) return new Date(event.at).getTime()
  if (run.createdAt) return new Date(run.createdAt).getTime()
  return 0
}
const runLastChangedLabel = (run: FlowRun) => {
  const changedMs = runLastChangedMs(run)
  return changedMs ? new Date(changedMs).toLocaleString() : 'No activity recorded yet'
}

const recentRuns = computed(() => [...workflowRuns.value].sort((a, b) => runLastChangedMs(b) - runLastChangedMs(a)))
const liveRunCount = computed(() => workflowRuns.value.filter((run) => ['running', 'pending'].includes(run.status)).length)
const approvalPressureCount = computed(() => workflowRuns.value.filter((run) => ['waiting_for_approval', 'changes_requested'].includes(run.status)).length)
const stalledRunCount = computed(() => workflowRuns.value.filter((run) => ['failed', 'cancelled'].includes(run.status)).length)
const completedRunCount = computed(() => workflowRuns.value.filter((run) => run.status === 'completed').length)
const agentStepCount = computed(() => steps.value.filter((step) => !!step.agent).length)
const approvalGateCount = computed(() => steps.value.filter((step) => !!step.approverPolicy || step.kind === 'approval').length)

const packageOverviewFacts = computed<OperationalFact[]>(() => [
  { label: 'Live runs', value: String(liveRunCount.value), tone: liveRunCount.value ? 'ok' : 'default' },
  { label: 'Approval pressure', value: String(approvalPressureCount.value), tone: approvalPressureCount.value ? 'warn' : 'default' },
  { label: 'Completed runs', value: String(completedRunCount.value), tone: completedRunCount.value ? 'ok' : 'default' },
  { label: 'Stalled runs', value: String(stalledRunCount.value), tone: stalledRunCount.value ? 'danger' : 'default' }
])

const packageIdentityFacts = computed<OperationalFact[]>(() => [
  { label: 'Workflow ID', value: workflow.value?.id || 'unknown', detail: 'Current package identifier from the API.' },
  { label: 'Definition path', value: workflow.value?.path || 'unknown', detail: 'Git-tracked source for this package definition.' },
  { label: 'Agent steps', value: String(agentStepCount.value), detail: 'Execution steps assigned to named operational specialists.' },
  { label: 'Approval gates', value: String(approvalGateCount.value), detail: 'Human decision checkpoints encoded in the package.' }
])

watch(
  variables,
  (nextVariables) => {
    const allowedKeys = new Set(nextVariables.map((variable) => variable.key))

    for (const key of Object.keys(variableValues)) {
      if (!allowedKeys.has(key)) {
        delete variableValues[key]
      }
    }

    for (const variable of nextVariables) {
      if (!(variable.key in variableValues)) {
        variableValues[variable.key] = ''
      }
    }
  },
  { immediate: true }
)

const missingRequiredVariables = computed(() => {
  return variables.value
    .filter((variable) => variable.required && !String(variableValues[variable.key] || '').trim())
    .map((variable) => variable.key)
})

const canCreateRun = computed(() => !!workflow.value && !submitPending.value && missingRequiredVariables.value.length === 0)
const createdRunId = computed(() => createdRun.value?.id || '')

const dependencySignals = computed(() => {
  return variables.value.flatMap((variable) => {
    const haystack = [variable.key, variable.type, variable.description]
      .filter(Boolean)
      .join(' ')
      .toLowerCase()

    const matches = dependencyKeywords.filter((keyword) => haystack.includes(keyword))
    if (matches.length === 0) return []

    return [{
      key: variable.key,
      type: variable.type || 'unknown',
      required: variable.required,
      reason: `${matches.slice(0, 2).join(' / ')} signal in contract`
    }]
  })
})

const variableReadinessFacts = computed<OperationalFact[]>(() => [
  { label: 'Required missing', value: String(missingRequiredVariables.value.length), tone: missingRequiredVariables.value.length ? 'warn' : 'ok' },
  { label: 'Dependency signals', value: String(dependencySignals.value.length), tone: dependencySignals.value.length ? 'warn' : 'default' },
  { label: 'Validation', value: validationResult.value ? (validationResult.value.valid ? 'Pass' : 'Issues') : 'Not run', tone: validationResult.value ? (validationResult.value.valid ? 'ok' : 'warn') : 'default' },
  { label: 'Dry-run', value: dryRunResult.value ? (dryRunResult.value.ready ? 'Ready' : 'Blocked') : 'Not run', tone: dryRunResult.value ? (dryRunResult.value.ready ? 'ok' : 'warn') : 'default' }
])

const createdRunSummary = computed(() => {
  if (!createdRun.value) return ''
  if (createdRun.value.status === 'pending') return 'Run created and staged. The next operational move is to open the mission view and advance it deliberately.'
  if (createdRun.value.status === 'running') return 'Run created and already executing. Open the mission view to monitor the active worker and current step.'
  if (createdRun.value.status === 'waiting_for_approval') return 'Run created and paused at a governance gate. Open the mission view, then move to approvals if a human decision is now the blocker.'
  if (createdRun.value.status === 'changes_requested') return 'Run created with rework pressure already recorded. Inspect the mission timeline before deciding how to continue.'
  return `Run created with status ${createdRun.value.status}. Open the mission view for current operational truth.`
})

const createdRunFacts = computed<OperationalFact[]>(() => {
  if (!createdRun.value) return []

  const statusTone: 'default' | 'ok' | 'warn' | 'danger' = createdRun.value.status === 'running'
    ? 'ok'
    : ['pending', 'waiting_for_approval', 'changes_requested'].includes(createdRun.value.status)
      ? 'warn'
      : ['failed', 'cancelled', 'rejected'].includes(createdRun.value.status)
        ? 'danger'
        : 'default'

  return [
    {
      label: 'Run',
      value: createdRun.value.id,
      tone: 'ok'
    },
    {
      label: 'Status',
      value: createdRun.value.status,
      tone: statusTone
    },
    {
      label: 'Current step',
      value: createdRun.value.currentStep || '—'
    }
  ]
})

const packageHeadline = computed(() => {
  if (approvalPressureCount.value > 0) return 'This package currently has human decisions shaping its operational flow.'
  if (stalledRunCount.value > 0) return 'This package has stalled history that should be reviewed before more launches.'
  if (liveRunCount.value > 0) return 'This package already has live or staged work in motion.'
  if (completedRunCount.value > 0) return 'This package has completed operational history you can inspect before the next launch.'
  return 'This package has not run yet. Validate it, dry-run it, then launch deliberately.'
})

const readinessSummary = computed(() => {
  if (missingRequiredVariables.value.length > 0) {
    return `Launch inputs still missing: ${missingRequiredVariables.value.join(', ')}`
  }
  if (validationResult.value && !validationResult.value.valid) {
    return 'Validation found issues that should be resolved before launch.'
  }
  if (dryRunResult.value && !dryRunResult.value.ready) {
    return 'Dry-run found blockers or pending requirements.'
  }
  if (dryRunResult.value?.ready) {
    return 'Dry-run says this package is currently ready to launch.'
  }
  return 'Run validation and dry-run when you want a current readiness verdict.'
})

const launchDecisionSummary = computed(() => {
  if (missingRequiredVariables.value.length > 0) return 'Launch is not ready yet because required inputs are still missing.'
  if (validationResult.value && !validationResult.value.valid) return 'The package contract is currently failing validation. Fix that before launch.'
  if (dryRunResult.value && !dryRunResult.value.ready) return 'Dry-run is signaling blockers or pending requirements, so launch should wait.'
  if (approvalPressureCount.value > 0) return 'This package already has governance pressure in flight. Launching more work may increase operator load.'
  if (stalledRunCount.value > 0) return 'Past failures exist. Review them before assuming the next run will be clean.'
  if (dryRunResult.value?.ready) return 'Current signals say this package is ready for a deliberate launch.'
  return 'Readiness is still partially unknown until validation and dry-run are executed.'
})

const missionStepSummary = (step: FlowWorkflowStep, index: number) => {
  const pieces = [`Step ${index + 1}`]
  if (step.kind) pieces.push(step.kind)
  if (step.agent) pieces.push(`agent ${step.agent}`)
  if (step.approverPolicy) pieces.push(`approval ${step.approverPolicy}`)
  return pieces.join(' · ')
}

const missionStepDetail = (step: FlowWorkflowStep) => {
  if (step.approverPolicy || step.kind === 'approval') {
    return 'This checkpoint exists to slow the system down for a deliberate human decision.'
  }
  if (step.agent) {
    return 'This step delegates real work to a named operational specialist.'
  }
  return 'This step advances the mission state without pretending to be more than the current workflow definition states.'
}

const statusTone = (status: string) => {
  if (status === 'running' || status === 'completed') return 'rf-badge rf-badge--ok'
  if (status.includes('approval') || status === 'changes_requested' || status === 'pending') return 'rf-badge rf-badge--warn'
  if (status === 'failed' || status === 'cancelled' || status === 'rejected') return 'rf-badge rf-badge--danger'
  return 'rf-badge'
}

const validateWorkflow = async () => {
  if (!workflow.value || validationPending.value) return

  validationPending.value = true
  validationError.value = ''

  try {
    validationResult.value = await api.validateWorkflow(workflow.value.id)
  } catch (err: any) {
    validationError.value = err?.data?.error || err?.message || 'Could not validate workflow'
  } finally {
    validationPending.value = false
  }
}

const dryRunWorkflow = async () => {
  if (!workflow.value || dryRunPending.value) return

  dryRunPending.value = true
  dryRunError.value = ''

  try {
    const variablesPayload = Object.fromEntries(
      Object.entries(variableValues)
        .map(([key, value]) => [key, String(value || '').trim()])
        .filter(([, value]) => value)
    )

    dryRunResult.value = await api.dryRunWorkflow(workflow.value.id, variablesPayload)
  } catch (err: any) {
    dryRunError.value = err?.data?.error || err?.message || 'Could not run dry-run'
  } finally {
    dryRunPending.value = false
  }
}

const createRun = async () => {
  if (!workflow.value || submitPending.value) return

  if (missingRequiredVariables.value.length > 0) {
    submitError.value = `Provide required variables: ${missingRequiredVariables.value.join(', ')}`
    return
  }

  submitPending.value = true
  submitError.value = ''
  createdRun.value = null

  try {
    const variablesPayload = Object.fromEntries(
      Object.entries(variableValues)
        .map(([key, value]) => [key, String(value || '').trim()])
        .filter(([, value]) => value)
    )

    const run = await api.createRun(workflow.value.id, variablesPayload)
    createdRun.value = run
    await refresh()
  } catch (err: any) {
    submitError.value = err?.data?.error || err?.message || 'Could not create run'
  } finally {
    submitPending.value = false
  }
}
</script>

<template>
  <div class="space-y-6">
    <section class="rf-card">
      <div class="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
        <div>
          <NuxtLink to="/workflows" class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)] hover:text-cyan-200">Workflow catalog</NuxtLink>
          <h1 class="mt-2 text-3xl font-semibold">{{ workflow?.name || workflowId }}</h1>
          <p class="mt-3 max-w-4xl text-sm text-[color:var(--rf-muted)]">
            Workflow package detail, readiness checks, launch controls, and recent mission history for one operational definition.
          </p>
          <p v-if="workflow" class="mt-3 text-sm text-white/90">{{ packageHeadline }}</p>
        </div>
        <div class="flex flex-wrap gap-3">
          <NuxtLink to="/" class="rf-button">Open cockpit</NuxtLink>
          <button class="rf-button" :disabled="pending" @click="refresh()">Refresh package</button>
        </div>
      </div>
    </section>

    <section v-if="pending" class="grid gap-6 xl:grid-cols-[minmax(0,1.45fr)_360px]">
      <div class="space-y-3">
        <div class="rf-skeleton h-40 rounded-2xl" />
        <div class="rf-skeleton h-56 rounded-2xl" />
        <div class="rf-skeleton h-56 rounded-2xl" />
      </div>
      <div class="space-y-3">
        <div class="rf-skeleton h-72 rounded-2xl" />
        <div class="rf-skeleton h-64 rounded-2xl" />
      </div>
    </section>

    <section v-else-if="loadError" class="rf-card border-rose-400/30 bg-rose-500/10 text-rose-100">
      Could not load workflow {{ workflowId }}. {{ loadError }}
    </section>

    <section v-else-if="!workflow" class="rf-card text-sm text-[color:var(--rf-muted)]">
      Workflow {{ workflowId }} was not found.
    </section>

    <section v-else class="grid gap-6 xl:grid-cols-[minmax(0,1.45fr)_360px]">
      <section class="space-y-6 min-w-0">
        <article class="rf-card">
          <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Operational package</div>
                <span class="rf-badge">v{{ workflow.version || 'n/a' }}</span>
              </div>
              <p class="mt-3 text-sm text-[color:var(--rf-muted)]">{{ workflow.description || 'No description provided.' }}</p>
            </div>
            <div class="lg:max-w-[18rem] lg:text-right">
              <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Launch stance</div>
              <p class="mt-2 text-sm text-cyan-200">{{ launchDecisionSummary }}</p>
            </div>
          </div>

          <div class="mt-5">
            <OperationalFactGrid :facts="packageOverviewFacts" :columns="4" />
          </div>

          <div class="mt-5">
            <OperationalFactGrid :facts="packageIdentityFacts" :columns="4" />
          </div>
        </article>

        <AttentionContextCard
          title="Launch readiness command"
          :summary="launchDecisionSummary"
          :facts="variableReadinessFacts"
          :bullets="[
            'Validation answers whether the workflow contract is structurally sound.',
            'Dry-run answers whether launch looks operationally ready with the current inputs.',
            'Dependency signals tell you where prompts, files, documents, or URLs may deserve extra trust scrutiny before launch.'
          ]"
          :links="[
            { label: 'Open cockpit', to: '/' },
            { label: 'Open resource intelligence', to: '/assets' }
          ]"
        />

        <article class="rf-card">
          <div class="border-b border-[color:var(--rf-border)] pb-3">
            <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Mission design</div>
            <h2 class="mt-2 text-xl font-semibold">Execution sequence</h2>
            <p class="mt-2 text-sm text-[color:var(--rf-muted)]">Read the workflow as a package of operational moves, not just a list of YAML steps.</p>
          </div>

          <p v-if="steps.length === 0" class="mt-4 text-sm text-[color:var(--rf-muted)]">
            No executable steps are currently defined.
          </p>

          <ol v-else class="mt-4 space-y-3">
            <li
              v-for="(step, index) in steps"
              :key="step.id || `${index}`"
              class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4"
            >
              <div class="flex flex-wrap items-center gap-2">
                <strong>{{ step.id || `step-${index + 1}` }}</strong>
                <span class="rf-badge">{{ step.kind || 'unknown' }}</span>
                <span v-if="step.agent" class="rf-badge rf-badge--ok">{{ step.agent }}</span>
                <span v-if="step.approverPolicy" class="rf-badge rf-badge--warn">{{ step.approverPolicy }}</span>
              </div>
              <p class="mt-2 text-sm text-white/90">{{ missionStepSummary(step, index) }}</p>
              <p class="mt-2 text-sm text-[color:var(--rf-muted)]">{{ missionStepDetail(step) }}</p>
            </li>
          </ol>
        </article>

        <article class="rf-card">
          <div class="border-b border-[color:var(--rf-border)] pb-3">
            <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Input contract</div>
            <h2 class="mt-2 text-xl font-semibold">Variables and resource signals</h2>
            <p class="mt-2 text-sm text-[color:var(--rf-muted)]">This is where launch intent becomes explicit. Required inputs, dependency-sensitive variables, and missing trust context should be visible before a run exists.</p>
          </div>

          <p v-if="variables.length === 0" class="mt-4 text-sm text-[color:var(--rf-muted)]">
            This workflow currently defines no variables.
          </p>

          <div v-else class="mt-4 grid gap-4 xl:grid-cols-[minmax(0,1.3fr)_minmax(280px,0.9fr)]">
            <ul class="space-y-3">
              <li
                v-for="variable in variables"
                :key="variable.key"
                class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4"
              >
                <div class="flex items-center justify-between gap-4">
                  <code class="font-semibold">{{ variable.key }}</code>
                  <div class="flex flex-wrap gap-2">
                    <span class="rf-badge">{{ variable.type || 'unknown' }}</span>
                    <span v-if="variable.required" class="rf-badge rf-badge--warn">required</span>
                  </div>
                </div>
                <p class="mt-2 text-sm text-[color:var(--rf-muted)]">{{ variable.description || 'No description provided.' }}</p>
              </li>
            </ul>

            <div class="space-y-4">
              <div class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
                <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Dependency-sensitive signals</div>
                <div v-if="dependencySignals.length === 0" class="mt-3 text-sm text-[color:var(--rf-muted)]">
                  No obvious resource-sensitive variable signatures were detected from current naming and descriptions.
                </div>
                <ul v-else class="mt-3 space-y-3 text-sm text-[color:var(--rf-muted)]">
                  <li v-for="signal in dependencySignals" :key="signal.key" class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-3">
                    <div class="flex flex-wrap items-center gap-2">
                      <code class="font-semibold text-white/90">{{ signal.key }}</code>
                      <span class="rf-badge">{{ signal.type }}</span>
                      <span v-if="signal.required" class="rf-badge rf-badge--warn">required</span>
                    </div>
                    <p class="mt-2 text-xs">{{ signal.reason }}</p>
                  </li>
                </ul>
              </div>

              <div class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
                <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Truth boundary</div>
                <p class="mt-3 text-sm text-[color:var(--rf-muted)]">
                  This page can detect likely resource-sensitive inputs from workflow contracts, but it still cannot prove asset lineage or version pinning. That remains backend work.
                </p>
              </div>
            </div>
          </div>
        </article>

        <AttentionContextCard
          v-if="createdRun"
          kicker="Launch bridge"
          title="Run created — go straight to the mission view"
          :summary="createdRunSummary"
          :facts="createdRunFacts"
          :bullets="[
            'The package page answered readiness and input questions. The mission view answers execution questions.',
            'Use the new run page for worker state, step progression, Git isolation, and governance handoff.',
            'If this run immediately enters approval pressure, jump from the mission view into the governance queue with context intact.'
          ]"
          :links="[
            { label: 'Open new run mission', to: `/runs/${createdRunId}` },
            { label: 'Open governance queue', to: '/approvals' }
          ]"
        />

        <article class="rf-card">
          <div class="border-b border-[color:var(--rf-border)] pb-3">
            <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Operational history</div>
            <h2 class="mt-2 text-xl font-semibold">Recent runs for this package</h2>
            <p class="mt-2 text-sm text-[color:var(--rf-muted)]">Use live and recent runs to judge whether you are launching into a calm system or an already-busy one.</p>
          </div>

          <div v-if="recentRuns.length === 0" class="mt-4 rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-6 text-sm text-[color:var(--rf-muted)]">
            No runs have been recorded for this package yet.
          </div>

          <div v-else class="mt-4 space-y-3">
            <NuxtLink
              v-for="run in recentRuns.slice(0, 8)"
              :key="run.id"
              :to="`/runs/${run.id}`"
              class="block rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4 transition hover:border-cyan-400/40 hover:bg-black/20"
            >
              <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                <div>
                  <div class="flex flex-wrap items-center gap-2">
                    <strong>{{ run.id }}</strong>
                    <span :class="statusTone(run.status)">{{ run.status }}</span>
                    <span v-if="createdRunId && run.id === createdRunId" class="rf-badge rf-badge--ok">new launch</span>
                  </div>
                  <div class="mt-2 text-sm text-[color:var(--rf-muted)]">Current step {{ run.currentStep || '—' }}</div>
                  <div class="mt-2 text-xs text-[color:var(--rf-muted)]">Latest change {{ runLastChangedLabel(run) }}</div>
                </div>
                <div class="md:max-w-[18rem] md:text-right">
                  <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Git branch</div>
                  <div class="mt-2 break-all font-mono text-xs text-white/90">{{ run.branch || '—' }}</div>
                </div>
              </div>
            </NuxtLink>
          </div>
        </article>
      </section>

      <aside class="space-y-6">
        <section class="rf-card">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Launch control</div>
          <h2 class="mt-2 text-lg font-semibold">Create run</h2>
          <p class="mt-2 text-sm text-[color:var(--rf-muted)]">
            Launches a real run via API and prepares runtime directories for this workflow.
          </p>

          <div v-if="variables.length" class="mt-4 space-y-4">
            <div v-for="variable in variables" :key="`input-${variable.key}`" class="space-y-2">
              <label :for="`variable-${variable.key}`" class="block text-sm font-medium text-[color:var(--rf-text)]">
                {{ variable.key }}
                <span v-if="variable.required" class="ml-1 text-rose-300">*</span>
              </label>
              <input
                :id="`variable-${variable.key}`"
                v-model="variableValues[variable.key]"
                type="text"
                class="w-full rounded-xl border border-[color:var(--rf-border)] bg-black/20 px-4 py-3 text-sm text-[color:var(--rf-text)] outline-none transition focus:border-cyan-300/40 focus:ring-2 focus:ring-cyan-300/15"
                :placeholder="variable.description || variable.key"
              >
              <p class="text-xs text-[color:var(--rf-muted)]">
                {{ variable.type || 'unknown' }}{{ variable.required ? ' · required' : ' · optional' }}
              </p>
            </div>
          </div>

          <div
            v-if="missingRequiredVariables.length"
            class="mt-4 rounded-xl border border-amber-300/25 bg-amber-500/10 p-3 text-sm text-amber-100"
          >
            Required before launch: {{ missingRequiredVariables.join(', ') }}
          </div>

          <button class="rf-button mt-4 w-full" :disabled="!canCreateRun" @click="createRun()">
            {{ submitPending ? 'Creating run…' : 'Create run' }}
          </button>

          <p v-if="submitError" class="mt-3 text-sm text-rose-200">{{ submitError }}</p>

          <div v-if="createdRun" class="mt-3 rounded-xl border border-emerald-300/30 bg-emerald-500/10 p-3 text-sm text-emerald-100">
            <p class="font-medium">Run created: <NuxtLink :to="`/runs/${createdRunId}`" class="underline">{{ createdRunId }}</NuxtLink></p>
            <p class="mt-2 text-emerald-50/90">{{ createdRunSummary }}</p>
          </div>
        </section>

        <section class="rf-card">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Readiness checks</div>
          <h2 class="mt-2 text-lg font-semibold">Validate before launch</h2>
          <div class="mt-4 grid gap-3">
            <button class="rf-button w-full" :disabled="validationPending" @click="validateWorkflow()">
              {{ validationPending ? 'Validating…' : 'Validate workflow' }}
            </button>
            <button class="rf-button w-full" :disabled="dryRunPending" @click="dryRunWorkflow()">
              {{ dryRunPending ? 'Planning…' : 'Dry run' }}
            </button>
          </div>

          <p v-if="validationError" class="mt-3 text-sm text-rose-200">{{ validationError }}</p>
          <p v-if="dryRunError" class="mt-3 text-sm text-rose-200">{{ dryRunError }}</p>

          <div
            v-if="validationResult"
            class="mt-4 rounded-2xl border p-4 text-sm"
            :class="validationResult.valid ? 'border-emerald-300/30 bg-emerald-500/10 text-emerald-100' : 'border-amber-300/25 bg-amber-500/10 text-amber-100'"
          >
            <p class="font-medium">
              {{ validationResult.valid ? 'Workflow contract is valid.' : 'Workflow contract has issues that must be fixed.' }}
            </p>
            <ul v-if="validationResult.errors.length" class="mt-3 space-y-2">
              <li v-for="issue in validationResult.errors" :key="`${issue.field}-${issue.code}`">
                {{ issue.field }} · {{ issue.message }}
              </li>
            </ul>
            <p v-else class="mt-3">No validation errors reported.</p>
          </div>

          <div
            v-if="dryRunResult"
            class="mt-4 rounded-2xl border p-4 text-sm"
            :class="dryRunResult.ready ? 'border-emerald-300/30 bg-emerald-500/10 text-emerald-100' : 'border-cyan-300/25 bg-cyan-500/10 text-cyan-100'"
          >
            <p class="font-medium">
              {{ dryRunResult.ready ? 'Dry-run says this workflow is ready for launch.' : 'Dry-run found launch blockers or pending requirements.' }}
            </p>
            <p v-if="dryRunResult.missingVariables.length" class="mt-3">
              Missing required variables: {{ dryRunResult.missingVariables.join(', ') }}
            </p>
            <p v-else class="mt-3">All required variables currently provided.</p>

            <ul class="mt-3 space-y-2">
              <li v-for="step in dryRunResult.steps" :key="`dry-${step.id}`">
                {{ step.id }} · {{ step.kind }}<span v-if="step.blocking"> · blocking approval checkpoint</span><span v-else-if="step.agent"> · {{ step.agent }}</span>
              </li>
            </ul>
          </div>
        </section>

        <section class="rf-card">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Operator doctrine</div>
          <h2 class="mt-2 text-lg font-semibold">What this page is for</h2>
          <ul class="mt-4 space-y-3 text-sm text-[color:var(--rf-muted)]">
            <li>Judge package readiness before you create another mission.</li>
            <li>See whether approvals, failures, or live runs already make this package a hot zone.</li>
            <li>Move from definition to action without pretending hidden context exists.</li>
          </ul>
        </section>
      </aside>
    </section>
  </div>
</template>
