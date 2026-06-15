import type { Page } from '@playwright/test'
import type { FlowApprovalRecord, FlowRun, FlowWorkflow, FlowWorkflowDetail } from '../../../types/flow'

type FlowFixtures = {
  workflows: FlowWorkflow[]
  workflowDetails: Record<string, FlowWorkflowDetail>
  runs: FlowRun[]
  approvals: FlowApprovalRecord[]
}

type FlowApiMockOptions = {
  runs?: FlowRun[]
  approvals?: FlowApprovalRecord[]
  fail?: {
    workflows?: boolean
    runs?: boolean
    approvals?: boolean
    runDetail?: boolean
    createRun?: boolean
    validateWorkflow?: boolean
    dryRunWorkflow?: boolean
    advance?: boolean
    dispatchStep?: boolean
    completeStep?: boolean
    failStep?: boolean
    resume?: boolean
    approvalDecision?: 'approve' | 'reject' | 'request-changes' | 'any'
  }
  failOnce?: {
    advance?: boolean
    dispatchStep?: boolean
    completeStep?: boolean
    failStep?: boolean
  }
}

const workflowByID = (workflowId: string) => fixtures.workflowDetails[workflowId]

const fixtures: FlowFixtures = {
  workflows: [
    {
      id: 'wf-launch-ops',
      name: 'Launch operations mission',
      version: '1.0.0',
      description: 'Primary operator workflow for mission launch readiness.',
      path: 'workflows/launch-ops.yaml'
    }
  ],
  workflowDetails: {
    'wf-launch-ops': {
      id: 'wf-launch-ops',
      name: 'Launch operations mission',
      version: '1.0.0',
      description: 'Primary operator workflow for mission launch readiness.',
      path: 'workflows/launch-ops.yaml',
      variables: [
        { key: 'objective', type: 'string', required: true, description: 'Mission objective' }
      ],
      steps: [
        { id: 'intake_context', kind: 'agent_task', agent: 'research-specialist' },
        { id: 'governance_gate', kind: 'human_approval', approverPolicy: 'operator_on_call' },
        { id: 'ship_execution', kind: 'agent_task', agent: 'implementation-specialist' }
      ]
    }
  },
  runs: [
    {
      id: 'run-101',
      workflowId: 'wf-launch-ops',
      status: 'waiting_for_approval',
      currentStep: 'governance_gate',
      branch: 'flow/run-101',
      worktreePath: '/tmp/ralleh-flow/run-101',
      createdAt: '2026-06-14T05:00:00.000Z',
      timeline: [
        { at: '2026-06-14T05:00:00.000Z', type: 'run_created', detail: 'Run staged and worktree prepared.' },
        { at: '2026-06-14T05:08:00.000Z', type: 'step_completed', detail: 'Context intake completed.' },
        { at: '2026-06-14T05:10:00.000Z', type: 'approval_requested', detail: 'Governance gate waiting for operator decision.' }
      ],
      steps: [
        {
          runId: 'run-101',
          stepId: 'intake_context',
          status: 'completed',
          workerId: 'worker-research-1',
          startedAt: '2026-06-14T05:01:00.000Z',
          finishedAt: '2026-06-14T05:08:00.000Z',
          kind: 'agent_task',
          agent: 'research-specialist'
        },
        {
          runId: 'run-101',
          stepId: 'governance_gate',
          status: 'waiting_for_approval',
          workerId: 'operator',
          startedAt: '2026-06-14T05:10:00.000Z',
          kind: 'human_approval'
        }
      ],
      handoffs: [
        {
          runId: 'run-101',
          stepId: 'governance_gate',
          status: 'pending',
          kind: 'human_approval',
          workerId: 'operator',
          sessionId: 'session:run-101:governance_gate',
          createdAt: '2026-06-14T05:10:00.000Z',
          updatedAt: '2026-06-14T05:10:00.000Z'
        }
      ]
    },
    {
      id: 'run-202',
      workflowId: 'wf-launch-ops',
      status: 'running',
      currentStep: 'ship_execution',
      branch: 'flow/run-202',
      worktreePath: '/tmp/ralleh-flow/run-202',
      createdAt: '2026-06-14T04:50:00.000Z',
      timeline: [
        { at: '2026-06-14T04:50:00.000Z', type: 'run_created', detail: 'Run staged.' },
        { at: '2026-06-14T05:05:00.000Z', type: 'step_started', detail: 'Execution step active.' }
      ],
      steps: [],
      handoffs: []
    },
    {
      id: 'run-303',
      workflowId: 'wf-launch-ops',
      status: 'failed',
      currentStep: 'ship_execution',
      branch: 'flow/run-303',
      worktreePath: '/tmp/ralleh-flow/run-303',
      createdAt: '2026-06-14T04:20:00.000Z',
      timeline: [
        { at: '2026-06-14T04:20:00.000Z', type: 'run_created', detail: 'Run staged.' },
        { at: '2026-06-14T04:40:00.000Z', type: 'step_failed', detail: 'Execution failed in sandbox.' }
      ],
      steps: [],
      handoffs: []
    }
  ],
  approvals: [
    {
      id: 'approval-1',
      runId: 'run-101',
      stepId: 'governance_gate',
      kind: 'human_approval',
      status: 'pending',
      approverPolicy: 'operator_on_call',
      requestedBy: 'worker-research-1',
      evidenceManifest: '/artifacts/run-101/governance/evidence.json',
      createdAt: '2026-06-14T05:10:00.000Z'
    }
  ]
}

