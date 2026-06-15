import { expect, test } from '@playwright/test'

test('@live workflow detail can create a run against the live API', async ({ page }) => {
  await page.goto('/workflows/feature-development')

  await expect(page.getByRole('heading', { name: 'Feature Development' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Create run' })).toBeVisible()

  await expect(page.getByRole('button', { name: 'Create run' })).toBeDisabled()

  await page.getByLabel('feature_name').fill('Playwright live smoke run')
  await page.getByLabel('target_repo').fill('github.com/ralleh-ai/ralleh-flow')

  const createRunResponsePromise = page.waitForResponse((response) => {
    return response.url().endsWith('/v1/runs') && response.request().method() === 'POST'
  })

  await page.getByRole('button', { name: 'Create run' }).click()

  const createRunResponse = await createRunResponsePromise
  expect(createRunResponse.status()).toBe(201)

  const createRunPayload = await createRunResponse.json() as { id: string, status: string }
  expect(createRunPayload.id).toMatch(/^run_/)

  await expect(page.getByText('Run created:')).toBeVisible()
  await expect(page.getByRole('link', { name: createRunPayload.id, exact: true })).toBeVisible()
  await expect(page.getByText('new launch')).toBeVisible()
})
