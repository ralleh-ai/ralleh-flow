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
    resume?: boolean
    approvalDecision?: 'approve' | 'reject' | 'request-changes' | 'any'
  }
}

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
    resume: false,
    approvalDecision: undefined,
    ...options.fail
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
      const workflow = fixtures.workflowDetails[workflowId]
      if (!workflow) return json({ error: 'workflow not found' }, 404)
      return json(workflow)
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

    return json({ error: `Unhandled mocked endpoint: ${method} ${path}` }, 404)
  })
}
