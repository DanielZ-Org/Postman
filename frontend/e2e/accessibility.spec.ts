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
  await page.waitForTimeout(400)
}

test('office select has no serious accessibility violations', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.request.post('/api/v1/debug/reset')
  await page.goto('/')
  await expect(page.getByText('Choose your head office')).toBeVisible()

  const results = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa']).analyze()
  const serious = results.violations.filter((v) => v.impact === 'serious' || v.impact === 'critical')
  expect(serious, JSON.stringify(serious.map((v) => ({ id: v.id, nodes: v.nodes.length })), null, 2)).toEqual([])
})

test('flow pipeline has no serious accessibility violations', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await selectOffice(page)

  const results = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa']).analyze()
  const serious = results.violations.filter((v) => v.impact === 'serious' || v.impact === 'critical')
  expect(serious, JSON.stringify(serious.map((v) => ({ id: v.id, nodes: v.nodes.length })), null, 2)).toEqual([])
})

async function seriousViolations(page: import('@playwright/test').Page) {
  const results = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa']).analyze()
  return results.violations.filter((v) => v.impact === 'serious' || v.impact === 'critical')
}

test('terminated contract notice has no serious accessibility violations', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await selectOffice(page)

  await page.request.post('/api/v1/debug/terminate-contract')
  await expect(page.getByText('Your small office contract was terminated.')).toBeVisible()
  await page.waitForTimeout(400)

  const serious = await seriousViolations(page)
  expect(serious, JSON.stringify(serious.map((v) => ({ id: v.id, nodes: v.nodes.length })), null, 2)).toEqual([])
})

test('game over screen has no serious accessibility violations', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await selectOffice(page)

  await page.request.post('/api/v1/debug/terminate-contract')
  await page.request.post('/api/v1/debug/bankrupt')
  await expect(page.getByRole('heading', { name: 'Game over' })).toBeVisible()
  await page.waitForTimeout(400)

  const serious = await seriousViolations(page)
  expect(serious, JSON.stringify(serious.map((v) => ({ id: v.id, nodes: v.nodes.length })), null, 2)).toEqual([])
})
