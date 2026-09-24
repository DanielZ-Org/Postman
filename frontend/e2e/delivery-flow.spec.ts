import { expect, test } from '@playwright/test'

async function resetGame(page: import('@playwright/test').Page) {
  await page.request.post('/api/v1/debug/reset')
}

test.beforeEach(async ({ page }) => {
  await resetGame(page)
})

test('boots into office select with cash and contract options', async ({ page }) => {
  await page.goto('/')

  await expect(page.getByText('Choose your head office')).toBeVisible()
  await expect(page.getByText('Delivery Office')).toBeVisible()
  await expect(page.getByText('£1,000.00').first()).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Small Office' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Large Office' })).toBeVisible()
})

test('selects an office and reveals the flow pipeline', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByText('Choose your head office')).toBeVisible()

  const selectBtn = page.getByRole('button', { name: 'Select office' }).first()
  await selectBtn.click()
  await page.getByRole('button', { name: /Confirm — pay/ }).click()

  await expect(page.getByRole('heading', { name: 'Delivery flow' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'In warehouse' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Packing' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Out for delivery' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Delivered' })).toBeVisible()
  await expect(page.getByText('£650.00').first()).toBeVisible()
  await expect(page.getByText(/All destinations local/)).toBeVisible()
})

test('hire → assign → package leaves warehouse', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByText('Choose your head office')).toBeVisible()
  const selectBtn = page.getByRole('button', { name: 'Select office' }).first()
  await selectBtn.click()
  await page.getByRole('button', { name: /Confirm — pay/ }).click()
  await expect(page.getByRole('heading', { name: 'Delivery flow' })).toBeVisible()

  await page.getByRole('tab', { name: 'Employees' }).click()
  await expect(page.getByRole('heading', { name: 'Employees' })).toBeVisible()
  await page.getByRole('button', { name: 'Hire walking employee' }).click()
  await expect(page.getByText('Bob Snail')).toBeVisible({ timeout: 15_000 })

  await page.getByRole('tab', { name: 'Deliveries' }).click()
  await expect(page.getByRole('heading', { name: 'Assign delivery batch' })).toBeVisible()
  await expect(page.getByText(/stored package\(s\) available/)).toHaveText(/\d+ stored package\(s\) available/)

  await page.getByRole('button', { name: /^Assign \d+ package\(s\)$/ }).click()

  await page.getByRole('tab', { name: 'Flow' }).click()
  await expect(page.getByRole('heading', { name: 'Out for delivery' })).toBeVisible()
  await expect(page.locator('.courier-card').first()).toBeVisible({ timeout: 10_000 })
})

test('clock controls change speed and pause state', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByText('Choose your head office')).toBeVisible()

  await page.getByRole('button', { name: '3×' }).click()
  await expect(page.getByRole('button', { name: '3×' })).toHaveClass(/is-active/)

  await page.getByTitle('Pause').click()
  await expect(page.getByTitle('Resume')).toBeVisible()
})

test('new game resets to starting cash', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByText('Choose your head office')).toBeVisible()

  const selectBtn = page.getByRole('button', { name: 'Select office' }).first()
  await selectBtn.click()
  await page.getByRole('button', { name: /Confirm — pay/ }).click()
  await expect(page.getByText('£650.00').first()).toBeVisible()

  await page.getByTitle('Reset mock game to a fresh save').click()
  await expect(page.getByText('Choose your head office')).toBeVisible({ timeout: 10_000 })
  await expect(page.getByText('£1,000.00').first()).toBeVisible()
})

test('finance tab shows statement after office select', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByText('Choose your head office')).toBeVisible()
  const selectBtn = page.getByRole('button', { name: 'Select office' }).first()
  await selectBtn.click()
  await page.getByRole('button', { name: /Confirm — pay/ }).click()
  await expect(page.getByRole('heading', { name: 'Delivery flow' })).toBeVisible()

  await page.getByRole('tab', { name: 'Finance' }).click()
  await expect(page.getByRole('heading', { name: 'Finance statement' })).toBeVisible()
  await expect(page.getByText('Package revenue')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Transactions' })).toBeVisible()
  await expect(page.getByText(/office/i).first()).toBeVisible()
})
