<script setup lang="ts">
import type { FlowApprovalRecord, FlowRun, FlowWorkflow } from '~/types/flow'
import type { OperationalFact } from '~/types/ui'
import OperationalFactGrid from '~/components/operations/OperationalFactGrid.vue'

const decidedBy = 'operator'
const api = useFlowApi()

type ApprovalsPagePayload = {
  approvals: FlowApprovalRecord[]
  runs: FlowRun[]
  workflows: FlowWorkflow[]
}

const { data, pending, error, refresh } = await useAsyncData<ApprovalsPagePayload>('flow-approvals', async () => {
  const [approvals, runs, workflows] = await Promise.all([
    api.getApprovals(),
    api.getRuns(),
    api.getWorkflows()
  ])

  return { approvals, runs, workflows }
}, {
  default: () => ({ approvals: [], runs: [], workflows: [] })
})

const items = computed(() => data.value?.approvals ?? [])
const runs = computed(() => data.value?.runs ?? [])
const workflows = computed(() => data.value?.workflows ?? [])
const pendingItems = computed(() => items.value.filter((approval) => approval.status === 'pending'))
const resumableItems = computed(() => items.value.filter((approval) => approval.status === 'changes_requested'))
const rejectedItems = computed(() => items.value.filter((approval) => approval.status === 'rejected'))
const approvedItems = computed(() => items.value.filter((approval) => approval.status === 'approved'))
const busyApprovalId = ref<string | null>(null)
const actionErrors = reactive<Record<string, string>>({})
const rationaleDrafts = reactive<Record<string, string>>({})

const clearActionError = (approvalId: string) => {
  if (actionErrors[approvalId]) delete actionErrors[approvalId]
}

const formatDecisionFailure = (decision: 'approve' | 'reject' | 'request-changes' | 'resume') => {
  if (decision === 'approve') {
    return 'Approval did not go through. Refresh mission context and retry. The run remains blocked until approval succeeds.'
  }
  if (decision === 'reject') {
    return 'Rejection did not save. Keep your operator rationale, refresh run context, and retry when the stop decision is still correct.'
  }
  if (decision === 'request-changes') {
    return 'Request changes did not save. Confirm the rationale is explicit and retry so recovery expectations stay auditable.'
  }
  return 'Could not resume this approval gate. Re-verify rework evidence and run state, then retry recovery.'
}

const statusRank = (status: string) => {
  if (status === 'pending') return 0
  if (status === 'changes_requested') return 1
  if (status === 'rejected') return 2
  if (status === 'approved') return 3
  return 4
}

const orderedItems = computed(() => {
  return [...items.value].sort((a, b) => {
    const rankDiff = statusRank(a.status) - statusRank(b.status)
    if (rankDiff !== 0) return rankDiff
    return new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime()
  })
})

const runById = computed(() => new Map(runs.value.map((run) => [run.id, run] as const)))
const workflowById = computed(() => new Map(workflows.value.map((workflow) => [workflow.id, workflow] as const)))

const linkedRun = (approval: FlowApprovalRecord) => runById.value.get(approval.runId) ?? null
const linkedWorkflow = (approval: FlowApprovalRecord) => {
  const run = linkedRun(approval)
  return run ? workflowById.value.get(run.workflowId) ?? null : null
}

const approvalAttentionSummary = computed(() => {
  if (pendingItems.value.length > 0) {
    return 'Governance is actively shaping live runs. Resolve pending gates before launching more work that depends on the same operational surface.'
  }
  if (resumableItems.value.length > 0) {
    return 'Rework has already been requested. Resume only when the operator believes the run is ready to face the same gate again.'
  }
  if (rejectedItems.value.length > 0) {
    return 'Prior governance has already stopped some runs. Review those outcomes before repeating the same mission pattern.'
  }
  return 'No urgent governance pressure from current API truth.'
})

const approvalAttentionFacts = computed<OperationalFact[]>(() => [
  { label: 'Pending gates', value: String(pendingItems.value.length), tone: pendingItems.value.length ? 'warn' : 'ok' },
  { label: 'Rework waiting', value: String(resumableItems.value.length), tone: resumableItems.value.length ? 'warn' : 'ok' },
  { label: 'Rejected', value: String(rejectedItems.value.length), tone: rejectedItems.value.length ? 'danger' : 'default' },
  { label: 'Approved', value: String(approvedItems.value.length), tone: approvedItems.value.length ? 'ok' : 'default' }
])

