<script setup lang="ts">
import { Activity, Radio, ShieldCheck, Workflow } from 'lucide-vue-next'

const api = useFlowApi()
const { data: runs } = useAsyncData('flow-topbar-runs', async () => {
  try {
    return await api.getRuns()
  } catch {
    return []
  }
}, {
  default: () => []
})

const telemetry = computed(() => {
  const allRuns = runs.value ?? []
  const activeRuns = allRuns.filter((run) => run.status === 'running').length
  const approvals = allRuns.filter((run) => run.status.includes('approval')).length
  const queuedRuns = allRuns.filter((run) => run.status === 'queued').length

  return {
    activeRuns,
    queuedRuns,
    approvals,
    events: allRuns.length ? 'API synced' : 'No run data'
  }
})
</script>

<template>
  <header class="sticky top-0 z-20 border-b border-[color:var(--rf-border)] bg-[color:var(--rf-surface)]/95 px-4 py-3 backdrop-blur lg:px-6">
    <div class="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
      <div>
        <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">/flow</div>
        <h2 class="mt-1 text-lg font-semibold">Workflow operations</h2>
      </div>
      <div class="grid grid-cols-2 gap-2 md:grid-cols-4">
        <div class="rf-pill"><Workflow class="h-4 w-4" /> {{ telemetry.activeRuns }} active</div>
        <div class="rf-pill"><Activity class="h-4 w-4" /> {{ telemetry.queuedRuns }} queued</div>
        <div class="rf-pill"><ShieldCheck class="h-4 w-4" /> {{ telemetry.approvals }} approvals</div>
        <div class="rf-pill"><Radio class="h-4 w-4" /> {{ telemetry.events }}</div>
      </div>
    </div>
  </header>
</template>
