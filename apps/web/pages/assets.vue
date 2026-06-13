<script setup lang="ts">
import type { FlowRun, FlowWorkflowDetail } from '~/types/flow'

const api = useFlowApi()

const dependencyKeywords = ['asset', 'file', 'path', 'url', 'uri', 'image', 'doc', 'document', 'prompt', 'template', 'input', 'source', 'artifact']

type AssetSurfacePayload = {
  workflows: FlowWorkflowDetail[]
  runs: FlowRun[]
  degraded: boolean
}

type WorkflowDependencySignal = {
  workflow: FlowWorkflowDetail
  matchedVariables: Array<{
    key: string
    reason: string
  }>
  relatedRuns: FlowRun[]
}

const { data, pending, error, refresh } = await useAsyncData<AssetSurfacePayload>('flow-assets-surface', async () => {
  try {
    const [workflowList, runs] = await Promise.all([
      api.getWorkflows(),
      api.getRuns()
    ])

    const workflows = await Promise.all(
      workflowList.map(async (workflow) => {
        try {
          return await api.getWorkflow(workflow.id)
        } catch {
          return {
            ...workflow,
            variables: [],
            steps: []
          }
        }
      })
    )

    return {
      workflows,
      runs,
      degraded: false
    }
  } catch {
    return {
      workflows: [],
      runs: [],
      degraded: true
    }
  }
}, {
  default: () => ({
    workflows: [],
    runs: [],
    degraded: false
  })
})

const workflows = computed(() => data.value?.workflows ?? [])
const runs = computed(() => data.value?.runs ?? [])
const degraded = computed(() => !!data.value?.degraded)

const detectDependencySignals = (workflow: FlowWorkflowDetail) => {
  return workflow.variables.flatMap((variable) => {
    const haystack = [variable.key, variable.type, variable.description]
      .filter(Boolean)
      .join(' ')
      .toLowerCase()

    const matches = dependencyKeywords.filter((keyword) => haystack.includes(keyword))
    if (matches.length === 0) return []

    return [{
      key: variable.key,
      reason: `${variable.type || 'unknown'} variable hints at ${matches.slice(0, 2).join(' / ')} dependency`
    }]
  })
}

const dependencySensitivePackages = computed<WorkflowDependencySignal[]>(() => {
  return workflows.value
    .map((workflow) => {
      const matchedVariables = detectDependencySignals(workflow)
      const relatedRuns = runs.value.filter((run) => run.workflowId === workflow.id)
      return {
        workflow,
        matchedVariables,
        relatedRuns
      }
    })
    .filter((item) => item.matchedVariables.length > 0)
    .sort((a, b) => {
      const aPressure = a.relatedRuns.filter((run) => ['pending', 'waiting_for_approval', 'changes_requested', 'failed'].includes(run.status)).length
      const bPressure = b.relatedRuns.filter((run) => ['pending', 'waiting_for_approval', 'changes_requested', 'failed'].includes(run.status)).length
      return bPressure - aPressure || b.matchedVariables.length - a.matchedVariables.length
    })
})

const dependencySensitiveIds = computed(() => new Set(dependencySensitivePackages.value.map((item) => item.workflow.id)))
const dependencySensitiveRuns = computed(() => runs.value.filter((run) => dependencySensitiveIds.value.has(run.workflowId)))
const readinessPressureRuns = computed(() => dependencySensitiveRuns.value.filter((run) => ['pending', 'waiting_for_approval', 'changes_requested', 'failed'].includes(run.status)))
const liveDependencyRuns = computed(() => dependencySensitiveRuns.value.filter((run) => ['running', 'pending'].includes(run.status)))
const workflowsWithVariables = computed(() => workflows.value.filter((workflow) => workflow.variables.length > 0))
const blindPackages = computed(() => workflowsWithVariables.value.filter((workflow) => !dependencySensitiveIds.value.has(workflow.id)))

const surfaceFacts = computed(() => [
  { label: 'Dependency-sensitive packages', value: String(dependencySensitivePackages.value.length), tone: dependencySensitivePackages.value.length ? 'warn' : 'default' },
  { label: 'Runs under readiness pressure', value: String(readinessPressureRuns.value.length), tone: readinessPressureRuns.value.length ? 'warn' : 'ok' },
  { label: 'Live runs with input risk', value: String(liveDependencyRuns.value.length), tone: liveDependencyRuns.value.length ? 'warn' : 'default' },
  { label: 'Signal', value: degraded.value ? 'Partial' : 'API synced', tone: degraded.value ? 'danger' : 'ok' }
] as const)