const decisionRequiredNow = computed(() => pendingItems.value.length)
const evidenceGapCount = computed(() => items.value.filter((approval) => !approval.evidenceManifest).length)
const liveGovernedRuns = computed(() => {
  return items.value.filter((approval) => {
    const run = linkedRun(approval)
    return run && ['running', 'pending', 'waiting_for_approval', 'changes_requested'].includes(run.status)
  }).length
})

const decideApproval = async (approval: FlowApprovalRecord, decision: 'approve' | 'reject' | 'request-changes' | 'resume') => {
  const rationale = (rationaleDrafts[approval.id] ?? '').trim()
  clearActionError(approval.id)

  if ((decision === 'reject' || decision === 'request-changes') && !rationale) {
    actionErrors[approval.id] = 'Operator rationale is required before rejecting or requesting changes.'
    return
  }

  busyApprovalId.value = approval.id
  try {
    if (decision === 'approve') {
      await api.approveApproval(approval.id, { decidedBy, rationale: rationale || undefined })
    } else if (decision === 'reject') {
      await api.rejectApproval(approval.id, { decidedBy, rationale })
    } else if (decision === 'request-changes') {
      await api.requestApprovalChanges(approval.id, { decidedBy, rationale })
    } else {
      await api.resumeRun(approval.runId)
    }
    rationaleDrafts[approval.id] = ''
    clearActionError(approval.id)
    await refresh()
  } catch (error: any) {
    actionErrors[approval.id] = error?.data?.error ?? error?.message ?? formatDecisionFailure(decision)
  } finally {
    busyApprovalId.value = null
  }
}

const badgeTone = (status: string) => {
  if (status === 'pending') return 'rf-badge rf-badge--warn'
  if (status === 'approved') return 'rf-badge rf-badge--ok'
  if (status === 'rejected') return 'rf-badge rf-badge--danger'
  if (status === 'changes_requested') return 'rf-badge rf-badge--warn'
  return 'rf-badge'
}

const statusLabel = (status: string) => status.replaceAll('_', ' ')

const manifestName = (approval: FlowApprovalRecord) => {
  if (!approval.evidenceManifest) return 'Not recorded yet'
  const parts = approval.evidenceManifest.split('/')
  return parts[parts.length - 1] || approval.evidenceManifest
}

const decisionContext = (approval: FlowApprovalRecord) => {
  const parts: string[] = []
  if (approval.decidedBy) parts.push(`Decision by ${approval.decidedBy}`)
  if (approval.decidedAt) parts.push(new Date(approval.decidedAt).toLocaleString())
  return parts.join(' · ')
}

const approvalNextMove = (approval: FlowApprovalRecord) => {
  if (approval.status === 'pending') return 'Open the linked run mission view, inspect current step, Git branch, and evidence, then decide deliberately.'
  if (approval.status === 'changes_requested') return 'Review the linked run for rework evidence, then resume only when the gate should reopen.'
  if (approval.status === 'rejected') return 'Use the linked run mission view to understand where the mission stopped and whether a new run is safer than resuming.'
  if (approval.status === 'approved') return 'Governance has cleared this gate. The linked run shows what happened next.'
  return 'Use the linked run mission view for operational context.'
}

const recommendationText = (approval: FlowApprovalRecord) => {
  const run = linkedRun(approval)

  if (approval.status === 'pending') {
    if (!approval.evidenceManifest) return 'Recommendation: do not approve from memory. Inspect the mission and confirm why evidence is missing before deciding.'
    if (run?.status === 'waiting_for_approval') return 'Recommendation: this is a clean trust event. Review the mission context and decide whether the run should cross the gate now.'
    return 'Recommendation: verify the linked run still matches the requested gate before approving or rejecting.'
  }

  if (approval.status === 'changes_requested') return 'Recommendation: resume only when the linked run shows real rework evidence, not just elapsed time.'
  if (approval.status === 'rejected') return 'Recommendation: treat this as a prior trust failure and inspect whether the package or evidence path needs redesign.'
  if (approval.status === 'approved') return 'Recommendation: use this row as audit context, then inspect downstream mission movement instead of re-deciding the same gate.'
  return 'Recommendation: inspect the linked run for current truth.'
}

