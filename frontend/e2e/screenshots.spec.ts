import { mkdirSync } from 'node:fs'
import { resolve } from 'node:path'
import { expect, test } from '@playwright/test'

const OUT = resolve(process.cwd(), '../docs/screenshots')

test.beforeAll(() => {
  mkdirSync(OUT, { recursive: true })
})

async function selectOffice(page: import('@playwright/test').Page) {
  await page.request.post('/api/v1/debug/reset')
  await page.goto('/')
  await expect(page.getByText('Choose your head office')).toBeVisible()
  const selectBtn = page.getByRole('button', { name: 'Select office' }).first()
  await selectBtn.click()
  await page.getByRole('button', { name: /Confirm — pay/ }).click()
  await expect(page.getByRole('heading', { name: 'Delivery flow' })).toBeVisible()
}

test('capture app screenshots', async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.request.post('/api/v1/debug/reset')
  await page.goto('/')
  await expect(page.getByText('Choose your head office')).toBeVisible()
  await page.screenshot({ path: `${OUT}/01-office-select.png`, fullPage: true })

  await selectOffice(page)
  await page.screenshot({ path: `${OUT}/02-flow-pipeline.png`, fullPage: true })

  await page.getByRole('tab', { name: 'Packages' }).click()
  await expect(page.getByRole('heading', { name: 'Packages' })).toBeVisible()
  await page.screenshot({ path: `${OUT}/03-packages.png`, fullPage: true })

  await page.getByRole('tab', { name: 'Employees' }).click()
  await expect(page.getByRole('heading', { name: 'Employees' })).toBeVisible()
  await page.getByRole('button', { name: 'Hire walking employee' }).click()
  await expect(page.getByText('Bob Snail')).toBeVisible({ timeout: 15_000 })
  await page.screenshot({ path: `${OUT}/04-employees.png`, fullPage: true })

  await page.getByRole('tab', { name: 'Deliveries' }).click()
  await expect(page.getByRole('heading', { name: 'Assign delivery batch' })).toBeVisible()
  await page.getByRole('button', { name: /^Assign \d+ package\(s\)$/ }).click()
  await page.screenshot({ path: `${OUT}/05-deliveries.png`, fullPage: true })

  await page.getByRole('tab', { name: 'Flow' }).click()
  await expect(page.locator('.courier-card').first()).toBeVisible({ timeout: 10_000 })
  await page.screenshot({ path: `${OUT}/06-flow-with-courier.png`, fullPage: true })

  await page.getByRole('tab', { name: 'Finance' }).click()
  await expect(page.getByRole('heading', { name: 'Finance statement' })).toBeVisible()
  await page.screenshot({ path: `${OUT}/07-finance.png`, fullPage: true })
})
