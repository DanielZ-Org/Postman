import { AxeBuilder } from '@axe-core/playwright'
import { expect, test } from '@playwright/test'

async function selectOffice(page: import('@playwright/test').Page) {
  await page.request.post('/api/v1/debug/reset')
  await page.goto('/')
  await expect(page.getByText('Choose your head office')).toBeVisible()
  const selectBtn = page.getByRole('button', { name: 'Select office' }).first()
  await selectBtn.click()
  await page.getByRole('button', { name: /Confirm — pay/ }).click()
  await expect(page.getByRole('heading', { name: 'Delivery flow' })).toBeVisible()
}

test('office select has no serious accessibility violations', async ({ page }) => {
  await page.request.post('/api/v1/debug/reset')
  await page.goto('/')
  await expect(page.getByText('Choose your head office')).toBeVisible()

  const results = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa']).analyze()
  const serious = results.violations.filter((v) => v.impact === 'serious' || v.impact === 'critical')
  expect(serious, JSON.stringify(serious.map((v) => ({ id: v.id, nodes: v.nodes.length })), null, 2)).toEqual([])
})

test('flow pipeline has no serious accessibility violations', async ({ page }) => {
  await selectOffice(page)

  const results = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa']).analyze()
  const serious = results.violations.filter((v) => v.impact === 'serious' || v.impact === 'critical')
  expect(serious, JSON.stringify(serious.map((v) => ({ id: v.id, nodes: v.nodes.length })), null, 2)).toEqual([])
})
