import { expect, test } from '@playwright/test'
import { installFlowApiMocks } from './fixtures/flowApi'

test.beforeEach(async ({ page }) => {
  await installFlowApiMocks(page)
})

test('dashboard command view renders actionable operational state', async ({ page }) => {
  await page.goto('/')

  await expect(page.getByTestId('dashboard-command-view')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Command view for live workflow operations' })).toBeVisible()
  await expect(page.getByTestId('dashboard-triage-run').filter({ hasText: 'run-101' })).toBeVisible()
  await expect(page.getByTestId('dashboard-triage-run').filter({ hasText: 'Approval gate is blocking progress.' })).toBeVisible()
  await expect(page.getByTestId('dashboard-live-run').filter({ hasText: 'run-202' })).toBeVisible()
})

test('approvals queue exposes trust context and supports decision state transitions', async ({ page }) => {
  await page.goto('/approvals')

  const approvalCard = page.getByTestId('approval-card-approval-1')
  await expect(approvalCard).toBeVisible()
  await expect(approvalCard.getByText('operator_on_call')).toBeVisible()
  await expect(approvalCard.getByText('Open run mission')).toBeVisible()

  await approvalCard.getByTestId('approval-note-approval-1').fill('Need rework on the risk summary before reopening the gate.')
  await approvalCard.getByTestId('approval-request-changes-approval-1').click()

  await expect(approvalCard.getByTestId('approval-status-approval-1')).toContainText('changes requested')
  await expect(approvalCard.getByTestId('approval-resume-approval-1')).toBeVisible()
  await expect(approvalCard).toContainText('Recovery path: resume returns this run to waiting_for_approval')

  await approvalCard.getByTestId('approval-resume-approval-1').click()
  await expect(approvalCard.getByTestId('approval-status-approval-1')).toContainText('pending')
})

test('run detail renders mission progression and linked approval context', async ({ page }) => {
  await page.goto('/runs/run-101')

  await expect(page.getByTestId('run-mission-header')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Mission sequence' })).toBeVisible()
  await expect(page.getByTestId('run-recovery-state')).toContainText('Recovery checkpoint: this run is back at the approval gate')

  const stepCards = page.getByTestId('run-step-card')
  await expect(stepCards).toHaveCount(3)
  await expect(stepCards.nth(0)).toContainText('completed')
  await expect(stepCards.nth(1)).toContainText('awaiting_approval')
  await expect(stepCards.nth(2)).toContainText('queued')

  const linkedApproval = page.getByTestId('linked-approval-record')
  await expect(linkedApproval).toBeVisible()
  await expect(linkedApproval).toContainText('Governance object in context')
  await expect(linkedApproval).toContainText('pending')
})
