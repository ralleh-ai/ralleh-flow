<script setup lang="ts">
import type { FlowApprovalRecord, FlowHandoffRecord, FlowRun, FlowStepRecord, FlowTimelineEvent, FlowWorkflowDetail, FlowWorkflowStep } from '~/types/flow'
import type { OperationalFact } from '~/types/ui'
import OperationalFactGrid from '~/components/operations/OperationalFactGrid.vue'

type RunPagePayload = {
  run: FlowRun | null
  workflow: FlowWorkflowDetail | null
  approvals: FlowApprovalRecord[]
}

type StepMissionState = 'completed' | 'running' | 'awaiting_approval' | 'changes_requested' | 'failed' | 'queued'

const emptyRunPayload = (): RunPagePayload => ({
  run: null,
  workflow: null,
  approvals: []
})

const route = useRoute()
const api = useFlowApi()

const runId = computed(() => String(route.params.runId || ''))

const loadError = ref('')
const workflowError = ref('')
const advanceError = ref('')
const stepActionError = ref('')
const advancing = ref(false)
const dispatching = ref(false)
const completing = ref(false)
const failing = ref(false)
const resuming = ref(false)

const { data, pending, refresh } = await useAsyncData<RunPagePayload>(
  () => `flow-run-${runId.value}`,
  async (): Promise<RunPagePayload> => {
    if (!runId.value) {
      loadError.value = 'Missing run id'
      return emptyRunPayload()
    }

    try {
      loadError.value = ''
      workflowError.value = ''
      const run = await api.getRun(runId.value)

      let workflow: FlowWorkflowDetail | null = null
      try {
        workflow = await api.getWorkflow(run.workflowId)
      } catch (err: any) {
        workflowError.value = err?.data?.error || err?.message || 'Could not load workflow context'
      }

      let approvals: FlowApprovalRecord[] = []
      try {
        approvals = await api.getApprovals()
      } catch {
        approvals = []
      }

      return { run, workflow, approvals }
    } catch (err: any) {
      loadError.value = err?.data?.error || err?.message || 'Could not load run'
      return emptyRunPayload()
    }
  },
  {
    watch: [runId],
    default: emptyRunPayload
  }
)

const run = ref<FlowRun | null>(null)
const workflow = ref<FlowWorkflowDetail | null>(null)
const approvals = ref<FlowApprovalRecord[]>([])

watch(data, (payload) => {
  const next = payload ?? emptyRunPayload()
  run.value = next.run
  workflow.value = next.workflow
  approvals.value = next.approvals
}, { immediate: true })
const timeline = computed(() => run.value?.timeline ?? [])
const steps = computed(() => run.value?.steps ?? [])
const handoffs = computed(() => run.value?.handoffs ?? [])
const workflowSteps = computed(() => workflow.value?.steps ?? [])
const latestEvent = computed(() => timeline.value[timeline.value.length - 1] ?? null)
const approvalEvents = computed(() => timeline.value.filter((event) => event.type.includes('approval')))
const activeStep = computed(() => steps.value.find((step) => step.status === 'running') || null)
const activeHandoff = computed(() => handoffs.value.find((handoff) => handoff.stepId === activeStep.value?.stepId) || null)
const canAdvance = computed(() => run.value?.status === 'pending')
const canResume = computed(() => run.value?.status === 'changes_requested')
const canDispatchStep = computed(() => !!activeStep.value && run.value?.status === 'running' && activeHandoff.value?.status === 'claimed')
const canCompleteStep = computed(() => !!activeStep.value && run.value?.status === 'running')

const latestStepRecords = computed<Record<string, FlowStepRecord>>(() => {
  return steps.value.reduce<Record<string, FlowStepRecord>>((acc, step) => {
    const current = acc[step.stepId]
    if (!current) {
      acc[step.stepId] = step
      return acc
    }

    const currentTime = current.finishedAt || current.startedAt || ''
    const stepTime = step.finishedAt || step.startedAt || ''
    if (stepTime >= currentTime) {
      acc[step.stepId] = step
    }
    return acc
  }, {})
})

const currentWorkflowStepIndex = computed(() => {
  if (!run.value?.currentStep) return -1
  return workflowSteps.value.findIndex((step) => step.id === run.value?.currentStep)
})

const completedStepCount = computed(() => {
  if (workflowSteps.value.length === 0) return 0
  return workflowSteps.value.filter((step, index) => {
    const state = missionStepState(step, index)
    return state === 'completed'
  }).length
})

const missionProgressLabel = computed(() => {
  if (workflowSteps.value.length === 0) return 'Workflow sequence unavailable'
  return `${completedStepCount.value}/${workflowSteps.value.length} steps completed`
})