const consequenceText = (approval: FlowApprovalRecord) => {
  if (approval.status === 'pending') return 'Consequence: this decision changes live mission state, so speed matters less than correctness.'
  if (approval.status === 'changes_requested') return 'Consequence: resuming reopens the same gate and puts the mission back into operator trust flow.'
  if (approval.status === 'rejected') return 'Consequence: this mission path is stopped unless a new run or deliberate recovery path is created.'
  if (approval.status === 'approved') return 'Consequence: the gate is already cleared and downstream state should be evaluated on the mission view.'
  return 'Consequence depends on current mission state.'
}

const trustGapText = (approval: FlowApprovalRecord) => {
  const run = linkedRun(approval)
  if (!run) return 'Missing linked run context from current API payload.'
  if (!linkedWorkflow(approval)) return 'Workflow package metadata is missing for this run from the current page payload.'
  if (!approval.evidenceManifest) return 'No evidence manifest path is recorded yet, so approval depends more heavily on mission context and operator judgment.'
  return 'Evidence manifest is recorded, but diff/artifact preview still needs deeper backend exposure.'
}

const approvalFacts = (approval: FlowApprovalRecord): OperationalFact[] => {
  const run = linkedRun(approval)
  const workflow = linkedWorkflow(approval)

  return [
    { label: 'Package', value: workflow?.name || workflow?.id || 'Unknown package' },
    { label: 'Run state', value: run?.status || 'Run missing', tone: run?.status === 'failed' ? 'danger' : ['waiting_for_approval', 'changes_requested', 'pending'].includes(run?.status || '') ? 'warn' : run?.status === 'running' ? 'ok' : 'default' },
    { label: 'Current step', value: run?.currentStep || approval.stepId || '—' },
    { label: 'Evidence', value: approval.evidenceManifest ? 'Manifest recorded' : 'Missing manifest', tone: approval.evidenceManifest ? 'ok' : 'warn' }
  ]
}
</script>

