import { expect, test } from '@playwright/test'

test('discovers runtime, binds agent, assigns work, and cancels assignment', async ({ page }) => {
  await page.goto('/')

  await expect(page.getByRole('heading', { name: 'Runtime QA work' })).toBeVisible()
  await expect(page.getByText('No agents configured yet.')).toBeVisible()

  await page.getByRole('button', { name: 'Settings' }).click()
  await expect(page.getByRole('heading', { name: 'Agent Runtimes' })).toBeVisible()
  await page.getByRole('button', { name: 'Discover' }).click()
  await expect(page.getByText('Codex CLI')).toBeVisible()
  await expect(page.getByText('Healthy', { exact: true })).toBeVisible()

  await page.getByRole('button', { name: 'Issues' }).click()
  await expect(page.getByRole('heading', { name: 'Runtime QA work' })).toBeVisible()
  await page.getByLabel('Name').fill('Builder')
  await page.getByLabel('Role').fill('Implementation')
  await page.getByLabel('Model').fill('gpt-5')
  await page.getByRole('button', { name: 'Create Agent' }).click()

  await expect(page.getByRole('button', { name: 'Assign Agent' })).toBeEnabled()
  await page.getByRole('button', { name: 'Assign Agent' }).click()
  await expect(page.getByText('Requested assignment for Builder')).toBeVisible()
  await expect(page.getByText('Queued').first()).toBeVisible()

  await page.getByRole('button', { name: 'Cancel' }).click()
  await expect(page.getByText('Cancelled assignment for Builder')).toBeVisible()
  await expect(page.locator('span').filter({ hasText: /^Cancelled$/ })).toBeVisible()
})