const runHeadline = computed(() => {
  if (!run.value) return 'Run not loaded'
  switch (run.value.status) {
    case 'running':
      return 'Operation is actively executing.'
    case 'waiting_for_approval':
      return 'Operation is paused at a deliberate approval gate.'
    case 'changes_requested':
      return 'Operation is paused for directed rework before approval can continue.'
    case 'pending':
      return 'Operation is staged and ready for the next execution step.'
    case 'completed':
      return 'Operation completed and outcome is recorded.'
    case 'failed':
      return 'Operation stalled and needs operator attention.'
    case 'cancelled':
      return 'Operation was cancelled before completion.'
    default:
      return 'Operational state loaded from current API truth.'
  }
})

const attentionSummary = computed(() => {
  if (!run.value) return 'No run loaded.'
  if (run.value.status === 'waiting_for_approval') return 'Attention required: approval decision needed before execution can continue.'
  if (run.value.status === 'changes_requested') return 'Attention required: rework was requested and the approval gate can now be resumed when ready.'
  if (run.value.status === 'failed') return 'Attention required: the active operation failed and should be reviewed before another step is triggered.'
  if (run.value.status === 'pending') return 'Operator can advance the run into the next execution step.'
  if (run.value.status === 'running') return 'Execution is live. Monitor agent activity, handoffs, and completion state.'
  return 'No immediate intervention required from current API state.'
})

const recentTimeline = computed(() => [...timeline.value].reverse())

const statusTone = (status: string) => {
  if (status === 'running' || status === 'completed' || status === 'approved') return 'rf-badge rf-badge--ok'
  if (status.includes('approval') || status === 'changes_requested' || status === 'pending') return 'rf-badge rf-badge--warn'
  if (status === 'failed' || status === 'cancelled' || status === 'rejected') return 'rf-badge rf-badge--danger'
  return 'rf-badge'
}

const stepStateTone = (state: StepMissionState) => {
  if (state === 'completed' || state === 'running') return 'rf-badge rf-badge--ok'
  if (state === 'awaiting_approval' || state === 'changes_requested' || state === 'queued') return 'rf-badge rf-badge--warn'
  if (state === 'failed') return 'rf-badge rf-badge--danger'
  return 'rf-badge'
}

const missionStepState = (step: FlowWorkflowStep, index: number): StepMissionState => {
  const record = latestStepRecords.value[step.id]

  if (record?.status === 'failed') return 'failed'
  if (record?.status === 'running') return 'running'
  if (record?.status === 'completed') return 'completed'

  if (run.value?.currentStep === step.id) {
    if (run.value.status === 'waiting_for_approval') return 'awaiting_approval'
    if (run.value.status === 'changes_requested') return 'changes_requested'
    if (run.value.status === 'running') return 'running'
    return 'queued'
  }

  if (currentWorkflowStepIndex.value !== -1 && index < currentWorkflowStepIndex.value) {
    return 'completed'
  }

  return 'queued'
}

const missionStepDetail = (step: FlowWorkflowStep, state: StepMissionState) => {
  if (state === 'completed') return 'Step finished and moved the operation forward.'
  if (state === 'running') return 'Step is currently executing with an active worker claim.'
  if (state === 'awaiting_approval') return 'This gate is waiting on an operator decision.'
  if (state === 'changes_requested') return 'Rework was requested before this gate can continue.'
  if (state === 'failed') return 'This step recorded a failure and needs intervention.'
  return 'Step is queued behind the current operational focus.'
}

const humanizeEvent = (event: FlowTimelineEvent) => {
  if (event.type.startsWith('approval.')) return 'Approval event'
  if (event.type.startsWith('run.')) return 'Run state change'
  if (event.type.startsWith('step.')) return 'Step change'
  return 'Operational event'
}

const handoffSummary = (handoff: FlowHandoffRecord) => {
  const parts = [handoff.kind || 'handoff']
  if (handoff.agent) parts.push(handoff.agent)
  if (handoff.sessionId) parts.push(handoff.sessionId)
  return parts.join(' · ')
}

const activeWorkerSummary = computed(() => {
  if (activeStep.value) {
    return `${activeStep.value.workerId} working ${activeStep.value.stepId}`
  }
  if (latestEvent.value?.type.includes('approval')) {
    return 'Approval gate is the active operational focus.'
  }
  return 'No active worker claim recorded.'
})

const activeHandoffLabel = computed(() => {
  if (!activeHandoff.value) return 'No active handoff record for the current focus.'

  const parts = [activeHandoff.value.status]
  if (activeHandoff.value.agent) parts.push(activeHandoff.value.agent)
  if (activeHandoff.value.sessionId) parts.push(activeHandoff.value.sessionId)
  return parts.join(' · ')
})

const dispatchReadinessSummary = computed(() => {
  if (!run.value) return 'Mission not loaded.'
  if (run.value.status === 'pending') return 'The mission is staged. Advance the run before any step can be dispatched.'
  if (!activeStep.value) return 'No active step is currently claiming execution focus.'
  if (!activeHandoff.value) return 'The active step has no persisted handoff record yet, so there is nothing explicit to dispatch.'
  if (activeHandoff.value.status === 'claimed') return 'The active step is claimed and ready for an explicit dispatch binding.'
  if (activeHandoff.value.status === 'dispatched') return 'Dispatch already happened. Watch completion or failure instead of rebinding blindly.'
  return `Active handoff is ${activeHandoff.value.status}. Operator should inspect the mission history before forcing the next move.`
})

