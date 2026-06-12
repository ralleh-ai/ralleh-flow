<script setup lang="ts">
import type { FlowApprovalRecord } from '~/types/flow'

const decidedBy = 'operator'

const api = useFlowApi()

const { data: approvals, pending, error, refresh } = await useAsyncData('flow-approvals', async () => {
  return await api.getApprovals()
})

const items = computed(() => approvals.value ?? [])
const pendingItems = computed(() => items.value.filter((approval) => approval.status === 'pending'))
const resumableItems = computed(() => items.value.filter((approval) => approval.status === 'changes_requested'))
const rejectedItems = computed(() => items.value.filter((approval) => approval.status === 'rejected'))
const busyApprovalId = ref<string | null>(null)
const actionError = ref<string>('')
const rationaleDrafts = reactive<Record<string, string>>({})

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

const approvalAttentionFacts = computed(() => [
  { label: 'Pending gates', value: String(pendingItems.value.length), tone: pendingItems.value.length ? 'warn' : 'ok' },
  { label: 'Rework waiting', value: String(resumableItems.value.length), tone: resumableItems.value.length ? 'warn' : 'ok' },
  { label: 'Rejected', value: String(rejectedItems.value.length), tone: rejectedItems.value.length ? 'danger' : 'default' }
] as const)