<template>
  <div class="space-y-6">
    <section class="rf-card">
      <div class="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Governance queue</div>
          <h1 class="mt-2 text-3xl font-semibold">Operational approvals and interventions</h1>
          <p class="mt-3 max-w-3xl text-sm text-[color:var(--rf-muted)]">
            Review live governance gates with enough context to make a deliberate decision. Approvals are trust events, not anonymous button rows.
          </p>
        </div>
        <button class="rf-button" :disabled="pending" @click="refresh()">Refresh approvals</button>
      </div>
    </section>

    <section class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
      <div class="rf-card">
        <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Decision required now</div>
        <div class="mt-3 text-3xl font-semibold">{{ decisionRequiredNow }}</div>
        <p class="mt-2 text-sm text-[color:var(--rf-muted)]">Pending trust events that can change live mission state.</p>
      </div>
      <div class="rf-card">
        <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Live governed runs</div>
        <div class="mt-3 text-3xl font-semibold">{{ liveGovernedRuns }}</div>
        <p class="mt-2 text-sm text-[color:var(--rf-muted)]">Runs currently under or near governance pressure.</p>
      </div>
      <div class="rf-card">
        <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Evidence gaps</div>
        <div class="mt-3 text-3xl font-semibold">{{ evidenceGapCount }}</div>
        <p class="mt-2 text-sm text-[color:var(--rf-muted)]">Approvals missing a recorded evidence manifest path.</p>
      </div>
      <div class="rf-card">
        <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Current scope</div>
        <p class="mt-3 text-sm text-[color:var(--rf-muted)]">
          Phase 4 truth today: approve / reject / request-changes is live end-to-end, and paused runs can now be resumed back into the approval gate after rework.
        </p>
      </div>
    </section>

    <AttentionContextCard
      title="Where governance pressure is coming from"
      :summary="approvalAttentionSummary"
      :facts="approvalAttentionFacts"
      :bullets="[
        'Every approval row should lead cleanly to its run mission view; decisions without mission context are theater.',
        'Use operator notes to make the human decision legible when someone reviews the mission later.',
        'Resume is not forgiveness—it is a deliberate choice to reopen the same gate after rework.'
      ]"
      :links="[
        { label: 'Open cockpit', to: '/' },
        { label: 'Open workflow packages', to: '/workflows' }
      ]"
    />

    <section class="rf-card min-w-0">
      <div class="flex items-center justify-between gap-4 border-b border-[color:var(--rf-border)] pb-3">
        <div>
          <h2 class="text-lg font-semibold">Approval requests</h2>
          <p class="text-sm text-[color:var(--rf-muted)]">Operator queue with decision context, evidence references, Git trust clues, and deliberate intervention controls.</p>
        </div>
      </div>

      <div v-if="pending" class="mt-4 space-y-3">
        <div v-for="n in 4" :key="n" class="rf-skeleton h-28 rounded-2xl" />
      </div>

      <div v-else-if="error" data-testid="approval-error-banner" class="mt-4 rounded-2xl border border-rose-400/30 bg-rose-500/10 p-4 text-sm text-rose-100">
        Could not load approvals. {{ error.message }}
      </div>

      <div v-else-if="orderedItems.length === 0" data-testid="approval-empty-state" class="mt-4 rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-6 text-sm text-[color:var(--rf-muted)]">
        No approval requests have been recorded yet.
      </div>

      <div v-else class="mt-4 space-y-4">
        <article
          v-for="approval in orderedItems"
          :key="approval.id"
          :data-testid="`approval-card-${approval.id}`"
          :data-approval-id="approval.id"
          class="rounded-[1.6rem] border border-[color:var(--rf-border)] bg-black/10 p-5"
        >
          <div class="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <h3 class="text-lg font-semibold">{{ linkedWorkflow(approval)?.name || approval.stepId }}</h3>
                <span :class="badgeTone(approval.status)" :data-testid="`approval-status-${approval.id}`">{{ statusLabel(approval.status) }}</span>
                <span class="rf-badge">{{ approval.kind }}</span>
                <span v-if="approval.approverPolicy" class="rf-badge">{{ approval.approverPolicy }}</span>
              </div>

              <div class="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-[color:var(--rf-muted)]">
                <span>Run {{ approval.runId }}</span>
                <span>Step {{ approval.stepId }}</span>
                <span>Requested by {{ approval.requestedBy || '—' }}</span>
                <span>{{ new Date(approval.createdAt).toLocaleString() }}</span>
              </div>

              <p class="mt-4 text-sm text-white/90">{{ approvalNextMove(approval) }}</p>

              <div class="mt-4">
                <OperationalFactGrid :facts="approvalFacts(approval)" :columns="4" />
              </div>

              <div class="mt-4 grid gap-3 md:grid-cols-2">
                <div class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
                  <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Decision context</div>
                  <p class="mt-2 text-sm text-[color:var(--rf-muted)]">{{ decisionContext(approval) || 'No decision recorded yet.' }}</p>
                  <p v-if="approval.rationale" class="mt-3 whitespace-pre-line text-sm text-white/85">{{ approval.rationale }}</p>
                </div>
                <div class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
                  <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Evidence manifest</div>
                  <p class="mt-2 text-sm text-white/90">{{ manifestName(approval) }}</p>
                  <p v-if="approval.evidenceManifest" class="mt-2 break-all font-mono text-xs text-[color:var(--rf-muted)]">{{ approval.evidenceManifest }}</p>
                  <p v-else class="mt-2 text-xs text-amber-200">No manifest path recorded yet.</p>
                </div>
              </div>

              <div class="mt-4 grid gap-3 md:grid-cols-3">
                <div class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
                  <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Recommendation</div>
                  <p class="mt-2 text-sm text-white/90">{{ recommendationText(approval) }}</p>
                </div>
                <div class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
                  <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Consequence</div>
                  <p class="mt-2 text-sm text-[color:var(--rf-muted)]">{{ consequenceText(approval) }}</p>
                </div>
                <div class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
                  <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Trust gap</div>
                  <p class="mt-2 text-sm text-[color:var(--rf-muted)]">{{ trustGapText(approval) }}</p>
                </div>
              </div>
            </div>

            <div class="xl:w-[22rem] xl:min-w-[22rem]">
              <div class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
                <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Linked mission view</div>
                <p class="mt-2 text-sm text-[color:var(--rf-muted)]">
                  Open the run mission view to inspect current step, timeline, worker state, and Git isolation before changing governance state.
                </p>
                <div class="mt-3 space-y-2 text-xs text-[color:var(--rf-muted)]">
                  <p>Run state: <span class="text-white/90">{{ linkedRun(approval)?.status || 'unknown' }}</span></p>
                  <p>Branch: <span class="break-all font-mono text-white/90">{{ linkedRun(approval)?.branch || 'not available' }}</span></p>
                  <p>Worktree: <span class="break-all font-mono text-white/90">{{ linkedRun(approval)?.worktreePath || 'not available' }}</span></p>
                </div>
                <NuxtLink :to="`/runs/${approval.runId}`" class="rf-button mt-4 w-full justify-center">Open run mission</NuxtLink>
              </div>

              <div v-if="approval.status === 'pending'" class="mt-4 rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
                <label class="mb-2 block text-left text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Operator note</label>
                <textarea
                  v-model="rationaleDrafts[approval.id]"
                  :data-testid="`approval-note-${approval.id}`"
                  :aria-invalid="Boolean(actionErrors[approval.id])"
                  rows="4"
                  class="w-full rounded-xl border border-[color:var(--rf-border)] bg-black/20 px-3 py-2 text-sm text-white outline-none transition focus:border-cyan-400/60"
                  placeholder="Capture why you are approving, rejecting, or requesting changes."
                  @input="clearActionError(approval.id)"
                />
                <p class="mt-2 text-xs text-[color:var(--rf-muted)]">Required for reject or request-changes. Strongly recommended for approve.</p>
                <p
                  v-if="actionErrors[approval.id]"
                  :data-testid="`approval-action-error-${approval.id}`"
                  class="mt-3 rounded-xl border border-rose-400/30 bg-rose-500/10 px-3 py-2 text-xs text-rose-100"
                  role="alert"
                  aria-live="polite"
                >
                  {{ actionErrors[approval.id] }}
                </p>
                <div class="mt-3 flex flex-wrap justify-end gap-2">
                  <button class="rf-button rf-button--ghost" :data-testid="`approval-reject-${approval.id}`" :disabled="busyApprovalId === approval.id" @click="decideApproval(approval, 'reject')">
                    Reject
                  </button>
                  <button class="rf-button rf-button--ghost" :data-testid="`approval-request-changes-${approval.id}`" :disabled="busyApprovalId === approval.id" @click="decideApproval(approval, 'request-changes')">
                    Request changes
                  </button>
                  <button class="rf-button" :data-testid="`approval-approve-${approval.id}`" :disabled="busyApprovalId === approval.id" @click="decideApproval(approval, 'approve')">
                    <span v-if="busyApprovalId === approval.id">Working…</span>
                    <span v-else>Approve</span>
                  </button>
                </div>
              </div>

              <div v-else-if="approval.status === 'changes_requested'" class="mt-4 rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
                <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Resume gate</div>
                <p class="mt-2 text-sm text-[color:var(--rf-muted)]">
                  Reopen this approval only after reviewing the linked mission and confirming the requested rework actually happened.
                </p>
                <div class="mt-3 rounded-xl border border-cyan-400/20 bg-cyan-500/10 px-3 py-2 text-xs text-cyan-100">
                  Recovery path: resume returns this run to <span class="font-semibold">waiting_for_approval</span> so the same gate is decided again with updated evidence.
                </div>
                <p
                  v-if="actionErrors[approval.id]"
                  :data-testid="`approval-action-error-${approval.id}`"
                  class="mt-3 rounded-xl border border-rose-400/30 bg-rose-500/10 px-3 py-2 text-xs text-rose-100"
                  role="alert"
                  aria-live="polite"
                >
                  {{ actionErrors[approval.id] }}
                </p>
                <button class="rf-button mt-4 w-full justify-center" :data-testid="`approval-resume-${approval.id}`" :disabled="busyApprovalId === approval.id" @click="decideApproval(approval, 'resume')">
                  <span v-if="busyApprovalId === approval.id">Working…</span>
                  <span v-else>Resume approval</span>
                </button>
              </div>

              <div v-else class="mt-4 rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4 text-sm text-[color:var(--rf-muted)]">
                Decision already recorded. Use the linked mission view to understand the downstream effect on the run timeline.
              </div>
            </div>
          </div>
        </article>
      </div>
    </section>
  </div>
</template>