const dispatchReadinessFacts = computed<OperationalFact[]>(() => {
  const facts: OperationalFact[] = [
    {
      label: 'Run state',
      value: run.value?.status || 'unknown',
      tone: run.value?.status === 'failed'
        ? 'danger'
        : ['waiting_for_approval', 'changes_requested', 'pending'].includes(run.value?.status || '')
          ? 'warn'
          : run.value?.status === 'running'
            ? 'ok'
            : 'default'
    },
    {
      label: 'Current focus',
      value: run.value?.currentStep || '—'
    }
  ]

  if (activeStep.value?.workerId) {
    facts.push({ label: 'Worker', value: activeStep.value.workerId, tone: 'ok' })
  }

  if (activeHandoff.value) {
    facts.push({
      label: 'Handoff',
      value: activeHandoff.value.status,
      tone: activeHandoff.value.status === 'claimed'
        ? 'warn'
        : activeHandoff.value.status === 'dispatched'
          ? 'ok'
          : activeHandoff.value.status === 'failed'
            ? 'danger'
            : 'default'
    })
  }

  return facts
})

const dispatchReadinessBullets = computed(() => {
  if (run.value?.status === 'pending') {
    return [
      'The run itself must be advanced before dispatch becomes a meaningful action.',
      'Pending is staging pressure, not worker activity.',
      'Use the top-level advance control first.'
    ]
  }

  if (!activeStep.value) {
    return [
      'No running step is claiming focus right now.',
      'If this seems wrong, inspect the mission timeline for the last transition rather than guessing missing worker state.'
    ]
  }

  if (!activeHandoff.value) {
    return [
      'A running step without a handoff record is an API truth gap, not a UI state to paper over.',
      'Check the timeline and step records before attempting another control action.'
    ]
  }

  if (activeHandoff.value.status === 'claimed') {
    return [
      'This is the cleanest moment to bind or dispatch the active step deliberately.',
      'The session binding shown here is still a placeholder path pending deeper execution integration.',
      'After dispatch, watch handoff and step checkpoints rather than assuming completion.'
    ]
  }

  if (activeHandoff.value.status === 'dispatched') {
    return [
      'The active step already has a dispatch record.',
      'Use completion or failure only when mission evidence supports it.',
      'Repeated dispatch without context would be operator guesswork.'
    ]
  }

  return [
    'Inspect handoff and step records before forcing another transition.',
    'This board exists to expose execution truth, not hide ambiguous state.'
  ]
})

const recentHandoff = computed(() => handoffs.value.slice().sort((a, b) => new Date(b.updatedAt || b.createdAt).getTime() - new Date(a.updatedAt || a.createdAt).getTime())[0] || null)
const recentCheckpoint = computed(() => steps.value.slice().sort((a, b) => {
  const aTime = new Date(a.finishedAt || a.startedAt).getTime()
  const bTime = new Date(b.finishedAt || b.startedAt).getTime()
  return bTime - aTime
})[0] || null)

const linkedApproval = computed<FlowApprovalRecord | null>(() => {
  if (!run.value) return null
  const runApprovals = approvals.value.filter((approval) => approval.runId === run.value?.id)
  const currentStepId = run.value.currentStep

  const exact = runApprovals.find((approval) => approval.stepId === currentStepId && ['pending', 'changes_requested', 'approved', 'rejected'].includes(approval.status))
  if (exact) return exact

  return runApprovals[0] || null
})

const governanceSummary = computed(() => {
  if (linkedApproval.value?.status === 'pending') {
    return 'This run is currently blocked by a live approval gate. The governance queue is the next place where a human decision changes the system.'
  }
  if (linkedApproval.value?.status === 'changes_requested') {
    return 'Governance already asked for rework. Confirm the rework in mission history, then resume the gate deliberately.'
  }
  if (linkedApproval.value?.status === 'rejected') {
    return 'Governance rejected this gate. Review the rationale before repeating the same run pattern.'
  }
  if (approvalEvents.value.length > 0) {
    return 'This run has governance history. Use the queue when you need cross-run approval context.'
  }
  return 'No linked approval record is visible from current API truth.'
})

const governanceFacts = computed(() => {
  const facts = [] as { label: string, value: string, tone?: 'default' | 'ok' | 'warn' | 'danger' }[]
  if (linkedApproval.value) {
    facts.push({
      label: 'Approval state',
      value: linkedApproval.value.status,
      tone: linkedApproval.value.status === 'rejected' ? 'danger' : linkedApproval.value.status === 'pending' || linkedApproval.value.status === 'changes_requested' ? 'warn' : linkedApproval.value.status === 'approved' ? 'ok' : 'default'
    })
    facts.push({ label: 'Gate', value: linkedApproval.value.stepId })
  }
  facts.push({ label: 'Approval events', value: String(approvalEvents.value.length), tone: approvalEvents.value.length ? 'warn' : 'default' })
  return facts
})

