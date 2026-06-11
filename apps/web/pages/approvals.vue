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
const busyApprovalId = ref<string | null>(null)
const actionError = ref<string>('')
const rationaleDrafts = reactive<Record<string, string>>({})

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

      <div v-else class="mt-4">
        <div v-if="actionError" class="mb-4 rounded-2xl border border-rose-400/30 bg-rose-500/10 p-4 text-sm text-rose-100">
          {{ actionError }}
        </div>

        <div class="overflow-x-auto">
        <table class="min-w-full text-sm">
          <thead class="text-left text-[color:var(--rf-muted)]">
            <tr>
              <th class="py-2 pr-4">Run</th>
              <th class="py-2 pr-4">Gate</th>
              <th class="py-2 pr-4">State</th>
              <th class="py-2 pr-4">Policy</th>
              <th class="py-2 pr-4">Requested by</th>
              <th class="py-2 pr-4">Requested at</th>
              <th class="py-2 pr-4">Evidence manifest</th>
              <th class="py-2 text-right">Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="approval in items" :key="approval.id" class="border-t border-[color:var(--rf-border)]/70 align-top">
              <td class="py-3 pr-4 font-medium">
                <NuxtLink :to="`/runs/${approval.runId}`" class="hover:text-cyan-300">{{ approval.runId }}</NuxtLink>
              </td>
              <td class="py-3 pr-4">
                <div class="font-medium">{{ approval.stepId }}</div>
                <div class="mt-1 text-xs text-[color:var(--rf-muted)]">{{ approval.kind }}</div>
              </td>
              <td class="py-3 pr-4">
                <span :class="badgeTone(approval.status)">{{ approval.status }}</span>
                <div v-if="decisionContext(approval)" class="mt-2 text-xs text-[color:var(--rf-muted)]">{{ decisionContext(approval) }}</div>
                <div v-if="approval.rationale" class="mt-2 max-w-xs whitespace-pre-line text-xs text-white/85">{{ approval.rationale }}</div>
              </td>
              <td class="py-3 pr-4">{{ approval.approverPolicy || '—' }}</td>
              <td class="py-3 pr-4">{{ approval.requestedBy || '—' }}</td>
              <td class="py-3 pr-4 text-xs text-[color:var(--rf-muted)]">{{ new Date(approval.createdAt).toLocaleString() }}</td>
              <td class="py-3 pr-4 text-xs text-[color:var(--rf-muted)]">
                <div class="font-medium text-white/90">{{ manifestName(approval) }}</div>
                <div v-if="approval.evidenceManifest" class="mt-1 break-all">{{ approval.evidenceManifest }}</div>
              </td>
              <td class="py-3 text-right">
                <div v-if="approval.status === 'pending'" class="min-w-[18rem]">
                  <label class="mb-2 block text-left text-xs uppercase tracking-[0.2em] text-[color:var(--rf-muted)]">Operator note</label>
                  <textarea
                    v-model="rationaleDrafts[approval.id]"
                    rows="3"
                    class="mb-3 w-full rounded-xl border border-[color:var(--rf-border)] bg-black/20 px-3 py-2 text-sm text-white outline-none transition focus:border-cyan-400/60"
                    placeholder="Capture why you are approving, rejecting, or requesting changes."
                  />
                  <div class="flex justify-end gap-2">
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
                <button v-else-if="approval.status === 'changes_requested'" class="rf-button" :disabled="busyApprovalId === approval.id" @click="decideApproval(approval, 'resume')">
                  <span v-if="busyApprovalId === approval.id">Working…</span>
                  <span v-else>Resume approval</span>
                </button>
                <span v-else class="text-xs text-[color:var(--rf-muted)]">Decision recorded</span>
              </td>
            </tr>
          </tbody>
        </table>
        </div>
      </div>
    </section>
  </div>
</template>
