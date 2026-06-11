<script setup lang="ts">
import type { FlowApprovalRecord } from '~/types/flow'

const api = useFlowApi()

const { data: approvals, pending, error, refresh } = await useAsyncData('flow-approvals', async () => {
  return await api.getApprovals()
})

const items = computed(() => approvals.value ?? [])
const pendingItems = computed(() => items.value.filter((approval) => approval.status === 'pending'))

const badgeTone = (status: string) => {
  if (status === 'pending') return 'rf-badge rf-badge--warn'
  if (status === 'approved') return 'rf-badge rf-badge--ok'
  if (status === 'rejected') return 'rf-badge rf-badge--danger'
  return 'rf-badge'
}

const manifestName = (approval: FlowApprovalRecord) => {
  if (!approval.evidenceManifest) return 'Not recorded yet'
  const parts = approval.evidenceManifest.split('/')
  return parts[parts.length - 1] || approval.evidenceManifest
}
</script>

<template>
  <div class="space-y-6">
    <section class="rf-card">
      <div class="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Approval inbox</div>
          <h1 class="mt-2 text-3xl font-semibold">Pending governance gates</h1>
          <p class="mt-3 text-sm text-[color:var(--rf-muted)]">
            This inbox now reflects persisted approval requests from the API. Decision actions are the next phase.
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
      <div class="rf-card md:col-span-2">
        <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Current scope</div>
        <p class="mt-3 text-sm text-[color:var(--rf-muted)]">
          Phase 4 truth today: request records + inbox visibility. Approve / reject / request-changes mutations are still pending backend work.
        </p>
      </div>
    </section>

    <section class="rf-card min-w-0">
      <div class="flex items-center justify-between gap-4 border-b border-[color:var(--rf-border)] pb-3">
        <div>
          <h2 class="text-lg font-semibold">Approval requests</h2>
          <p class="text-sm text-[color:var(--rf-muted)]">Run, gate, policy, requestor, and manifest currently on record.</p>
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

      <div v-else class="mt-4 overflow-x-auto">
        <table class="min-w-full text-sm">
          <thead class="text-left text-[color:var(--rf-muted)]">
            <tr>
              <th class="py-2 pr-4">Run</th>
              <th class="py-2 pr-4">Gate</th>
              <th class="py-2 pr-4">State</th>
              <th class="py-2 pr-4">Policy</th>
              <th class="py-2 pr-4">Requested by</th>
              <th class="py-2 pr-4">Requested at</th>
              <th class="py-2">Evidence manifest</th>
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
              <td class="py-3 pr-4"><span :class="badgeTone(approval.status)">{{ approval.status }}</span></td>
              <td class="py-3 pr-4">{{ approval.approverPolicy || '—' }}</td>
              <td class="py-3 pr-4">{{ approval.requestedBy || '—' }}</td>
              <td class="py-3 pr-4 text-xs text-[color:var(--rf-muted)]">{{ new Date(approval.createdAt).toLocaleString() }}</td>
              <td class="py-3 text-xs text-[color:var(--rf-muted)]">
                <div class="font-medium text-white/90">{{ manifestName(approval) }}</div>
                <div v-if="approval.evidenceManifest" class="mt-1 break-all">{{ approval.evidenceManifest }}</div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>