const approvalContextBullets = computed(() => {
  if (linkedApproval.value?.status === 'pending') {
    return [
      'Open the governance queue when you want the dedicated operator controls for this gate.',
      'Use the mission timeline here to validate whether the evidence and current step match the requested approval.',
      'Do not approve from memory; this mission view exists so the operator can inspect live context first.'
    ]
  }
  if (linkedApproval.value?.status === 'changes_requested') {
    return [
      'The run is paused for rework, not silently resumed.',
      'Check recent timeline and step records for evidence that the requested changes actually happened.',
      'Resume only when reopening the same gate is the right operational move.'
    ]
  }
  if (linkedApproval.value?.status === 'rejected') {
    return [
      'A rejection is part of mission history, not an implementation detail.',
      'Review rationale and timeline before creating another run that repeats the same failure pattern.'
    ]
  }
  if (approvalEvents.value.length > 0) {
    return [
      'This run has governance history even if no gate is active right now.',
      'Use the queue for broader cross-run governance context when you need it.'
    ]
  }
  return [
    'No approval object is linked from current API truth.',
    'If a gate should exist here, that gap belongs in the API contract—not in UI guesswork.'
  ]
})

const approvalDecisionLabel = computed(() => {
  const approval = linkedApproval.value
  if (!approval) return 'No linked decision record'

  const parts: string[] = []
  if (approval.decidedBy) parts.push(`Decision by ${approval.decidedBy}`)
  if (approval.decidedAt) parts.push(new Date(approval.decidedAt).toLocaleString())
  return parts.join(' · ') || 'Decision not recorded yet'
})

const evidenceManifestLabel = computed(() => {
  const manifest = linkedApproval.value?.evidenceManifest
  if (!manifest) return 'Not recorded yet'
  const parts = manifest.split('/')
  return parts[parts.length - 1] || manifest
})

const evidenceTruthSummary = computed(() => {
  if (linkedApproval.value?.evidenceManifest) {
    return 'This mission has a recorded evidence manifest path, plus timeline, handoff, checkpoint, and Git-isolation context. Diff and artifact registry exposure still need deeper backend support.'
  }
  if (linkedApproval.value) {
    return 'A governance object exists, but no evidence manifest path is recorded yet. The operator should rely more heavily on mission progression, Git isolation, and recent handoff/checkpoint truth.'
  }
  return 'No linked approval evidence object is visible yet. Timeline, handoff, checkpoint, and Git context are still the current trust surface.'
})

const evidenceBundleFacts = computed<OperationalFact[]>(() => [
  {
    label: 'Approval state',
    value: linkedApproval.value?.status || 'No linked approval',
    tone: linkedApproval.value?.status === 'rejected'
      ? 'danger'
      : linkedApproval.value?.status === 'pending' || linkedApproval.value?.status === 'changes_requested'
        ? 'warn'
        : linkedApproval.value?.status === 'approved'
          ? 'ok'
          : 'default',
    detail: linkedApproval.value ? `Gate ${linkedApproval.value.stepId}` : 'No governance object linked from the current run payload.'
  },
  {
    label: 'Evidence manifest',
    value: evidenceManifestLabel.value,
    tone: linkedApproval.value?.evidenceManifest ? 'ok' : 'warn',
    detail: linkedApproval.value?.evidenceManifest || 'Manifest path has not been persisted yet.'
  },
  {
    label: 'Decision record',
    value: approvalDecisionLabel.value,
    detail: linkedApproval.value?.rationale || 'No written rationale recorded in the current payload.'
  }
])

const artifactVisibilityFacts = computed<OperationalFact[]>(() => [
  {
    label: 'Timeline events',
    value: String(timeline.value.length),
    tone: timeline.value.length ? 'ok' : 'default',
    detail: latestEvent.value ? `${latestEvent.value.type} at ${new Date(latestEvent.value.at).toLocaleString()}` : 'No recorded event stream yet.'
  },
  {
    label: 'Step checkpoints',
    value: String(steps.value.length),
    tone: steps.value.length ? 'ok' : 'default',
    detail: recentCheckpoint.value ? `${recentCheckpoint.value.stepId} · ${recentCheckpoint.value.status}` : 'No persisted step checkpoints yet.'
  },
  {
    label: 'Dispatch handoffs',
    value: String(handoffs.value.length),
    tone: handoffs.value.length ? 'ok' : 'default',
    detail: recentHandoff.value ? `${recentHandoff.value.stepId} · ${recentHandoff.value.status}` : 'No persisted handoff records yet.'
  },
  {
    label: 'Artifact registry',
    value: 'Not exposed yet',
    tone: 'warn',
    detail: 'The API still does not expose artifact records, promotion state, or preview surfaces on this mission page.'
  }
])

