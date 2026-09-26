import { expect, test } from '@playwright/test'

// Full-stack suite: the real React UI drives the real Go backend (built from
// ../backend) through the Vite proxy. One fresh SQLite database per run, so
// there is no debug/reset endpoint — tests are ordered and share server state.
test.describe.serial('full stack: UI against the Go backend', () => {
  test('boots into office select from the real API', async ({ page }) => {
    await page.goto('/')

    await expect(page.getByText('Choose your head office')).toBeVisible()
    await expect(page.getByText('£1,000.00').first()).toBeVisible()
    await expect(page.getByRole('heading', { name: 'Small Office' })).toBeVisible()
    await expect(page.getByRole('heading', { name: 'Large Office' })).toBeVisible()

    // mock-only control must stay hidden outside mock mode
    await expect(page.getByTitle('Reset mock game to a fresh save')).toHaveCount(0)
  })

  test('selects an office and pays the down payment', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByText('Choose your head office')).toBeVisible()

    await page.getByRole('button', { name: 'Select office' }).first().click()
    await page.getByRole('button', { name: /Confirm — pay/ }).click()

    await expect(page.getByRole('heading', { name: 'Delivery flow' })).toBeVisible()
    await expect(page.getByText('£650.00').first()).toBeVisible()
    await expect(page.getByText(/All destinations local/)).toBeVisible()
  })

  test('reads the real runtime office projection without a parse error', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByRole('heading', { name: 'Delivery flow' })).toBeVisible()

    // The client parses GET /api/v1/office (SPEC 4.1) for the contract status. A shape
    // mismatch surfaces as a sticky error banner, not a crash, so assert its absence.
    await expect(page.locator('.error-banner')).toHaveCount(0)

    const res = await page.request.get('/api/v1/office')
    expect(res.status()).toBe(200)
    const body = (await res.json()) as { office: Record<string, unknown> | null }
    expect(body.office?.id).toBe('office-small-01')
    expect(body.office?.contract_status).toBe('active')
    expect(body.office?.missed_rent_payments).toBe(0)
  })

  test('runs the clock at 3× for faster simulation', async ({ page }) => {    await page.goto('/')

    const speed3 = page.getByRole('button', { name: '3×' })
    await expect(speed3).toBeVisible()
    await speed3.click()
    await expect(speed3).toHaveClass(/is-active/)
  })

  test('hires the first walking courier', async ({ page }) => {
    await page.goto('/')

    await page.getByRole('tab', { name: 'Employees' }).click()
    await page.getByRole('button', { name: 'Hire walking employee' }).click()
    await expect(page.getByText('Bob Snail')).toBeVisible({ timeout: 15_000 })
  })

  test('warehouse fills from real package generation', async ({ page }) => {
    await page.goto('/')

    await page.getByRole('tab', { name: 'Deliveries' }).click()
    await expect(page.getByText(/[1-9]\d* stored package\(s\) available/)).toBeVisible({
      timeout: 30_000,
    })
  })

  test('assigns a batch and the courier walks out', async ({ page }) => {
    await page.goto('/')

    await page.getByRole('tab', { name: 'Deliveries' }).click()
    await page.getByRole('button', { name: /^Assign \d+ package\(s\)$/ }).click()

    await page.getByRole('tab', { name: 'Flow' }).click()
    await expect(page.locator('.courier-card.is-packing')).toBeVisible({ timeout: 15_000 })
    await expect(page.locator('.courier-card.is-out_for_delivery')).toBeVisible({ timeout: 30_000 })
  })

  test('delivers the package and collects revenue', async ({ page }) => {
    await page.goto('/')

    await page.getByRole('tab', { name: 'Flow' }).click()
    await expect(page.locator('.stage-delivered .flow-count')).toHaveText(/^[1-9]\d*$/, {
      timeout: 60_000,
    })
    await expect(page.locator('.pkg-chip-rev').first()).toBeVisible({ timeout: 15_000 })
  })

  test('finance statement and transactions come from the backend', async ({ page }) => {
    await page.goto('/')

    await page.getByRole('tab', { name: 'Finance' }).click()
    await expect(page.getByRole('heading', { name: 'Finance statement' })).toBeVisible()
    await expect(page.getByRole('heading', { name: 'Transactions' })).toBeVisible()

    const revenueRow = page.locator('.finance-row', { hasText: 'Package revenue' }).locator('.mono')
    await expect(revenueRow).not.toHaveText('£0.00')

    await expect(page.getByText('office_down_payment').first()).toBeVisible()
    await expect(page.getByText('hiring_bonus').first()).toBeVisible()
    await expect(page.getByText('loan_disbursement').first()).toBeVisible()
  })

  test('pause and resume round trip against the clock API', async ({ page }) => {
    await page.goto('/')

    const pause = page.getByTitle('Pause')
    await expect(pause).toBeVisible()
    await pause.click()
    await expect(page.getByTitle('Resume')).toBeVisible()

    await page.getByTitle('Resume').click()
    await expect(page.getByTitle('Pause')).toBeVisible()
  })
})