const packagePressureLabel = (item: WorkflowDependencySignal) => {
  const pressuredRuns = item.relatedRuns.filter((run) => ['pending', 'waiting_for_approval', 'changes_requested', 'failed'].includes(run.status)).length
  const liveRuns = item.relatedRuns.filter((run) => ['running', 'pending'].includes(run.status)).length

  if (pressuredRuns > 0) return `${pressuredRuns} run(s) currently need deliberate operator review before input trust should be assumed.`
  if (liveRuns > 0) return `${liveRuns} live or staged run(s) depend on variables that look resource-sensitive.`
  if (item.relatedRuns.length > 0) return `${item.relatedRuns.length} prior run(s) exist, but none currently signal readiness pressure.`
  return 'No runs recorded yet, so this package should be checked at launch time rather than trusted by default.'
}

const runStatusTone = (status: string) => {
  if (status === 'running' || status === 'completed') return 'rf-badge rf-badge--ok'
  if (status === 'waiting_for_approval' || status === 'changes_requested' || status === 'pending') return 'rf-badge rf-badge--warn'
  if (status === 'failed' || status === 'cancelled' || status === 'rejected') return 'rf-badge rf-badge--danger'
  return 'rf-badge'
}

const readinessHeadline = computed(() => {
  if (readinessPressureRuns.value.length > 0) {
    return 'Some active workflows already look sensitive to missing or untrusted inputs. Use this page to spot those packages before pretending an asset registry exists.'
  }
  if (dependencySensitivePackages.value.length > 0) {
    return 'The system exposes packages whose variables look resource-sensitive, but there is still no first-class registry proving those resources are ready.'
  }
  return 'No obvious dependency-sensitive package signatures were detected from current workflow variable contracts.'
})

const truthBoundaries = [
  'There is still no shipped asset ingest or asset registry API.',
  'This screen uses workflow variable contracts as a proxy for resource sensitivity; it does not claim real provenance.',
  'Readiness pressure here is inferred from run status plus variable signatures, not from actual asset validation.',
  'Pinned versions, lineage graphs, and per-run asset manifests remain backend work, not hidden UI state.'
]

const nextSlice = [
  'Expose real asset records and asset-to-run pinning from the backend.',
  'Show lineage and version history on the run mission view when a run consumes external resources.',
  'Block launch with explicit readiness states when required inputs are missing, invalid, or stale.'
]
</script>

