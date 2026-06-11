<script setup lang="ts">
const route = useRoute()
const api = useFlowApi()

const workflowId = computed(() => String(route.params.workflowId || ''))

const loadError = ref('')
const submitPending = ref(false)
const submitError = ref('')
const createdRunId = ref('')
const validationPending = ref(false)
const validationError = ref('')
const validationResult = ref<Awaited<ReturnType<typeof api.validateWorkflow>> | null>(null)
const dryRunPending = ref(false)
const dryRunError = ref('')
const dryRunResult = ref<Awaited<ReturnType<typeof api.dryRunWorkflow>> | null>(null)
const variableValues = reactive<Record<string, string>>({})

const { data: workflow, pending, refresh } = useAsyncData(
  () => `flow-workflow-${workflowId.value}`,
  async () => {
    if (!workflowId.value) {
      loadError.value = 'Missing workflow id'
      return null
    }

    try {
      loadError.value = ''
      return await api.getWorkflow(workflowId.value)
    } catch (err: any) {
      loadError.value = err?.data?.error || err?.message || 'Could not load workflow'
      return null
    }
  },
  {
    watch: [workflowId],
    default: () => null
  }
)

const variables = computed(() => workflow.value?.variables ?? [])
const steps = computed(() => workflow.value?.steps ?? [])

watch(
  variables,
  (nextVariables) => {
    const allowedKeys = new Set(nextVariables.map(variable => variable.key))

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
    .filter(variable => variable.required && !String(variableValues[variable.key] || '').trim())
    .map(variable => variable.key)
})

const canCreateRun = computed(() => !!workflow.value && !submitPending.value && missingRequiredVariables.value.length === 0)