const decideApproval = async (approval: FlowApprovalRecord, decision: 'approve' | 'reject' | 'request-changes' | 'resume') => {
  const rationale = (rationaleDrafts[approval.id] ?? '').trim()

  if ((decision === 'reject' || decision === 'request-changes') && !rationale) {
    actionError.value = 'Add an operator note before rejecting or requesting changes.'
    return
  }

  busyApprovalId.value = approval.id
  actionError.value = ''
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
    await refresh()
  } catch (error: any) {
    actionError.value = error?.data?.error ?? error?.message ?? `Could not ${decision} approval.`
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
  if (approval.status === 'pending') return 'Open the linked run mission view, check current step and evidence, then decide deliberately.'
  if (approval.status === 'changes_requested') return 'Review the linked run for rework evidence, then resume only when the gate should reopen.'
  if (approval.status === 'rejected') return 'Use the linked run mission view to understand where the mission stopped and whether a new run is safer than resuming.'
  if (approval.status === 'approved') return 'Governance has cleared this gate. The linked run shows what happened next.'
  return 'Use the linked run mission view for operational context.'
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
            Review live governance gates with enough context to make a deliberate decision. Approvals, rejections, change requests, and resumed gates should all read like operational interventions—not anonymous status flips.
          </p>
        </div>
        <button class="rf-button" :disabled="pending" @click="refresh()">Refresh approvals</button>
      </div>
    </section>

    <section class="grid gap-4 md:grid-cols-3">
      <div class="rf-card">
        <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Pending</div>
        <div class="mt-3 text-3xl font-semibold">{{ pendingItems.length }}</div>
        <p class="mt-2 text-sm text-[color:var(--rf-muted)]">Approvals currently blocking workflow progress.</p>
      </div>
      <div class="rf-card">
        <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Needs changes</div>
        <div class="mt-3 text-3xl font-semibold">{{ resumableItems.length }}</div>
        <p class="mt-2 text-sm text-[color:var(--rf-muted)]">Runs paused for rework that can be reopened to the approval gate.</p>
      </div>
      <div class="rf-card md:col-span-2">
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
          <p class="text-sm text-[color:var(--rf-muted)]">Operator queue with decision context, evidence manifest references, and deliberate intervention controls.</p>
        </div>
      </div>

      <div v-if="pending" class="mt-4 space-y-3">
        <div v-for="n in 4" :key="n" class="rf-skeleton h-14 rounded-xl" />
      </div>

      <div v-else-if="error" class="mt-4 rounded-2xl border border-rose-400/30 bg-rose-500/10 p-4 text-sm text-rose-100">
        Could not load approvals. {{ error.message }}
      </div>

      <div v-else-if="items.length === 0" class="mt-4 rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-6 text-sm text-[color:var(--rf-muted)]">
        No approval requests have been recorded yet.
      </div>

      <div v-else class="mt-4 space-y-4">
        <div v-if="actionError" class="rounded-2xl border border-rose-400/30 bg-rose-500/10 p-4 text-sm text-rose-100">
          {{ actionError }}
        </div>

        <article
          v-for="approval in items"
          :key="approval.id"
          class="rounded-[1.6rem] border border-[color:var(--rf-border)] bg-black/10 p-5"
        >
          <div class="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <h3 class="text-lg font-semibold">{{ approval.stepId }}</h3>
                <span :class="badgeTone(approval.status)">{{ approval.status }}</span>
                <span class="rf-badge">{{ approval.kind }}</span>
                <span v-if="approval.approverPolicy" class="rf-badge">{{ approval.approverPolicy }}</span>
              </div>

              <div class="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-[color:var(--rf-muted)]">
                <span>Run {{ approval.runId }}</span>
                <span>Requested by {{ approval.requestedBy || '—' }}</span>
                <span>{{ new Date(approval.createdAt).toLocaleString() }}</span>
              </div>

              <p class="mt-4 text-sm text-white/90">{{ approvalNextMove(approval) }}</p>

              <div class="mt-4 grid gap-3 md:grid-cols-2">
                <div class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
                  <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Decision context</div>
                  <p class="mt-2 text-sm text-[color:var(--rf-muted)]">{{ decisionContext(approval) || 'No decision recorded yet.' }}</p>
                  <p v-if="approval.rationale" class="mt-3 whitespace-pre-line text-sm text-white/85">{{ approval.rationale }}</p>
                </div>
                <div class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
                  <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Evidence manifest</div>
                  <p class="mt-2 text-sm text-white/90">{{ manifestName(approval) }}</p>
                  <p v-if="approval.evidenceManifest" class="mt-2 break-all text-xs text-[color:var(--rf-muted)]">{{ approval.evidenceManifest }}</p>
                  <p v-else class="mt-2 text-xs text-[color:var(--rf-muted)]">No manifest path recorded yet.</p>
                </div>
              </div>
            </div>

            <div class="xl:w-[22rem] xl:min-w-[22rem]">
              <div class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
                <div class="text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Linked mission view</div>
                <p class="mt-2 text-sm text-[color:var(--rf-muted)]">
                  Open the run mission view to inspect current step, timeline, worker state, and Git isolation before changing governance state.
                </p>
                <NuxtLink :to="`/runs/${approval.runId}`" class="rf-button mt-4 w-full justify-center">Open run mission</NuxtLink>
              </div>

              <div v-if="approval.status === 'pending'" class="mt-4 rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
                <label class="mb-2 block text-left text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Operator note</label>
                <textarea
                  v-model="rationaleDrafts[approval.id]"
                  rows="4"
                  class="w-full rounded-xl border border-[color:var(--rf-border)] bg-black/20 px-3 py-2 text-sm text-white outline-none transition focus:border-cyan-400/60"
                  placeholder="Capture why you are approving, rejecting, or requesting changes."
                />
                <div class="mt-3 flex flex-wrap justify-end gap-2">
                  <button class="rf-button rf-button--ghost" :disabled="busyApprovalId === approval.id" @click="decideApproval(approval, 'reject')">
                    Reject
                  </button>
                  <button class="rf-button rf-button--ghost" :disabled="busyApprovalId === approval.id" @click="decideApproval(approval, 'request-changes')">
                    Request changes
                  </button>
                  <button class="rf-button" :disabled="busyApprovalId === approval.id" @click="decideApproval(approval, 'approve')">
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
                <button class="rf-button mt-4 w-full justify-center" :disabled="busyApprovalId === approval.id" @click="decideApproval(approval, 'resume')">
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
