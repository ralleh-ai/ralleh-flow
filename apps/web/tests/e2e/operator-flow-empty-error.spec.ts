import { expect, test } from '@playwright/test'
import type { FlowRun } from '../../../types/flow'
import { installFlowApiMocks } from './fixtures/flowApi'

const quietRuns: FlowRun[] = [
  {
    id: 'run-quiet-1',
    workflowId: 'wf-launch-ops',
    status: 'completed',
    currentStep: 'ship_execution',
    branch: 'flow/run-quiet-1',
    worktreePath: '/tmp/ralleh-flow/run-quiet-1',
    createdAt: '2026-06-14T04:00:00.000Z',
    timeline: [
      { at: '2026-06-14T04:00:00.000Z', type: 'run_created', detail: 'Run staged.' },
      { at: '2026-06-14T04:30:00.000Z', type: 'run_completed', detail: 'Mission completed cleanly.' }
    ],
    steps: [],
    handoffs: []
  }
]

test('dashboard command view shows no-active-runs empty state', async ({ page }) => {
  await installFlowApiMocks(page, { runs: quietRuns, approvals: [] })

  await page.goto('/')

  await expect(page.getByTestId('dashboard-command-view')).toBeVisible()
  await expect(page.getByTestId('dashboard-live-empty')).toBeVisible()
  await expect(page.getByTestId('dashboard-live-run')).toHaveCount(0)
})

test('approvals queue shows explicit empty state when no approvals exist', async ({ page }) => {
  await installFlowApiMocks(page, { approvals: [] })

  await page.goto('/approvals')

  await expect(page.getByTestId('approval-empty-state')).toBeVisible()
  await expect(page.getByTestId('approval-card-approval-1')).toHaveCount(0)
})

test('run detail shows not-found state for missing run id', async ({ page }) => {
  await installFlowApiMocks(page)

  await page.goto('/runs/run-missing')

  await expect(page.getByTestId('run-not-found')).toBeVisible()
  await expect(page.getByText('Run run-missing was not found.')).toBeVisible()
})

test('dashboard renders API failure banner when cockpit data cannot load', async ({ page }) => {
  await installFlowApiMocks(page, { fail: { runs: true } })

  await page.goto('/')

  await expect(page.getByTestId('dashboard-error-banner')).toBeVisible()
  await expect(page.getByText('Could not load cockpit data.')).toBeVisible()
})

test('approvals renders API failure banner when queue fetch fails', async ({ page }) => {
  await installFlowApiMocks(page, { fail: { approvals: true } })

  await page.goto('/approvals')

  await expect(page.getByTestId('approval-error-banner')).toBeVisible()
  await expect(page.getByText('Could not load approvals.')).toBeVisible()
})
