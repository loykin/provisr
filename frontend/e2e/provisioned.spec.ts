import { expect, test } from '@playwright/test'
import { ADMIN_PASSWORD, ADMIN_USERNAME, API_PROCESS_NAME, INLINE_PROCESS_NAME } from './env'

const LOCKED_HINT = /Defined in the main config file/

test.beforeEach(async ({ page }) => {
  await page.goto('')
  await page.getByLabel('Username').fill(ADMIN_USERNAME)
  await page.getByLabel('Password').fill(ADMIN_PASSWORD)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page.getByRole('heading', { name: 'Processes' })).toBeVisible()
})

test('a process declared in [[processes]] shows the config badge and locks edit/unregister', async ({ page }) => {
  const row = page.getByRole('row').filter({ hasText: INLINE_PROCESS_NAME })
  await expect(row.getByText('config', { exact: true })).toBeVisible()

  const lockedButtons = row.getByRole('button', { name: LOCKED_HINT })
  await expect(lockedButtons).toHaveCount(2) // Edit + Unregister
  for (const button of await lockedButtons.all()) {
    await expect(button).toBeDisabled()
  }
})

test('a process registered through the API has no badge and stays fully editable', async ({ page }) => {
  const row = page.getByRole('row').filter({ hasText: API_PROCESS_NAME })
  await expect(row.getByText('config', { exact: true })).toHaveCount(0)

  await expect(row.getByRole('button', { name: 'Edit process' })).toBeEnabled()
  await expect(row.getByRole('button', { name: 'Unregister process' })).toBeEnabled()
})
