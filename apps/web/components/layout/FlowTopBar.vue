<script setup lang="ts">
import { AlertTriangle, GitBranch, Radio, ShieldCheck, Waypoints, Workflow } from 'lucide-vue-next'
import type { FlowApprovalRecord, FlowRun, FlowWorkflow } from '~/types/flow'

const route = useRoute()
const requestUrl = useRequestURL()
const config = useRuntimeConfig()
const api = useFlowApi()

const { data: shellData } = useAsyncData('flow-shell-summary', async () => {
  try {
    const [runs, approvals, workflows] = await Promise.all([
      api.getRuns(),
      api.getApprovals(),
      api.getWorkflows()
    ])

    return { runs, approvals, workflows, degraded: false }
  } catch {
    return {
      runs: [] as FlowRun[],
      approvals: [] as FlowApprovalRecord[],
      workflows: [] as FlowWorkflow[],
      degraded: true
    }
  }
}, {
  default: () => ({
    runs: [] as FlowRun[],
    approvals: [] as FlowApprovalRecord[],
    workflows: [] as FlowWorkflow[],
    degraded: false
  })
})

const runs = computed(() => shellData.value?.runs ?? [])
const approvals = computed(() => shellData.value?.approvals ?? [])
const workflows = computed(() => shellData.value?.workflows ?? [])
const degraded = computed(() => !!shellData.value?.degraded)

const activeRuns = computed(() => runs.value.filter((run) => run.status === 'running').length)
const stagedRuns = computed(() => runs.value.filter((run) => run.status === 'pending').length)
const attentionRuns = computed(() => runs.value.filter((run) => ['waiting_for_approval', 'changes_requested', 'failed', 'pending'].includes(run.status)).length)
const approvalPressure = computed(() => approvals.value.filter((approval) => ['pending', 'changes_requested'].includes(approval.status)).length)
const hostLabel = computed(() => requestUrl.host || 'local control surface')
const shellName = computed(() => config.public.appName || 'Ralleh Flow')

const shellView = computed(() => {
  if (route.path === '/') {
    return {
      label: 'Operations cockpit',
      summary: 'Triage first: what needs attention, what is live, and what changed most recently.'
    }
  }
  if (route.path === '/workflows') {
    return {
      label: 'Workflow packages',
      summary: 'Scan operational packages by pressure, readiness context, and recent mission activity.'
    }
  }
  if (route.path.startsWith('/workflows/')) {
    return {
      label: 'Workflow package detail',
      summary: 'Judge readiness, inspect recent runs, and launch deliberately from current API truth.'
    }
  }
  if (route.path === '/approvals') {
    return {
      label: 'Governance queue',
      summary: 'Human decisions live here: approvals, rejections, change requests, and resumed gates.'
    }
  }
  if (route.path === '/assets') {
    return {
      label: 'Resource intelligence',
      summary: 'Track provenance and future dependency truth without pretending the registry is already shipped.'
    }
  }
  if (route.path.startsWith('/runs/')) {
    return {
      label: 'Run mission view',
      summary: 'One mission at a time: progression, approvals, Git context, worker state, and outcomes.'
    }
  }

  return {
    label: 'Operational surface',
    summary: 'Read the system by current state, intervention pressure, and truth boundaries.'
  }
})

const signalLabel = computed(() => degraded.value ? 'Partial signal' : 'API synced')
</script>

<template>
  <header class="sticky top-0 z-20 border-b border-[color:var(--rf-border)] bg-[color:var(--rf-surface)]/95 px-4 py-4 backdrop-blur lg:px-6">
    <div class="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
      <div class="min-w-0">
        <div class="flex flex-wrap items-center gap-2 text-xs uppercase tracking-[0.28em] text-[color:var(--rf-muted)]">
          <span>{{ shellName }}</span>
          <span>•</span>
          <span>{{ hostLabel }}</span>
        </div>
        <h2 class="mt-2 text-xl font-semibold">{{ shellView.label }}</h2>
        <p class="mt-2 max-w-3xl text-sm text-[color:var(--rf-muted)]">{{ shellView.summary }}</p>
      </div>

      <div class="grid gap-3 sm:grid-cols-2 xl:w-[44rem] xl:grid-cols-4">
        <div class="rf-shell-kpi">
          <div class="rf-shell-kpi__label"><Waypoints class="h-4 w-4" /> Attention</div>
          <div class="rf-shell-kpi__value">{{ attentionRuns }}</div>
          <div class="rf-shell-kpi__hint">Runs needing intervention</div>
        </div>

        <div class="rf-shell-kpi">
          <div class="rf-shell-kpi__label"><Workflow class="h-4 w-4" /> Live</div>
          <div class="rf-shell-kpi__value">{{ activeRuns }}</div>
          <div class="rf-shell-kpi__hint">Operations executing now</div>
        </div>

        <div class="rf-shell-kpi">
          <div class="rf-shell-kpi__label"><ShieldCheck class="h-4 w-4" /> Governance</div>
          <div class="rf-shell-kpi__value">{{ approvalPressure }}</div>
          <div class="rf-shell-kpi__hint">Approval gates in play</div>
        </div>

        <div class="rf-shell-kpi">
          <div class="rf-shell-kpi__label"><Radio class="h-4 w-4" /> Signal</div>
          <div class="rf-shell-kpi__value text-lg">{{ signalLabel }}</div>
          <div class="rf-shell-kpi__hint">{{ workflows.length }} packages · {{ stagedRuns }} staged</div>
        </div>
      </div>
    </div>

    <div class="mt-4 flex flex-wrap items-center gap-2 text-xs text-[color:var(--rf-muted)]">
      <span class="rf-pill"><AlertTriangle class="h-4 w-4" /> {{ attentionRuns }} operator attention</span>
      <span class="rf-pill"><GitBranch class="h-4 w-4" /> Git trust stays on each run mission view</span>
      <span class="rf-pill"><Workflow class="h-4 w-4" /> {{ workflows.length }} operational packages</span>
    </div>
  </header>
</template>
