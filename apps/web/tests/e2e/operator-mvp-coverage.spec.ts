import { expect, test } from '@playwright/test'
import type { FlowRun } from '../../types/flow'
import { installFlowApiMocks } from './fixtures/flowApi'

test('workflow catalog and assets pages render key operator states', async ({ page }) => {
  await installFlowApiMocks(page)

  await page.goto('/workflows')

  await expect(page.getByRole('heading', { name: 'Operational packages' })).toBeVisible()
  const workflowCard = page.locator('article').filter({ has: page.getByRole('heading', { name: 'Launch operations mission' }) })
  await expect(workflowCard).toBeVisible()
  await expect(workflowCard).toContainText('Human approval is currently the deciding factor for this package.')
  await expect(workflowCard).toContainText('Review blocked approvals')

  await page.goto('/assets')

  await expect(page.getByRole('heading', { name: 'Input readiness and provenance pressure' })).toBeVisible()
  await expect(page.getByText('No dependency-sensitive package signatures were detected from current workflow variable contracts.')).toBeVisible()
  await expect(page.getByText('API synced')).toBeVisible()
})

test('workflow detail validates, dry-runs missing+ready paths, and surfaces create-run API errors', async ({ page }) => {
  await installFlowApiMocks(page, { fail: { createRun: true } })

  await page.goto('/workflows/wf-launch-ops')

  await page.getByRole('button', { name: 'Validate workflow' }).click()
  await expect(page.getByText('Workflow contract is valid.')).toBeVisible()

  await page.getByRole('button', { name: 'Dry run' }).click()
  await expect(page.getByText('Dry-run found launch blockers or pending requirements.')).toBeVisible()
  await expect(page.getByText('Missing required variables: objective')).toBeVisible()

  await page.getByLabel('objective').fill('Ship operator-ready mission controls')
  await page.getByRole('button', { name: 'Dry run' }).click()
  await expect(page.getByText('Dry-run says this workflow is ready for launch.')).toBeVisible()
  await expect(page.getByText('All required variables currently provided.')).toBeVisible()

  await page.getByRole('button', { name: 'Create run' }).click()
  await expect(page.getByText('create run unavailable right now')).toBeVisible()
  await expect(page.getByText('Run created:')).toHaveCount(0)
})

test('run mission controls support advance, dispatch with failure recovery, and complete step', async ({ page }) => {
  const pendingRun: FlowRun = {
    id: 'run-pending-1',
    workflowId: 'wf-launch-ops',
    status: 'pending',
    currentStep: 'intake_context',
    branch: 'flow/run-pending-1',
    worktreePath: '/tmp/ralleh-flow/run-pending-1',
    createdAt: '2026-06-14T06:00:00.000Z',
    timeline: [{ at: '2026-06-14T06:00:00.000Z', type: 'run_created', detail: 'Run staged.' }],
    steps: [],
    handoffs: []
  }

  await installFlowApiMocks(page, {
    runs: [pendingRun],
    approvals: [],
    failOnce: { dispatchStep: true }
  })

  await page.goto('/runs/run-pending-1')

  await page.getByRole('button', { name: 'Advance run' }).click()
  await expect(page.getByTestId('run-mission-header')).toContainText('running')

  await page.getByRole('button', { name: 'Dispatch active step' }).click()
  await expect(page.getByTestId('run-step-action-error')).toContainText('dispatch service unavailable right now')

  await page.getByRole('button', { name: 'Dispatch active step' }).click()
  await expect(page.getByTestId('run-step-action-error')).toHaveCount(0)

  await page.getByRole('button', { name: 'Complete active step' }).click()
  await expect(page.getByTestId('run-mission-header')).toContainText('waiting_for_approval')
  await expect(page.getByTestId('run-recovery-state')).toContainText('Recovery checkpoint: this run is back at the approval gate')
})

test('run mission controls can fail an active step', async ({ page }) => {
  const runningRun: FlowRun = {
    id: 'run-running-fail-1',
    workflowId: 'wf-launch-ops',
    status: 'running',
    currentStep: 'ship_execution',
    branch: 'flow/run-running-fail-1',
    worktreePath: '/tmp/ralleh-flow/run-running-fail-1',
    createdAt: '2026-06-14T06:00:00.000Z',
    timeline: [
      { at: '2026-06-14T06:00:00.000Z', type: 'run_created', detail: 'Run staged.' },
      { at: '2026-06-14T06:05:00.000Z', type: 'step_started', detail: 'Ship execution started.' }
    ],
    steps: [
      {
        runId: 'run-running-fail-1',
        stepId: 'ship_execution',
        status: 'running',
        workerId: 'worker-ship',
        startedAt: '2026-06-14T06:05:00.000Z',
        kind: 'agent_task',
        agent: 'implementation-specialist'
      }
    ],
    handoffs: [
      {
        runId: 'run-running-fail-1',
        stepId: 'ship_execution',
        status: 'dispatched',
        kind: 'agent_task',
        workerId: 'worker-ship',
        sessionId: 'session:run-running-fail-1:ship_execution',
        dispatchAttemptAt: '2026-06-14T06:06:00.000Z',
        createdAt: '2026-06-14T06:05:00.000Z',
        updatedAt: '2026-06-14T06:06:00.000Z'
      }
    ]
  }

  await installFlowApiMocks(page, {
    runs: [runningRun],
    approvals: []
  })

  await page.goto('/runs/run-running-fail-1')

  await page.getByRole('button', { name: 'Fail active step' }).click()
  await expect(page.getByTestId('run-mission-header')).toContainText('failed')
  await expect(page.getByText('Attention required: the active operation failed and should be reviewed before another step is triggered.')).toBeVisible()
})
