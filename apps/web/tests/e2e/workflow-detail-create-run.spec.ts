import { expect, test } from '@playwright/test'
import { installFlowApiMocks } from './fixtures/flowApi'

test.beforeEach(async ({ page }) => {
  await installFlowApiMocks(page)
})

test('workflow detail shows live package context and creates a run from operator launch control', async ({ page }) => {
  await page.goto('/workflows/wf-launch-ops')

  await expect(page.getByRole('heading', { name: 'Launch operations mission' })).toBeVisible()
  await expect(page.getByText('Workflow package detail, readiness checks, launch controls')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Create run' })).toBeVisible()

  await expect(page.getByText('Required before launch: objective')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Create run' })).toBeDisabled()

  await page.getByLabel('objective').fill('Ship API-backed workflow operator detail UI')
  await expect(page.getByRole('button', { name: 'Create run' })).toBeEnabled()
  await page.getByRole('button', { name: 'Create run' }).click()

  await expect(page.getByText('Run created:')).toBeVisible()
  await expect(page.getByRole('link', { name: 'run-created-4', exact: true })).toBeVisible()
  await expect(page.getByRole('link', { name: /run-created-4 pending new launch/i })).toBeVisible()
  await expect(page.getByText('new launch')).toBeVisible()
})