const gitTrustFacts = computed<OperationalFact[]>(() => [
  {
    label: 'Branch',
    value: run.value?.branch || '—',
    detail: 'Git branch isolation for this mission.'
  },
  {
    label: 'Worktree',
    value: run.value?.worktreePath || '—',
    detail: 'Dedicated worktree path from current run payload.'
  },
  {
    label: 'Current focus',
    value: run.value?.currentStep || '—',
    tone: run.value?.currentStep ? 'ok' : 'default',
    detail: activeWorkerSummary.value
  },
  {
    label: 'Session binding',
    value: activeHandoff.value?.sessionId || 'Not recorded',
    tone: activeHandoff.value?.sessionId ? 'ok' : 'warn',
    detail: activeHandoff.value ? handoffSummary(activeHandoff.value) : 'No active handoff binding is visible right now.'
  }
])

const advanceRun = async () => {
  if (!run.value || !canAdvance.value || advancing.value) return

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

const resumeRun = async () => {
  if (!run.value || !canResume.value || resuming.value) return

  try {
    resuming.value = true
    stepActionError.value = ''
    run.value = await api.resumeRun(run.value.id)
  } catch (err: any) {
    stepActionError.value = err?.data?.error || err?.message || 'Could not resume run'
  } finally {
    resuming.value = false
  }
}

const dispatchStep = async () => {
  if (!run.value || !canDispatchStep.value || dispatching.value) return

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
  if (!run.value || !canCompleteStep.value || completing.value) return

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
  if (!run.value || !canCompleteStep.value || failing.value) return

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
    <section class="rf-card" data-testid="run-mission-header">
      <div class="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
        <div>
          <NuxtLink to="/" class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)] hover:text-cyan-200">Operations cockpit</NuxtLink>
          <div class="mt-2 flex flex-wrap items-center gap-3">
            <h1 class="text-3xl font-semibold">{{ workflow?.name || run?.workflowId || 'Run mission view' }}</h1>
            <span v-if="run" :class="statusTone(run.status)">{{ run.status }}</span>
          </div>
          <p class="mt-2 max-w-4xl text-sm text-[color:var(--rf-muted)]">{{ runHeadline }}</p>
          <div class="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-[color:var(--rf-muted)]">
            <span>Run {{ runId }}</span>
            <span v-if="workflow?.id">Workflow {{ workflow.id }}</span>
            <span v-if="run?.currentStep">Current focus {{ run.currentStep }}</span>
            <span v-if="latestEvent">Latest change {{ new Date(latestEvent.at).toLocaleString() }}</span>
          </div>
        </div>
        <div class="flex flex-wrap gap-3">
          <button class="rf-button" :disabled="pending" @click="refresh()">Refresh mission</button>
          <button class="rf-button" :disabled="!canAdvance || advancing || pending" @click="advanceRun()">
            {{ advancing ? 'Advancing…' : 'Advance run' }}
          </button>
          <button class="rf-button" :disabled="!canResume || resuming || pending" @click="resumeRun()">
            {{ resuming ? 'Resuming…' : 'Resume approval gate' }}
          </button>
        </div>
      </div>

      <div class="mt-5 grid gap-4 xl:grid-cols-[minmax(0,1.15fr)_minmax(0,1fr)_minmax(0,1fr)]">
        <div class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Attention required</div>
          <p class="mt-3 text-sm text-white/90">{{ attentionSummary }}</p>
        </div>
        <div class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Operational summary</div>
          <div class="mt-3 grid gap-3 sm:grid-cols-2">
            <div>
              <div class="text-xs text-[color:var(--rf-muted)]">Progress</div>
              <div class="mt-1 text-sm font-medium">{{ missionProgressLabel }}</div>
            </div>
            <div>
              <div class="text-xs text-[color:var(--rf-muted)]">Active worker</div>
              <div class="mt-1 text-sm font-medium">{{ activeWorkerSummary }}</div>
            </div>
            <div>
              <div class="text-xs text-[color:var(--rf-muted)]">Latest event</div>
              <div class="mt-1 text-sm font-medium">{{ latestEvent?.type || 'No events yet' }}</div>
            </div>
            <div>
              <div class="text-xs text-[color:var(--rf-muted)]">Git branch</div>
              <div class="mt-1 break-all font-mono text-xs text-white/90">{{ run?.branch || '—' }}</div>
            </div>
          </div>
        </div>
        <div class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Dispatch posture</div>
          <p class="mt-3 text-sm text-white/90">{{ dispatchReadinessSummary }}</p>
          <div class="mt-3 grid gap-3 sm:grid-cols-2">
            <div>
              <div class="text-xs text-[color:var(--rf-muted)]">Active handoff</div>
              <div class="mt-1 text-sm font-medium">{{ activeHandoffLabel }}</div>
            </div>
            <div>
              <div class="text-xs text-[color:var(--rf-muted)]">Last checkpoint</div>
              <div class="mt-1 text-sm font-medium">{{ recentCheckpoint?.stepId || 'No step record yet' }}</div>
            </div>
          </div>
        </div>
      </div>

      <p v-if="advanceError" class="mt-4 text-sm text-rose-200">{{ advanceError }}</p>
      <p v-if="stepActionError" class="mt-2 text-sm text-rose-200">{{ stepActionError }}</p>
      <p v-if="workflowError" class="mt-2 text-sm text-amber-200">Workflow context is partial: {{ workflowError }}</p>
    </section>

    <section v-if="pending" class="grid gap-6 xl:grid-cols-[320px_minmax(0,1fr)]">
      <div class="rf-card space-y-3">
        <div class="rf-skeleton h-5 w-2/3 rounded" />
        <div class="rf-skeleton h-4 w-full rounded" />
        <div class="rf-skeleton h-4 w-4/5 rounded" />
      </div>
      <div class="rf-card space-y-3">
        <div v-for="n in 6" :key="n" class="rf-skeleton h-12 rounded-xl" />
      </div>
    </section>

    <section v-else-if="loadError" class="rf-card border-rose-400/30 bg-rose-500/10 text-rose-100">
      Could not load run {{ runId }}. {{ loadError }}
    </section>

    <section v-else-if="!run" class="rf-card text-sm text-[color:var(--rf-muted)]">
      Run {{ runId }} was not found.
    </section>

    <section v-else class="grid gap-6 xl:grid-cols-[320px_minmax(0,1.3fr)_minmax(280px,360px)]">
      <aside class="space-y-6 xl:sticky xl:top-24 xl:self-start">
        <section class="rf-card">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Mission context</div>
          <h2 class="mt-2 text-lg font-semibold">Execution truth</h2>
          <dl class="mt-4 space-y-4 text-sm">
            <div>
              <dt class="text-[color:var(--rf-muted)]">Workflow</dt>
              <dd class="mt-1">{{ workflow?.name || run.workflowId }}</dd>
            </div>
            <div>
              <dt class="text-[color:var(--rf-muted)]">Current step</dt>
              <dd class="mt-1">{{ run.currentStep || '—' }}</dd>
            </div>
            <div>
              <dt class="text-[color:var(--rf-muted)]">Worker focus</dt>
              <dd class="mt-1 text-xs text-white/90">{{ activeWorkerSummary }}</dd>
            </div>
            <div>
              <dt class="text-[color:var(--rf-muted)]">Branch</dt>
              <dd class="mt-1 break-all font-mono text-xs text-white/90">{{ run.branch }}</dd>
            </div>
            <div>
              <dt class="text-[color:var(--rf-muted)]">Worktree</dt>
              <dd class="mt-1 break-all font-mono text-xs text-white/90">{{ run.worktreePath }}</dd>
            </div>
            <div>
              <dt class="text-[color:var(--rf-muted)]">Created</dt>
              <dd class="mt-1 text-xs text-white/90">{{ run.createdAt ? new Date(run.createdAt).toLocaleString() : 'Not recorded' }}</dd>
            </div>
          </dl>
        </section>

        <section class="rf-card">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Intervention controls</div>
          <div class="mt-4 space-y-3">
            <button class="rf-button w-full" :disabled="!canDispatchStep || dispatching || completing || failing" @click="dispatchStep()">
              {{ dispatching ? 'Dispatching…' : 'Dispatch active step' }}
            </button>
            <button class="rf-button w-full" :disabled="!canCompleteStep || dispatching || completing || failing" @click="completeStep()">
              {{ completing ? 'Completing…' : 'Complete active step' }}
            </button>
            <button class="rf-button w-full" :disabled="!canCompleteStep || completing || failing" @click="failStep()">
              {{ failing ? 'Failing…' : 'Fail active step' }}
            </button>
          </div>
          <p class="mt-3 text-xs text-[color:var(--rf-muted)]">
            Controls remain explicit and limited to current API truth. No hidden automation is implied here.
          </p>
        </section>

        <AttentionContextCard
          title="Dispatch and worker truth"
          :summary="dispatchReadinessSummary"
          :facts="dispatchReadinessFacts"
          :bullets="dispatchReadinessBullets"
          :links="workflow?.id ? [{ label: 'Open workflow package', to: `/workflows/${workflow.id}` }] : []"
        />

        <AttentionContextCard
          title="Governance handoff for this mission"
          :summary="governanceSummary"
          :facts="governanceFacts"
          :bullets="approvalContextBullets"
          :links="[
            { label: 'Open governance queue', to: '/approvals' },
            ...(workflow?.id ? [{ label: 'Open workflow package', to: `/workflows/${workflow.id}` }] : [])
          ]"
        />
      </aside>

      <div class="space-y-6 min-w-0">
        <section class="rf-card min-w-0">
          <div class="border-b border-[color:var(--rf-border)] pb-3">
            <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Artifacts and evidence</div>
            <h2 class="mt-2 text-xl font-semibold">Trust strip for this mission</h2>
            <p class="mt-2 text-sm text-[color:var(--rf-muted)]">{{ evidenceTruthSummary }}</p>
          </div>

          <div class="mt-4 grid gap-4 xl:grid-cols-3">
            <section class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
              <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Governance evidence</div>
              <div class="mt-4">
                <OperationalFactGrid :facts="evidenceBundleFacts" :columns="3" />
              </div>
            </section>

            <section class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
              <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Artifact visibility</div>
              <div class="mt-4">
                <OperationalFactGrid :facts="artifactVisibilityFacts" :columns="2" />
              </div>
            </section>

            <section class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
              <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Git and dispatch trust</div>
              <div class="mt-4">
                <OperationalFactGrid :facts="gitTrustFacts" :columns="2" />
              </div>
            </section>
          </div>
        </section>

        <section class="rf-card min-w-0">
          <div class="border-b border-[color:var(--rf-border)] pb-3">
            <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Operational progression</div>
            <h2 class="mt-2 text-xl font-semibold">Mission sequence</h2>
            <p class="mt-2 text-sm text-[color:var(--rf-muted)]">Workflow steps shown as the operational path, not a generic diagram.</p>
          </div>

          <div v-if="workflowSteps.length === 0" class="mt-4 rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4 text-sm text-[color:var(--rf-muted)]">
            Workflow step sequence is not available from current API state.
          </div>

          <div v-else class="mt-5 grid gap-3">
            <div v-for="(step, index) in workflowSteps" :key="step.id" data-testid="run-step-card" :data-step-id="step.id" class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
              <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                <div>
                  <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Step {{ index + 1 }}</div>
                  <h3 class="mt-1 text-base font-semibold">{{ step.id }}</h3>
                  <p class="mt-2 text-sm text-[color:var(--rf-muted)]">{{ missionStepDetail(step, missionStepState(step, index)) }}</p>
                </div>
                <div class="flex flex-wrap items-center gap-2">
                  <span class="rf-badge">{{ step.kind }}</span>
                  <span :class="stepStateTone(missionStepState(step, index))">{{ missionStepState(step, index) }}</span>
                </div>
              </div>
              <dl class="mt-3 grid gap-2 text-xs text-[color:var(--rf-muted)] sm:grid-cols-2" v-if="step.agent || step.approverPolicy || latestStepRecords[step.id]">
                <div v-if="step.agent">
                  <dt>Agent</dt>
                  <dd class="mt-1 text-white/90">{{ step.agent }}</dd>
                </div>
                <div v-if="step.approverPolicy">
                  <dt>Approver policy</dt>
                  <dd class="mt-1 text-white/90">{{ step.approverPolicy }}</dd>
                </div>
                <div v-if="latestStepRecords[step.id]?.workerId">
                  <dt>Latest worker</dt>
                  <dd class="mt-1 text-white/90">{{ latestStepRecords[step.id]?.workerId }}</dd>
                </div>
                <div v-if="latestStepRecords[step.id]?.startedAt">
                  <dt>Latest start</dt>
                  <dd class="mt-1 text-white/90">{{ new Date(latestStepRecords[step.id]!.startedAt).toLocaleString() }}</dd>
                </div>
              </dl>
            </div>
          </div>
        </section>

        <section class="rf-card min-w-0">
          <div class="border-b border-[color:var(--rf-border)] pb-3">
            <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Recent changes</div>
            <h2 class="mt-2 text-xl font-semibold">Operational timeline</h2>
          </div>

          <div v-if="recentTimeline.length === 0" class="mt-4 rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4 text-sm text-[color:var(--rf-muted)]">
            No timeline events recorded yet.
          </div>

          <div v-else class="mt-5 space-y-4">
            <div v-for="event in recentTimeline" :key="`${event.at}-${event.type}`" class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
              <div class="flex flex-col gap-2 md:flex-row md:items-start md:justify-between">
                <div>
                  <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">{{ humanizeEvent(event) }}</div>
                  <strong class="mt-1 block">{{ event.type }}</strong>
                </div>
                <span class="text-xs text-[color:var(--rf-muted)]">{{ new Date(event.at).toLocaleString() }}</span>
              </div>
              <p class="mt-3 text-sm text-[color:var(--rf-muted)]">{{ event.detail }}</p>
            </div>
          </div>
        </section>
      </div>

      <aside class="space-y-6">
        <section v-if="linkedApproval" class="rf-card" data-testid="linked-approval-record">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Linked approval record</div>
          <h2 class="mt-2 text-lg font-semibold">Governance object in context</h2>
          <div class="mt-4 space-y-3 text-sm text-[color:var(--rf-muted)]">
            <div>
              <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">State</div>
              <div class="mt-1 flex flex-wrap items-center gap-2">
                <span :class="statusTone(linkedApproval.status)">{{ linkedApproval.status }}</span>
                <span class="rf-badge">{{ linkedApproval.stepId }}</span>
              </div>
            </div>
            <div>
              <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Decision record</div>
              <p class="mt-1">{{ approvalDecisionLabel }}</p>
              <p v-if="linkedApproval.rationale" class="mt-2 whitespace-pre-line text-white/85">{{ linkedApproval.rationale }}</p>
            </div>
            <div>
              <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Evidence manifest</div>
              <p class="mt-1 break-all text-xs text-white/90">{{ linkedApproval.evidenceManifest || 'Not recorded yet' }}</p>
            </div>
          </div>
        </section>

        <section class="rf-card">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Agent activity</div>
          <h2 class="mt-2 text-lg font-semibold">Dispatch handoffs</h2>
          <div v-if="recentHandoff" class="mt-4 rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-3 text-sm text-[color:var(--rf-muted)]">
            <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Most recent movement</div>
            <p class="mt-2 text-white/90">{{ recentHandoff.stepId }} · {{ recentHandoff.status }}</p>
            <p class="mt-1 text-xs">{{ handoffSummary(recentHandoff) }}</p>
            <p class="mt-1 text-xs">Updated {{ new Date(recentHandoff.updatedAt || recentHandoff.createdAt).toLocaleString() }}</p>
          </div>
          <p class="mt-2 text-sm text-[color:var(--rf-muted)]" v-if="handoffs.length === 0">
            No persisted handoff records yet.
          </p>
          <ul v-else class="mt-4 space-y-3 text-sm text-[color:var(--rf-muted)]">
            <li v-for="handoff in handoffs.slice().reverse()" :key="`${handoff.stepId}-${handoff.createdAt}`" class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-3">
              <div class="flex items-center justify-between gap-3">
                <strong class="text-white/90">{{ handoff.stepId }}</strong>
                <span :class="statusTone(handoff.status)">{{ handoff.status }}</span>
              </div>
              <p class="mt-2 text-xs">{{ handoffSummary(handoff) }}</p>
              <p class="mt-1 text-xs">Worker {{ handoff.workerId }}</p>
              <p class="mt-1 text-xs">Created {{ new Date(handoff.createdAt).toLocaleString() }}</p>
              <p class="mt-1 text-xs">Updated {{ new Date(handoff.updatedAt || handoff.createdAt).toLocaleString() }}</p>
              <p class="mt-1 text-xs" v-if="handoff.dispatchAttemptAt">Dispatch attempt {{ new Date(handoff.dispatchAttemptAt).toLocaleString() }}</p>
            </li>
          </ul>
        </section>

        <section class="rf-card">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Step records</div>
          <h2 class="mt-2 text-lg font-semibold">Execution checkpoints</h2>
          <div v-if="recentCheckpoint" class="mt-4 rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-3 text-sm text-[color:var(--rf-muted)]">
            <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Latest checkpoint</div>
            <p class="mt-2 text-white/90">{{ recentCheckpoint.stepId }} · {{ recentCheckpoint.status }}</p>
            <p class="mt-1 text-xs">Worker {{ recentCheckpoint.workerId }}</p>
          </div>
          <p class="mt-2 text-sm text-[color:var(--rf-muted)]" v-if="steps.length === 0">
            No persisted step records yet.
          </p>
          <ul v-else class="mt-4 space-y-3 text-sm text-[color:var(--rf-muted)]">
            <li v-for="step in steps.slice().reverse()" :key="`${step.stepId}-${step.startedAt}`" class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-3">
              <div class="flex items-center justify-between gap-3">
                <strong class="text-white/90">{{ step.stepId }}</strong>
                <span :class="statusTone(step.status)">{{ step.status }}</span>
              </div>
              <p class="mt-2 text-xs">Worker {{ step.workerId }}</p>
              <p class="mt-1 text-xs">Started {{ new Date(step.startedAt).toLocaleString() }}</p>
              <p class="mt-1 text-xs" v-if="step.finishedAt">Finished {{ new Date(step.finishedAt).toLocaleString() }}</p>
              <p class="mt-1 text-xs" v-if="step.kind || step.agent">{{ step.kind || 'unknown kind' }}<span v-if="step.agent"> · {{ step.agent }}</span></p>
            </li>
          </ul>
        </section>

        <section class="rf-card">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Git trust context</div>
          <h2 class="mt-2 text-lg font-semibold">Repository isolation</h2>
          <dl class="mt-4 space-y-3 text-sm text-[color:var(--rf-muted)]">
            <div>
              <dt>Branch</dt>
              <dd class="mt-1 break-all font-mono text-xs text-white/90">{{ run.branch }}</dd>
            </div>
            <div>
              <dt>Worktree</dt>
              <dd class="mt-1 break-all font-mono text-xs text-white/90">{{ run.worktreePath }}</dd>
            </div>
          </dl>
          <p class="mt-3 text-xs text-[color:var(--rf-muted)]">
            Commit, diff, and artifact evidence still need broader API exposure. This panel stays explicit so the operator can trust what is and is not yet available.
          </p>
        </section>
      </aside>
    </section>
  </div>
</template>