<template>
  <div class="space-y-6">
    <section class="rf-card">
      <div class="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Resource intelligence</p>
          <h1 class="mt-3 text-2xl font-semibold md:text-3xl">Input readiness and provenance pressure</h1>
          <p class="mt-3 max-w-4xl text-sm text-[color:var(--rf-muted)]">
            Until the backend ships a real asset registry, this surface should still help an operator identify which workflow packages are most likely to depend on external resources, where readiness pressure is accumulating, and what truth is still missing.
          </p>
        </div>
        <div class="flex flex-wrap gap-3">
          <NuxtLink to="/workflows" class="rf-button">Open workflow packages</NuxtLink>
          <button class="rf-button" :disabled="pending" @click="refresh()">Refresh resource view</button>
        </div>
      </div>
    </section>

    <AttentionContextCard
      title="What this page can honestly tell the operator"
      :summary="readinessHeadline"
      :facts="surfaceFacts"
      :bullets="[
        'Packages are flagged here only when workflow variable contracts suggest external files, documents, prompts, URLs, or other resource-like inputs.',
        'Readiness pressure means the workflow package deserves inspection before launch or approval—not that the system has already validated the resource.',
        'This is a dependency-awareness surface today, not a fake upload console.'
      ]"
      :links="[
        { label: 'Open cockpit', to: '/' },
        { label: 'Open approvals', to: '/approvals' }
      ]"
    />

    <section v-if="error" class="rf-card border-rose-400/30 bg-rose-500/10 text-rose-100">
      Could not load resource intelligence context. {{ error.message }}
    </section>

    <section class="grid gap-6 xl:grid-cols-[minmax(0,1.25fr)_340px]">
      <section class="rf-card min-w-0">
        <div class="border-b border-[color:var(--rf-border)] pb-3">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Operational pressure</div>
          <h2 class="mt-2 text-xl font-semibold">Packages most likely to care about input trust</h2>
          <p class="mt-2 text-sm text-[color:var(--rf-muted)]">
            Ranked by current run pressure first, then by how many variable contracts look resource-sensitive.
          </p>
        </div>

        <div v-if="pending" class="mt-4 space-y-3">
          <div v-for="n in 4" :key="n" class="rf-skeleton h-28 rounded-2xl" />
        </div>

        <div v-else-if="dependencySensitivePackages.length === 0" class="mt-4 rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-6 text-sm text-[color:var(--rf-muted)]">
          No dependency-sensitive package signatures were detected from current workflow variable contracts.
        </div>

        <div v-else class="mt-4 space-y-4">
          <article
            v-for="item in dependencySensitivePackages"
            :key="item.workflow.id"
            class="rounded-[1.6rem] border border-[color:var(--rf-border)] bg-black/10 p-5"
          >
            <div class="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
              <div class="min-w-0 flex-1">
                <div class="flex flex-wrap items-center gap-2">
                  <h3 class="text-lg font-semibold">{{ item.workflow.name }}</h3>
                  <span class="rf-badge">{{ item.workflow.id }}</span>
                  <span class="rf-badge rf-badge--warn">{{ item.matchedVariables.length }} signal{{ item.matchedVariables.length === 1 ? '' : 's' }}</span>
                </div>

                <p class="mt-3 text-sm text-white/90">{{ packagePressureLabel(item) }}</p>

                <ul class="mt-4 space-y-2 text-sm text-[color:var(--rf-muted)]">
                  <li
                    v-for="variable in item.matchedVariables.slice(0, 4)"
                    :key="`${item.workflow.id}-${variable.key}`"
                    class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-3"
                  >
                    <div class="flex flex-wrap items-center gap-2">
                      <code class="font-semibold text-white/90">{{ variable.key }}</code>
                      <span class="rf-badge">contract signal</span>
                    </div>
                    <p class="mt-2 text-xs">{{ variable.reason }}</p>
                  </li>
                </ul>
              </div>

              <div class="xl:w-[22rem] xl:min-w-[22rem] space-y-3">
                <div class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
                  <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Next operator move</div>
                  <p class="mt-2 text-sm text-[color:var(--rf-muted)]">
                    Check the package detail before launch so missing documents, prompts, files, or URLs do not get discovered too late inside a run.
                  </p>
                  <NuxtLink :to="`/workflows/${item.workflow.id}`" class="rf-button mt-4 w-full justify-center">Open package detail</NuxtLink>
                </div>

                <div class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
                  <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Run pressure</div>
                  <div class="mt-3 flex flex-wrap gap-2">
                    <span
                      v-for="run in item.relatedRuns.slice(0, 4)"
                      :key="run.id"
                      :class="runStatusTone(run.status)"
                    >
                      {{ run.status }} · {{ run.id }}
                    </span>
                    <span v-if="item.relatedRuns.length === 0" class="rf-badge">No runs yet</span>
                  </div>
                </div>
              </div>
            </div>
          </article>
        </div>
      </section>

      <aside class="space-y-6">
        <section class="rf-card">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Blind spots</div>
          <h2 class="mt-2 text-lg font-semibold">Where the current API still goes dark</h2>
          <ul class="mt-4 space-y-3 text-sm text-[color:var(--rf-muted)]">
            <li class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-3">
              {{ workflowsWithVariables.length }} package(s) expose variables, but only {{ dependencySensitivePackages.length }} look resource-sensitive from naming and descriptions.
            </li>
            <li class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-3">
              {{ blindPackages.length }} package(s) may still depend on assets implicitly even though their contracts do not say so clearly.
            </li>
            <li class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-3">
              {{ dependencySensitiveRuns.length }} recorded run(s) belong to dependency-sensitive packages, but none expose actual asset manifests yet.
            </li>
          </ul>
        </section>

        <section class="rf-card">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Truth boundaries</div>
          <h2 class="mt-2 text-lg font-semibold">What is not being faked</h2>
          <ul class="mt-4 space-y-3 text-sm text-[color:var(--rf-muted)]">
            <li v-for="item in truthBoundaries" :key="item" class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-3">
              {{ item }}
            </li>
          </ul>
        </section>

        <section class="rf-card">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Next real slice</div>
          <h2 class="mt-2 text-lg font-semibold">Backend work this UI is pointing at</h2>
          <ul class="mt-4 space-y-3 text-sm text-[color:var(--rf-muted)]">
            <li v-for="item in nextSlice" :key="item" class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-3">
              {{ item }}
            </li>
          </ul>
        </section>
      </aside>
    </section>
  </div>
</template>
