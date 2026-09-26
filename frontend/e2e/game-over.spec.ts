import { expect, test } from '@playwright/test'

// SPEC 4.2: two missed rent payments terminate the contract. A terminated contract
// that the player can still pay for frees the head-office slot (re-selection); one
// they cannot ends the run. The mock's debug hooks set up those preconditions — the
// game-over decision itself is still taken by the mock's SPEC 4.2 rule.

async function resetGame(page: import('@playwright/test').Page) {
  await page.request.post('/api/v1/debug/reset')
}

async function selectSmallOffice(page: import('@playwright/test').Page) {
  await page.goto('/')
  await expect(page.getByText('Choose your head office')).toBeVisible()
  await page.getByRole('button', { name: 'Select office' }).first().click()
  await page.getByRole('button', { name: /Confirm — pay/ }).click()
  await expect(page.getByRole('heading', { name: 'Delivery flow' })).toBeVisible()
}

test.beforeEach(async ({ page }) => {
  await resetGame(page)
})

test('a terminated contract explains itself and re-offers an office', async ({ page }) => {
  await selectSmallOffice(page)
  await page.request.post('/api/v1/debug/terminate-contract')

  await expect(page.getByText('Your small office contract was terminated.')).toBeVisible({ timeout: 10_000 })
  await expect(page.getByText(/2 rent payments were missed/)).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Sign a new head office' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Small Office' })).toBeVisible()
  // the terminated contract is gone from the dashboard, not just dimmed
  await expect(page.getByRole('tab', { name: 'Flow' })).toHaveCount(0)
})

test('re-selecting after termination restores the dashboard', async ({ page }) => {
  await selectSmallOffice(page)
  await page.request.post('/api/v1/debug/terminate-contract')
  await expect(page.getByText('Your small office contract was terminated.')).toBeVisible({ timeout: 10_000 })

  await page.getByRole('button', { name: 'Select office' }).first().click()
  await page.getByRole('button', { name: /Confirm — pay/ }).click()

  await expect(page.getByRole('heading', { name: 'Delivery flow' })).toBeVisible({ timeout: 10_000 })
  await expect(page.getByText('Your small office contract was terminated.')).toHaveCount(0)
  await expect(page.getByRole('tab', { name: 'Finance' })).toBeVisible()
})

test('an unaffordable replacement ends the run with a game over screen', async ({ page }) => {
  await selectSmallOffice(page)
  await page.request.post('/api/v1/debug/terminate-contract')
  await page.request.post('/api/v1/debug/bankrupt')

  await expect(page.getByRole('heading', { name: 'Game over' })).toBeVisible({ timeout: 10_000 })
  await expect(page.getByText(/2 missed rent payments/)).toBeVisible()
  await expect(page.getByText('Final standings')).toBeVisible()
  // the run is terminal: no dashboard, no clock controls
  await expect(page.getByRole('tab', { name: 'Flow' })).toHaveCount(0)
  await expect(page.getByTitle('Skip to next opening')).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Select office' })).toHaveCount(0)
})

test('the backend refuses a new contract once the run has ended', async ({ page }) => {
  await selectSmallOffice(page)
  await page.request.post('/api/v1/debug/terminate-contract')
  await page.request.post('/api/v1/debug/bankrupt')
  await expect(page.getByRole('heading', { name: 'Game over' })).toBeVisible({ timeout: 10_000 })

  const res = await page.request.post('/api/v1/offices/select', { data: { office_id: 'office-small-01' } })
  expect(res.status()).toBe(409)
  const body = (await res.json()) as { error?: { code?: string } }
  expect(body.error?.code).toBe('GAME_OVER')
})

test('a new game clears the game over screen', async ({ page }) => {
  await selectSmallOffice(page)
  await page.request.post('/api/v1/debug/terminate-contract')
  await page.request.post('/api/v1/debug/bankrupt')
  await expect(page.getByRole('heading', { name: 'Game over' })).toBeVisible({ timeout: 10_000 })

  await page.getByRole('button', { name: 'Start a new game' }).click()

  await expect(page.getByRole('heading', { name: 'Game over' })).toHaveCount(0, { timeout: 10_000 })
  await expect(page.getByText('Choose your head office')).toBeVisible()
  await expect(page.getByText('£1,000.00').first()).toBeVisible()
})
