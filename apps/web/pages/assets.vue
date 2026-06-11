<script setup lang="ts">
const plannedCapabilities = [
  {
    title: 'Provenance',
    detail: 'Every asset should carry source provenance, storage location, and pinned identity so a run can explain what it depended on.'
  },
  {
    title: 'Version history',
    detail: 'Asset versions should be immutable for the lifetime of a workflow run and inspectable later during audits or recovery.'
  },
  {
    title: 'Workflow references',
    detail: 'Operators should be able to see which workflows and runs reference an asset before approving or resuming execution.'
  },
  {
    title: 'Readiness',
    detail: 'The system should make missing, invalid, or waiting-for-assets conditions explicit before a run pretends it can execute.'
  }
]

const operatorQuestions = [
  'What is this resource, really?',
  'Where did it come from?',
  'Which version is pinned to a run?',
  'What workflows depend on it?',
  'Is it ready, missing, or invalid?'
]

const plannedApi = [
  'POST /v1/assets',
  'GET /v1/assets/{id}'
]

const plannedStates = [
  'waiting_for_assets',
  'preparing',
  'running',
  'checkpointed'
]

const truthBoundaries = [
  'No asset ingest API is shipped yet.',
  'No persisted asset registry data is exposed to this page yet.',
  'No version history or lineage graph is available from the current backend.',
  'Run-specific asset pinning remains planned, not implemented on this screen.'
]
</script>

<template>
  <div class="space-y-6">
    <section class="rf-card">
      <div class="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
        <div>
          <p class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Resource intelligence</p>
          <h1 class="mt-3 text-2xl font-semibold md:text-3xl">Asset registry</h1>
          <p class="mt-3 max-w-4xl text-sm text-[color:var(--rf-muted)]">
            Assets are not files or folders. They are operational resources with provenance, pinned versions, workflow references, and readiness implications. This page stays honest about that target while the backend asset APIs are still unshipped.
          </p>
        </div>
        <div class="rounded-2xl border border-amber-400/30 bg-amber-500/10 px-4 py-3 text-sm text-amber-100">
          MVP truth: asset ingest and registry APIs are planned, not live.
        </div>
      </div>
    </section>

    <section class="grid gap-6 xl:grid-cols-[minmax(0,1.2fr)_340px]">
      <section class="rf-card min-w-0">
        <div class="border-b border-[color:var(--rf-border)] pb-3">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Design stance</div>
          <h2 class="mt-2 text-xl font-semibold">What this surface should help an operator decide</h2>
        </div>

        <div class="mt-5 grid gap-3 md:grid-cols-2">
          <div
            v-for="question in operatorQuestions"
            :key="question"
            class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4 text-sm text-white/90"
          >
            {{ question }}
          </div>
        </div>

        <div class="mt-6 rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-5">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Why this matters</div>
          <p class="mt-3 text-sm text-[color:var(--rf-muted)]">
            Asset handling should support governance, restart safety, and auditability. The operator should never have to guess whether a run consumed the right prompt, fixture, document, or transformed artifact.
          </p>
        </div>
      </section>

      <aside class="space-y-6">
        <section class="rf-card">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Planned API surface</div>
          <h2 class="mt-2 text-lg font-semibold">Backend truth we are designing toward</h2>
          <ul class="mt-4 space-y-3 text-sm text-[color:var(--rf-muted)]">
            <li v-for="endpoint in plannedApi" :key="endpoint" class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-3 font-mono text-xs text-white/90">
              {{ endpoint }}
            </li>
          </ul>
        </section>

        <section class="rf-card">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Operational state impact</div>
          <h2 class="mt-2 text-lg font-semibold">Run states influenced by assets</h2>
          <div class="mt-4 flex flex-wrap gap-2">
            <span v-for="state in plannedStates" :key="state" class="rf-badge rf-badge--warn">{{ state }}</span>
          </div>
          <p class="mt-3 text-xs text-[color:var(--rf-muted)]">
            These are meaningful only when backed by real asset validation and pinning rules.
          </p>
        </section>
      </aside>
    </section>

    <section class="rf-card">
      <div class="border-b border-[color:var(--rf-border)] pb-3">
        <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Capability model</div>
        <h2 class="mt-2 text-xl font-semibold">What the real registry needs to express</h2>
        <p class="mt-2 text-sm text-[color:var(--rf-muted)]">
          Until the asset APIs exist, the best interim UI is a precise contract with the future system — not fake rows, fake upload buttons, or decorative empty states.
        </p>
      </div>

      <div class="mt-5 grid gap-3 lg:grid-cols-2">
        <div
          v-for="capability in plannedCapabilities"
          :key="capability.title"
          class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4"
        >
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">{{ capability.title }}</div>
          <p class="mt-3 text-sm text-[color:var(--rf-muted)]">{{ capability.detail }}</p>
        </div>
      </div>
    </section>

    <section class="grid gap-6 xl:grid-cols-[minmax(0,1.2fr)_340px]">
      <section class="rf-card">
        <div class="border-b border-[color:var(--rf-border)] pb-3">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Interim operator guidance</div>
          <h2 class="mt-2 text-xl font-semibold">How to read the current product honestly</h2>
        </div>
        <ul class="mt-5 space-y-4 text-sm text-[color:var(--rf-muted)]">
          <li class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
            Workflow dry-run and validation already establish the idea that missing inputs should be surfaced before execution.
          </li>
          <li class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
            Approval and run views are already moving toward stronger governance semantics; the asset registry must eventually match that level of explicitness.
          </li>
          <li class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4">
            The right future slice after backend asset APIs land is not a file browser. It is a provenance and dependency console with run/workflow references.
          </li>
        </ul>
      </section>

      <aside class="rf-card">
        <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Truth boundaries</div>
        <h2 class="mt-2 text-lg font-semibold">What is not shipped yet</h2>
        <ul class="mt-4 space-y-3 text-sm text-[color:var(--rf-muted)]">
          <li v-for="item in truthBoundaries" :key="item" class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-3">
            {{ item }}
          </li>
        </ul>
      </aside>
    </section>
  </div>
</template>