const validateWorkflow = async () => {
  if (!workflow.value || validationPending.value) {
    return
  }

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
  if (!workflow.value || dryRunPending.value) {
    return
  }

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
  if (!workflow.value || submitPending.value) {
    return
  }

  if (missingRequiredVariables.value.length > 0) {
    submitError.value = `Provide required variables: ${missingRequiredVariables.value.join(', ')}`
    return
  }

  submitPending.value = true
  submitError.value = ''
  createdRunId.value = ''

  try {
    const variablesPayload = Object.fromEntries(
      Object.entries(variableValues)
        .map(([key, value]) => [key, String(value || '').trim()])
        .filter(([, value]) => value)
    )

    const run = await api.createRun(workflow.value.id, variablesPayload)
    createdRunId.value = run.id
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
      <div class="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <NuxtLink to="/workflows" class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)] hover:text-cyan-200">Workflow catalog</NuxtLink>
          <h1 class="mt-2 text-3xl font-semibold">{{ workflow?.name || workflowId }}</h1>
          <p class="mt-2 text-sm text-[color:var(--rf-muted)]">Workflow definition, operator inputs, and launch controls.</p>
        </div>
        <button class="rf-button" :disabled="pending" @click="refresh()">Refresh workflow</button>
      </div>
    </section>

    <section v-if="pending" class="grid gap-6 xl:grid-cols-[1.3fr_1fr]">
      <div class="rf-card space-y-3">
        <div class="rf-skeleton h-5 w-1/2 rounded" />
        <div class="rf-skeleton h-4 w-full rounded" />
        <div class="rf-skeleton h-4 w-4/5 rounded" />
        <div class="rf-skeleton h-24 w-full rounded" />
      </div>
      <div class="rf-card space-y-3">
        <div class="rf-skeleton h-10 w-full rounded-xl" />
        <div class="rf-skeleton h-10 w-full rounded-xl" />
        <div class="rf-skeleton h-28 w-full rounded-xl" />
      </div>
    </section>

    <section v-else-if="loadError" class="rf-card border-rose-400/30 bg-rose-500/10 text-rose-100">
      Could not load workflow {{ workflowId }}. {{ loadError }}
    </section>

    <section v-else-if="!workflow" class="rf-card text-sm text-[color:var(--rf-muted)]">
      Workflow {{ workflowId }} was not found.
    </section>

    <section v-else class="grid gap-6 xl:grid-cols-[1.3fr_1fr]">
      <section class="space-y-6">
        <article class="rf-card">
          <div class="flex items-start justify-between gap-4">
            <div>
              <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Metadata</div>
              <h2 class="mt-2 text-xl font-semibold">{{ workflow.id }}</h2>
            </div>
            <span class="rf-badge">v{{ workflow.version || 'n/a' }}</span>
          </div>

          <p class="mt-3 text-sm text-[color:var(--rf-muted)]">{{ workflow.description || 'No description provided.' }}</p>

          <dl class="mt-5 space-y-2 text-sm">
            <div class="flex items-center justify-between gap-4">
              <dt class="text-[color:var(--rf-muted)]">Definition path</dt>
              <dd class="max-w-[70%] truncate font-mono text-xs">{{ workflow.path }}</dd>
            </div>
          </dl>
        </article>

        <article class="rf-card">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Variables</div>
          <h2 class="mt-2 text-xl font-semibold">Input contract</h2>

          <p v-if="variables.length === 0" class="mt-3 text-sm text-[color:var(--rf-muted)]">
            This workflow currently defines no variables.
          </p>

          <ul v-else class="mt-4 space-y-3">
            <li
              v-for="variable in variables"
              :key="variable.key"
              class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4"
            >
              <div class="flex items-center justify-between gap-4">
                <code class="font-semibold">{{ variable.key }}</code>
                <span class="rf-badge">{{ variable.type || 'unknown' }}</span>
              </div>
              <p class="mt-2 text-sm text-[color:var(--rf-muted)]">{{ variable.description || 'No description provided.' }}</p>
              <div class="mt-2 text-xs text-[color:var(--rf-muted)]">{{ variable.required ? 'Required' : 'Optional' }}</div>
            </li>
          </ul>
        </article>

        <article class="rf-card">
          <div class="text-xs uppercase tracking-[0.3em] text-[color:var(--rf-muted)]">Steps</div>
          <h2 class="mt-2 text-xl font-semibold">Execution order</h2>

          <p v-if="steps.length === 0" class="mt-3 text-sm text-[color:var(--rf-muted)]">
            No executable steps are currently defined.
          </p>

          <ol v-else class="mt-4 space-y-3">
            <li
              v-for="(step, index) in steps"
              :key="step.id || `${index}`"
              class="rounded-2xl border border-[color:var(--rf-border)] bg-black/10 p-4"
            >
              <div class="flex items-center justify-between gap-4">
                <strong>{{ step.id || `step-${index + 1}` }}</strong>
                <span class="rf-badge">{{ step.kind || 'unknown' }}</span>
              </div>
              <dl class="mt-2 space-y-1 text-sm text-[color:var(--rf-muted)]">
                <div v-if="step.agent" class="flex gap-2">
                  <dt>Agent:</dt>
                  <dd>{{ step.agent }}</dd>
                </div>
                <div v-if="step.approverPolicy" class="flex gap-2">
                  <dt>Approver policy:</dt>
                  <dd>{{ step.approverPolicy }}</dd>
                </div>
              </dl>
            </li>
          </ol>
        </article>
      </section>

      <aside class="space-y-6">
        <section class="rf-card">
          <h2 class="text-lg font-semibold">Create run</h2>
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

          <div v-if="createdRunId" class="mt-3 rounded-xl border border-emerald-300/30 bg-emerald-500/10 p-3 text-sm text-emerald-100">
            Run created: <NuxtLink :to="`/runs/${createdRunId}`" class="underline">{{ createdRunId }}</NuxtLink>
          </div>
        </section>

        <section class="rf-card">
          <h2 class="text-lg font-semibold">Workflow checks</h2>
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
      </aside>
    </section>
  </div>
</template>