const clone = <T>(value: T): T => JSON.parse(JSON.stringify(value))

export const installFlowApiMocks = async (page: Page, options: FlowApiMockOptions = {}) => {
  const state = {
    runs: clone(options.runs ?? fixtures.runs),
    approvals: clone(options.approvals ?? fixtures.approvals)
  }

  const fail = {
    workflows: false,
    runs: false,
    approvals: false,
    runDetail: false,
    createRun: false,
    validateWorkflow: false,
    dryRunWorkflow: false,
    advance: false,
    dispatchStep: false,
    completeStep: false,
    failStep: false,
    resume: false,
    approvalDecision: undefined,
    ...options.fail
  }

  const failOnceRemaining = {
    advance: options.failOnce?.advance ? 1 : 0,
    dispatchStep: options.failOnce?.dispatchStep ? 1 : 0,
    completeStep: options.failOnce?.completeStep ? 1 : 0,
    failStep: options.failOnce?.failStep ? 1 : 0
  }

  const shouldFailOnce = (key: keyof typeof failOnceRemaining) => {
    if (failOnceRemaining[key] <= 0) return false
    failOnceRemaining[key] -= 1
    return true
  }

  const fixedAt = '2026-06-14T06:10:00.000Z'

  const findRun = (runId: string) => state.runs.find((item) => item.id === runId)

  const latestRunningStep = (run: FlowRun) => {
    const runningSteps = run.steps.filter((step) => step.status === 'running')
    return runningSteps[runningSteps.length - 1]
  }

  const ensureHandoff = (run: FlowRun, stepId: string) => {
    let handoff = run.handoffs.find((item) => item.stepId === stepId)
    if (!handoff) {
      handoff = {
        runId: run.id,
        stepId,
        status: 'claimed',
        kind: 'agent_task',
        workerId: `worker-${stepId}`,
        sessionId: `session:${run.id}:${stepId}`,
        createdAt: fixedAt,
        updatedAt: fixedAt
      }
      run.handoffs.push(handoff)
    }
    return handoff
  }

  await page.route('**/api/flow/v1/**', async (route) => {
    const req = route.request()
    const url = new URL(req.url())
    const path = url.pathname
    const method = req.method()

    const json = (body: unknown, status = 200) => route.fulfill({
      status,
      contentType: 'application/json',
      body: JSON.stringify(body)
    })

    if (method === 'GET' && path.endsWith('/workflows')) {
      if (fail.workflows) return json({ error: 'workflow service unavailable' }, 503)
      return json({ items: fixtures.workflows })
    }

    if (method === 'GET' && path.endsWith('/runs')) {
      if (fail.runs) return json({ error: 'run service unavailable' }, 500)
      return json({ items: state.runs })
    }

    if (method === 'GET' && path.endsWith('/approvals')) {
      if (fail.approvals) return json({ error: 'approval service unavailable' }, 500)
      return json({ items: state.approvals })
    }

    const workflowMatch = path.match(/\/api\/flow\/v1\/workflows\/([^/]+)$/)
    if (method === 'GET' && workflowMatch?.[1]) {
      const workflowId = workflowMatch[1]
      const workflow = workflowByID(workflowId)
      if (!workflow) return json({ error: 'workflow not found' }, 404)
      return json(workflow)
    }

    const validateMatch = path.match(/\/api\/flow\/v1\/workflows\/([^/]+)\/validate$/)
    if (method === 'POST' && validateMatch?.[1]) {
      if (fail.validateWorkflow) return json({ error: 'validation service unavailable' }, 503)
      const workflowId = validateMatch[1]
      const workflow = workflowByID(workflowId)
      if (!workflow) return json({ error: 'workflow not found' }, 404)
      return json({
        workflowId,
        valid: true,
        errors: [],
        warnings: []
      })
    }

    const dryRunMatch = path.match(/\/api\/flow\/v1\/workflows\/([^/]+)\/dry-run$/)
    if (method === 'POST' && dryRunMatch?.[1]) {
      if (fail.dryRunWorkflow) return json({ error: 'dry-run service unavailable' }, 503)
      const workflowId = dryRunMatch[1]
      const workflow = workflowByID(workflowId)
      if (!workflow) return json({ error: 'workflow not found' }, 404)

      const payloadText = req.postData() || '{}'
      const payload = JSON.parse(payloadText)
      const provided = payload?.variables ?? {}

      const missingVariables = workflow.variables
        .filter((variable) => variable.required && !String(provided[variable.key] || '').trim())
        .map((variable) => variable.key)

      return json({
        workflowId,
        ready: missingVariables.length === 0,
        missingVariables,
        warnings: [],
        steps: workflow.steps.map((step) => ({
          id: step.id,
          kind: step.kind,
          agent: step.agent,
          blocking: step.kind === 'human_approval' || !!step.approverPolicy
        }))
      })
    }

    if (method === 'POST' && path.endsWith('/api/flow/v1/runs')) {
      if (fail.createRun) return json({ error: 'create run unavailable right now' }, 503)
      const payloadText = req.postData() || '{}'
      const payload = JSON.parse(payloadText)
      const workflowId = String(payload.workflowId || '')
      const workflow = workflowByID(workflowId)
      if (!workflowId) return json({ error: 'workflowId is required' }, 400)
      if (!workflow) return json({ error: 'workflow not found' }, 404)

      const provided = payload?.variables ?? {}
      const missingVariables = workflow.variables
        .filter((variable) => variable.required && !String(provided[variable.key] || '').trim())
        .map((variable) => variable.key)

      if (missingVariables.length > 0) {
        return json({ error: `missing required variables: ${missingVariables.join(', ')}` }, 400)
      }

      const runId = `run-created-${state.runs.length + 1}`
      const run: FlowRun = {
        id: runId,
        workflowId,
        status: 'pending',
        currentStep: workflow.steps[0]?.id || '',
        branch: `flow/${runId}`,
        worktreePath: `/tmp/ralleh-flow/${runId}`,
        createdAt: '2026-06-14T06:00:00.000Z',
        timeline: [{ at: '2026-06-14T06:00:00.000Z', type: 'run_created', detail: 'Run created from package detail page.' }],
        steps: [],
        handoffs: []
      }

      state.runs.unshift(run)
      return json(run, 201)
    }

    const runMatch = path.match(/\/api\/flow\/v1\/runs\/([^/]+)$/)
    if (method === 'GET' && runMatch?.[1]) {
      if (fail.runDetail) return json({ error: 'run detail unavailable' }, 500)
      const runId = runMatch[1]
      const run = state.runs.find((item) => item.id === runId)
      if (!run) return json({ error: 'run not found' }, 404)
      return json(run)
    }

    const approvalDecisionMatch = path.match(/\/api\/flow\/v1\/approvals\/([^/]+)\/(approve|reject|request-changes)$/)
    if (method === 'POST' && approvalDecisionMatch) {
      const [, approvalId, decision] = approvalDecisionMatch
      const approval = state.approvals.find((item) => item.id === approvalId)
      if (!approval) return json({ error: 'approval not found' }, 404)

      if (fail.approvalDecision === 'any' || fail.approvalDecision === decision) {
        return json({ error: `could not ${decision} approval right now` }, 500)
      }

      const run = state.runs.find((item) => item.id === approval.runId)
      const payloadText = req.postData() || '{}'
      const payload = JSON.parse(payloadText)

      if ((decision === 'reject' || decision === 'request-changes') && !String(payload.rationale || '').trim()) {
        return json({ error: 'rationale is required for this decision' }, 422)
      }

      approval.decidedBy = payload.decidedBy || 'operator'
      approval.rationale = payload.rationale
      approval.decidedAt = '2026-06-14T05:20:00.000Z'

      if (decision === 'approve') {
        approval.status = 'approved'
        if (run) run.status = 'running'
      } else if (decision === 'reject') {
        approval.status = 'rejected'
        if (run) run.status = 'rejected'
      } else {
        approval.status = 'changes_requested'
        if (run) run.status = 'changes_requested'
      }

      return json(run ?? { id: approval.runId })
    }

    const resumeMatch = path.match(/\/api\/flow\/v1\/runs\/([^/]+)\/resume$/)
    if (method === 'POST' && resumeMatch?.[1]) {
      if (fail.resume) return json({ error: 'run recovery unavailable right now' }, 503)
      const runId = resumeMatch[1]
      const run = state.runs.find((item) => item.id === runId)
      if (!run) return json({ error: 'run not found' }, 404)
      run.status = 'waiting_for_approval'

      const latestApproval = state.approvals
        .filter((item) => item.runId === runId)
        .sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime())[0]
      if (latestApproval && latestApproval.status === 'changes_requested') {
        latestApproval.status = 'pending'
        latestApproval.decidedBy = undefined
        latestApproval.decidedAt = undefined
      }

      return json(run)
    }

    const advanceMatch = path.match(/\/api\/flow\/v1\/runs\/([^/]+)\/advance$/)
    if (method === 'POST' && advanceMatch?.[1]) {
      if (fail.advance || shouldFailOnce('advance')) return json({ error: 'run advance unavailable right now' }, 503)

      const run = findRun(advanceMatch[1])
      if (!run) return json({ error: 'run not found' }, 404)
      if (run.status !== 'pending') return json({ error: `run is ${run.status}; only pending runs can be advanced` }, 409)

      const workflow = workflowByID(run.workflowId)
      if (!workflow) return json({ error: 'workflow not found' }, 404)

      const stepId = run.currentStep || workflow.steps[0]?.id
      if (!stepId) return json({ error: 'workflow has no steps to advance' }, 422)

      run.status = 'running'
      run.currentStep = stepId

      if (!run.steps.find((step) => step.stepId === stepId && step.status === 'running')) {
        run.steps.push({
          runId: run.id,
          stepId,
          status: 'running',
          workerId: `worker-${stepId}`,
          startedAt: fixedAt,
          kind: workflow.steps.find((step) => step.id === stepId)?.kind,
          agent: workflow.steps.find((step) => step.id === stepId)?.agent
        })
      }

      const handoff = ensureHandoff(run, stepId)
      handoff.status = 'claimed'
      handoff.updatedAt = fixedAt

      run.timeline.push({ at: fixedAt, type: 'run_advanced', detail: `Run advanced into ${stepId}.` })
      return json(run)
    }

    const dispatchStepMatch = path.match(/\/api\/flow\/v1\/runs\/([^/]+)\/dispatch-step$/)
    if (method === 'POST' && dispatchStepMatch?.[1]) {
      if (fail.dispatchStep || shouldFailOnce('dispatchStep')) return json({ error: 'dispatch service unavailable right now' }, 503)

      const run = findRun(dispatchStepMatch[1])
      if (!run) return json({ error: 'run not found' }, 404)
      if (run.status !== 'running') return json({ error: 'run is not running' }, 409)

      const activeStep = latestRunningStep(run)
      if (!activeStep) return json({ error: 'no active step to dispatch' }, 409)

      const payloadText = req.postData() || '{}'
      const payload = JSON.parse(payloadText)

      const handoff = ensureHandoff(run, activeStep.stepId)
      handoff.status = 'dispatched'
      handoff.sessionId = payload.sessionId || handoff.sessionId
      handoff.dispatchAttemptAt = fixedAt
      handoff.updatedAt = fixedAt

      run.timeline.push({ at: fixedAt, type: 'step_dispatched', detail: `Step ${activeStep.stepId} dispatched.` })
      return json(run)
    }

    const completeStepMatch = path.match(/\/api\/flow\/v1\/runs\/([^/]+)\/complete-step$/)
    if (method === 'POST' && completeStepMatch?.[1]) {
      if (fail.completeStep || shouldFailOnce('completeStep')) return json({ error: 'step completion unavailable right now' }, 503)

      const run = findRun(completeStepMatch[1])
      if (!run) return json({ error: 'run not found' }, 404)

      const workflow = workflowByID(run.workflowId)
      if (!workflow) return json({ error: 'workflow not found' }, 404)

      const activeStep = latestRunningStep(run)
      if (!activeStep) return json({ error: 'no active step to complete' }, 409)

      activeStep.status = 'completed'
      activeStep.finishedAt = fixedAt

      const currentIndex = workflow.steps.findIndex((step) => step.id === activeStep.stepId)
      const nextStep = currentIndex >= 0 ? workflow.steps[currentIndex + 1] : undefined

      if (!nextStep) {
        run.status = 'completed'
      } else {
        run.currentStep = nextStep.id
        if (nextStep.kind === 'human_approval' || nextStep.approverPolicy) {
          run.status = 'waiting_for_approval'
          run.steps.push({
            runId: run.id,
            stepId: nextStep.id,
            status: 'waiting_for_approval',
            workerId: 'operator',
            startedAt: fixedAt,
            kind: nextStep.kind,
            agent: nextStep.agent
          })
          const approvalHandoff = ensureHandoff(run, nextStep.id)
          approvalHandoff.status = 'pending'
          approvalHandoff.kind = nextStep.kind
          approvalHandoff.workerId = 'operator'
          approvalHandoff.updatedAt = fixedAt
        } else {
          run.status = 'running'
          run.steps.push({
            runId: run.id,
            stepId: nextStep.id,
            status: 'running',
            workerId: `worker-${nextStep.id}`,
            startedAt: fixedAt,
            kind: nextStep.kind,
            agent: nextStep.agent
          })
          const nextHandoff = ensureHandoff(run, nextStep.id)
          nextHandoff.status = 'claimed'
          nextHandoff.kind = nextStep.kind
          nextHandoff.workerId = `worker-${nextStep.id}`
          nextHandoff.updatedAt = fixedAt
        }
      }

      run.timeline.push({ at: fixedAt, type: 'step_completed', detail: `Step ${activeStep.stepId} completed.` })
      return json(run)
    }

    const failStepMatch = path.match(/\/api\/flow\/v1\/runs\/([^/]+)\/fail-step$/)
    if (method === 'POST' && failStepMatch?.[1]) {
      if (fail.failStep || shouldFailOnce('failStep')) return json({ error: 'step failure endpoint unavailable right now' }, 503)

      const run = findRun(failStepMatch[1])
      if (!run) return json({ error: 'run not found' }, 404)

      const activeStep = latestRunningStep(run)
      if (!activeStep) return json({ error: 'no active step to fail' }, 409)

      activeStep.status = 'failed'
      activeStep.finishedAt = fixedAt
      run.status = 'failed'

      const handoff = ensureHandoff(run, activeStep.stepId)
      handoff.status = 'failed'
      handoff.updatedAt = fixedAt

      run.timeline.push({ at: fixedAt, type: 'step_failed', detail: `Step ${activeStep.stepId} failed.` })
      return json(run)
    }

    return json({ error: `Unhandled mocked endpoint: ${method} ${path}` }, 404)
  })
}
