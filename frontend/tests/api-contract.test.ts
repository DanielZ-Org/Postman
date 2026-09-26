import { afterAll, beforeAll, beforeEach, describe, expect, it } from 'vitest'
import { startMockApiServer, type MockApiServer } from './helpers/mock-server'

interface ErrorBody {
  error?: { code?: string; message?: string; details?: unknown }
}

let server: MockApiServer

async function apiFetch(path: string, init?: RequestInit): Promise<Response> {
  return fetch(`${server.apiBase}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
  })
}

async function readJson(res: Response): Promise<unknown> {
  const text = await res.text()
  expect(text.length).toBeGreaterThan(0)
  return JSON.parse(text)
}

async function post(path: string, body: unknown = {}): Promise<Response> {
  return apiFetch(path, { method: 'POST', body: JSON.stringify(body) })
}

beforeAll(async () => {
  server = await startMockApiServer()
})

afterAll(async () => {
  await server.close()
})

beforeEach(async () => {
  const res = await post('/debug/reset')
  expect(res.status).toBe(200)
})

describe('mock API health', () => {
  it('GET /game returns a parseable initial state', async () => {
    const res = await apiFetch('/game')
    expect(res.status).toBe(200)
    const body = (await readJson(res)) as Record<string, unknown>
    expect(body).toHaveProperty('game')
    expect(body).toHaveProperty('player')
    expect(body).toHaveProperty('operations')
    expect(body).toHaveProperty('finance')
    expect(body.office).toBeNull()

    const player = body.player as Record<string, unknown>
    expect(player.cash).toBe(1000)
    expect(player.trait).toBe('financial')

    const game = body.game as Record<string, unknown>
    expect(typeof game.game_datetime).toBe('string')
    expect(game.game_datetime).toMatch(/^\d{4}-\d{2}-\d{2}T/)
  })

  it('GET /clock returns speed, pause, and payroll/rent countdowns', async () => {
    const res = await apiFetch('/clock')
    expect(res.status).toBe(200)
    const body = (await readJson(res)) as Record<string, unknown>
    expect([1, 2, 3]).toContain(body.speed)
    expect(typeof body.paused).toBe('boolean')
    expect(typeof body.office_open).toBe('boolean')
    expect(typeof body.days_until_next_payroll).toBe('number')
    expect(typeof body.days_until_next_rent).toBe('number')
    expect(typeof body.day_of_week).toBe('string')
  })
})

describe('GET /offices', () => {
  it('returns both contract options with storage and rent fields', async () => {
    const res = await apiFetch('/offices')
    expect(res.status).toBe(200)
    const body = (await readJson(res)) as { offices?: unknown[] } | unknown[]
    const list = Array.isArray(body) ? body : (body.offices ?? [])
    expect(Array.isArray(list)).toBe(true)
    expect(list.length).toBeGreaterThanOrEqual(2)

    for (const raw of list) {
      const office = raw as Record<string, unknown>
      expect(typeof office.id).toBe('string')
      expect(typeof office.down_payment).toBe('number')
      expect(typeof office.weekly_rent).toBe('number')
      expect(typeof office.is_head_office).toBe('boolean')
      expect(office.contract_status).toBeDefined()
      const storage = office.storage as Record<string, unknown>
      expect(typeof storage.base_capacity).toBe('number')
      expect(typeof storage.used_units).toBe('number')
      expect(Array.isArray(office.accepted_package_sizes)).toBe(true)
    }
  })
})

describe('POST /offices/select', () => {
  it('selects the small office and charges the down payment', async () => {
    const selectRes = await post('/offices/select', { office_id: 'office-small-01' })
    expect(selectRes.status).toBe(200)

    const gameRes = await apiFetch('/game')
    const game = (await readJson(gameRes)) as Record<string, unknown>
    const office = game.office as Record<string, unknown> | null
    expect(office).not.toBeNull()
    expect(office?.id).toBe('office-small-01')
    expect(office?.is_head_office).toBeUndefined()

    const player = game.player as Record<string, unknown>
    expect(player.cash).toBe(650)

    const ops = game.operations as Record<string, unknown>
    expect(Number(ops.stored_packages)).toBeGreaterThan(0)
  })

  it('rejects an unknown office id with a structured error', async () => {
    const res = await post('/offices/select', { office_id: 'office-ghost-01' })
    expect(res.status).toBeGreaterThanOrEqual(400)
    expect(res.status).toBeLessThan(500)
    const body = (await readJson(res)) as ErrorBody
    expect(body.error?.code).toBeTruthy()
    expect(typeof body.error?.message).toBe('string')
  })

  it('rejects a missing office_id', async () => {
    const res = await post('/offices/select', {})
    expect(res.status).toBe(400)
    const body = (await readJson(res)) as ErrorBody
    expect(body.error?.code).toBe('INVALID_OFFICE_ID')
  })
})

describe('GET /office and contract termination (SPEC 4.1/4.2)', () => {
  it('returns a null office before a selection', async () => {
    const res = await apiFetch('/office')
    expect(res.status).toBe(200)
    const body = (await readJson(res)) as Record<string, unknown>
    expect(body.office).toBeNull()
  })

  it('returns the active runtime office after a selection', async () => {
    await post('/offices/select', { office_id: 'office-small-01' })
    const res = await apiFetch('/office')
    const body = (await readJson(res)) as { office: Record<string, unknown> | null }
    expect(body.office).toBeTruthy()
    expect(body.office?.id).toBe('office-small-01')
    expect(body.office?.contract_status).toBe('active')
    expect(body.office?.missed_rent_payments).toBe(0)
    expect(body.office?.is_head_office).toBe(true)
  })

  it('keeps a terminated contract out of /game but visible on /office', async () => {
    await post('/offices/select', { office_id: 'office-small-01' })
    const terminate = await post('/debug/terminate-contract')
    expect(terminate.status).toBe(200)

    const game = (await readJson(await apiFetch('/game'))) as Record<string, unknown>
    expect(game.office).toBeNull()
    expect((game.game as Record<string, unknown>).status).toBe('running')

    const body = (await readJson(await apiFetch('/office'))) as { office: Record<string, unknown> | null }
    expect(body.office?.contract_status).toBe('terminated')
    expect(body.office?.missed_rent_payments).toBe(2)
  })

  it('lets the player re-select a contract after termination (re-entry rule)', async () => {
    await post('/offices/select', { office_id: 'office-small-01' })
    await post('/debug/terminate-contract')

    const res = await post('/offices/select', { office_id: 'office-large-01' })
    expect(res.status).toBe(200)

    const game = (await readJson(await apiFetch('/game'))) as Record<string, unknown>
    expect((game.office as Record<string, unknown> | null)?.id).toBe('office-large-01')
    expect((game.game as Record<string, unknown>).status).toBe('running')
  })

  it('refuses a second selection while the contract is active', async () => {
    await post('/offices/select', { office_id: 'office-small-01' })
    const res = await post('/offices/select', { office_id: 'office-large-01' })
    expect(res.status).toBe(409)
    const body = (await readJson(res)) as ErrorBody
    expect(body.error?.code).toBe('OFFICE_ALREADY_SELECTED')
  })

  it('ends the game when a terminated contract can no longer be replaced', async () => {
    await post('/offices/select', { office_id: 'office-small-01' })
    await post('/debug/terminate-contract')
    await post('/debug/bankrupt')

    const game = (await readJson(await apiFetch('/game'))) as Record<string, unknown>
    expect((game.game as Record<string, unknown>).status).toBe('game_over')
    expect(game.office).toBeNull()

    const clock = (await readJson(await apiFetch('/clock'))) as Record<string, unknown>
    expect(clock.paused).toBe(true)
  })

  it('rejects hiring, assigning, and re-selecting after game over', async () => {
    await post('/offices/select', { office_id: 'office-small-01' })
    await post('/debug/terminate-contract')
    await post('/debug/bankrupt')

    const hire = await post('/employees/hire')
    expect(hire.status).toBe(409)
    expect(((await readJson(hire)) as ErrorBody).error?.code).toBe('NO_OFFICE')

    const select = await post('/offices/select', { office_id: 'office-small-01' })
    expect(select.status).toBe(409)
    expect(((await readJson(select)) as ErrorBody).error?.code).toBe('GAME_OVER')
  })

  it('refuses to terminate twice or without an office', async () => {
    expect((await post('/debug/terminate-contract')).status).toBe(409)

    await post('/offices/select', { office_id: 'office-small-01' })
    await post('/debug/terminate-contract')
    expect((await post('/debug/terminate-contract')).status).toBe(409)
  })
})

describe('GET /packages', () => {
  it('returns package rows with SPEC status vocabulary', async () => {
    await post('/offices/select', { office_id: 'office-small-01' })
    const res = await apiFetch('/packages')
    expect(res.status).toBe(200)
    const body = (await readJson(res)) as { packages?: unknown[] } | unknown[]
    const list = Array.isArray(body) ? body : (body.packages ?? [])
    expect(list.length).toBeGreaterThan(0)

    const allowed = new Set(['stored', 'assigned', 'out_for_delivery', 'delivered'])
    for (const raw of list) {
      const pkg = raw as Record<string, unknown>
      expect(typeof pkg.id).toBe('string')
      expect(allowed.has(String(pkg.status))).toBe(true)
      expect(String(pkg.destination_type)).toBe('local')
      expect(typeof pkg.base_fee).toBe('number')
      expect(pkg.assigned_employee_id === null || typeof pkg.assigned_employee_id === 'string').toBe(true)
      expect(pkg.final_revenue === null || typeof pkg.final_revenue === 'number').toBe(true)
    }
  })
})

describe('employees and hiring', () => {
  beforeEach(async () => {
    await post('/offices/select', { office_id: 'office-small-01' })
  })

  it('GET /employees returns empty list and hiring state after office select', async () => {
    const res = await apiFetch('/employees')
    expect(res.status).toBe(200)
    const body = (await readJson(res)) as Record<string, unknown>
    expect(Array.isArray(body.employees)).toBe(true)
    expect(body.hiring).toBeTruthy()
    const hiring = body.hiring as Record<string, unknown>
    expect(hiring.next_hiring_fee).toBeGreaterThan(0)
    expect(hiring.current_employee_count).toBe(0)
  })

  it('POST /employees/hire charges fee and creates a ready courier', async () => {
    const before = (await readJson(await apiFetch('/game'))) as Record<string, unknown>
    const cashBefore = (before.player as Record<string, unknown>).cash as number

    const employeesBefore = (await readJson(await apiFetch('/employees'))) as Record<string, unknown>
    const feeBefore = (employeesBefore.hiring as Record<string, unknown>).next_hiring_fee as number

    const hireRes = await post('/employees/hire')
    expect(hireRes.status).toBe(200)

    const employeesBody = (await readJson(await apiFetch('/employees'))) as Record<string, unknown>
    const employees = employeesBody.employees as Record<string, unknown>[]
    expect(employees.length).toBe(1)
    expect(employees[0].status).toBe('ready')
    expect(employees[0].current_delivery_mode).toBe('foot')

    const after = (await readJson(await apiFetch('/game'))) as Record<string, unknown>
    const cashAfter = (after.player as Record<string, unknown>).cash as number
    expect(cashBefore - cashAfter).toBe(feeBefore)
    expect(feeBefore).toBeGreaterThan(0)
  })

  it('rejects hiring without an office', async () => {
    await post('/debug/reset')
    const res = await post('/employees/hire')
    expect(res.status).toBeGreaterThanOrEqual(400)
    const body = (await readJson(res)) as ErrorBody
    expect(body.error?.code).toBeTruthy()
  })
})

describe('POST /deliveries/assign full cycle', () => {
  it(
    'assigns stored packages, moves them through the pipeline, and books revenue',
    async () => {
      await post('/offices/select', { office_id: 'office-small-01' })
      await post('/employees/hire')

      const employeesBody = (await readJson(await apiFetch('/employees'))) as Record<string, unknown>
      const employee = (employeesBody.employees as Record<string, unknown>[])[0]
      expect(employee).toBeTruthy()

      const packagesBefore = (await readJson(await apiFetch('/packages'))) as { packages: Record<string, unknown>[] }
      const storedBefore = packagesBefore.packages.filter((p) => p.status === 'stored')
      expect(storedBefore.length).toBeGreaterThan(0)

      const assignRes = await post('/deliveries/assign', {
        employee_id: employee.id,
        package_count: 1,
      })
      expect(assignRes.status).toBe(200)

      const during = (await readJson(await apiFetch('/packages'))) as { packages: Record<string, unknown>[] }
      const active = during.packages.filter((p) => p.status === 'assigned' || p.status === 'out_for_delivery')
      expect(active.length).toBe(1)
      expect(active[0].assigned_employee_id).toBe(employee.id)

      const busyRes = await post('/deliveries/assign', {
        employee_id: employee.id,
        package_count: 1,
      })
      expect(busyRes.status).toBe(409)
      const busyBody = (await readJson(busyRes)) as ErrorBody
      expect(busyBody.error?.code).toBe('EMPLOYEE_BUSY')

      await post('/clock/speed', { speed: 3 })
      await post('/clock/pause', { paused: false })

      const deadline = Date.now() + 55_000
      let delivered = false
      while (Date.now() < deadline && !delivered) {
        await new Promise((r) => setTimeout(r, 500))
        const snapshot = (await readJson(await apiFetch('/packages'))) as { packages: Record<string, unknown>[] }
        delivered = snapshot.packages.some((p) => p.status === 'delivered')
      }
      expect(delivered).toBe(true)

      const finance = (await readJson(await apiFetch('/finance'))) as Record<string, unknown>
      const income = finance.income as Record<string, unknown>
      expect(Number(income.package_revenue)).toBeGreaterThan(0)

      const txns = (await readJson(await apiFetch('/finance/transactions'))) as {
        transactions: Record<string, unknown>[]
      }
      expect(txns.transactions.length).toBeGreaterThan(0)
for (const txn of txns.transactions) {
      expect(typeof txn.amount).toBe('number')
      expect(typeof txn.category).toBe('string')
      expect(typeof txn.game_datetime).toBe('string')
    }
  }, 70_000)

  it('rejects assign with an unknown employee', async () => {
    await post('/offices/select', { office_id: 'office-small-01' })
    const res = await post('/deliveries/assign', { employee_id: 'emp-ghost', package_count: 1 })
    expect(res.status).toBeGreaterThanOrEqual(400)
    expect(res.status).toBeLessThan(500)
    const body = (await readJson(res)) as ErrorBody
    expect(body.error?.code).toBeTruthy()
  })

  it('rejects assign with package_count of zero', async () => {
    await post('/offices/select', { office_id: 'office-small-01' })
    await post('/employees/hire')
    const employeesBody = (await readJson(await apiFetch('/employees'))) as Record<string, unknown>
    const employee = (employeesBody.employees as Record<string, unknown>[])[0]
    const res = await post('/deliveries/assign', { employee_id: employee.id, package_count: 0 })
    expect(res.status).toBe(400)
  })
})

describe('GET /finance', () => {
  it('returns statement with income, expenses, liabilities, and period', async () => {
    await post('/offices/select', { office_id: 'office-small-01' })
    const res = await apiFetch('/finance')
    expect(res.status).toBe(200)
    const body = (await readJson(res)) as Record<string, unknown>

    expect(typeof body.cash_balance).toBe('number')
    expect(typeof body.net_change).toBe('number')

    const period = body.period as Record<string, unknown>
    expect(typeof period.from).toBe('string')
    expect(typeof period.to).toBe('string')

    for (const key of ['income', 'expenses', 'liabilities']) {
      expect(body[key]).toBeTruthy()
      const section = body[key] as Record<string, unknown>
      for (const value of Object.values(section)) {
        if (value !== null) expect(typeof value).toBe('number')
      }
    }

    const income = body.income as Record<string, unknown>
    expect(typeof income.package_revenue).toBe('number')
    expect(typeof income.trait_bonus).toBe('number')
  })

  it('records the office down payment as a negative transaction', async () => {
    await post('/offices/select', { office_id: 'office-small-01' })
    const txns = (await readJson(await apiFetch('/finance/transactions'))) as {
      transactions: Record<string, unknown>[]
    }
    const downPayment = txns.transactions.find(
      (t) => String(t.category).includes('office') || Number(t.amount) < 0,
    )
    expect(downPayment).toBeTruthy()
    expect(Number(downPayment?.amount)).toBeLessThan(0)
  })
})

describe('clock controls', () => {
  it('accepts speeds 1, 2, 3 and rejects others', async () => {
    for (const speed of [1, 2, 3]) {
      const res = await post('/clock/speed', { speed })
      expect(res.status).toBe(200)
    }
    const bad = await post('/clock/speed', { speed: 9 })
    expect(bad.status).toBe(400)
    const body = (await readJson(bad)) as ErrorBody
    expect(body.error?.code).toBe('INVALID_SPEED')
  })

  it('toggles pause', async () => {
    await post('/clock/pause', { paused: true })
    const clock = (await readJson(await apiFetch('/clock'))) as Record<string, unknown>
    expect(clock.paused).toBe(true)

    await post('/clock/pause', { paused: false })
    const resumed = (await readJson(await apiFetch('/clock'))) as Record<string, unknown>
    expect(resumed.paused).toBe(false)
  })

  it('skip-to-next-opening moves the clock forward or keeps it during opening hours', async () => {
    const before = (await readJson(await apiFetch('/clock'))) as Record<string, unknown>
    const res = await post('/clock/skip-to-next-opening', {})
    expect(res.status).toBe(200)
    const after = (await readJson(await apiFetch('/clock'))) as Record<string, unknown>
    expect(String(after.game_datetime) >= String(before.game_datetime)).toBe(true)
  })
})

describe('error envelope', () => {
  it('unknown route returns {error:{code,message}}', async () => {
    const res = await apiFetch('/does-not-exist')
    expect(res.status).toBe(404)
    const body = (await readJson(res)) as ErrorBody
    expect(body.error?.code).toBe('NOT_FOUND')
    expect(typeof body.error?.message).toBe('string')
  })

  it('invalid JSON body returns INVALID_REQUEST, not a crash', async () => {
    const res = await apiFetch('/clock/speed', { method: 'POST', body: 'not-json' })
    expect(res.status).toBe(400)
    const body = (await readJson(res)) as ErrorBody
    expect(body.error?.code).toBe('INVALID_REQUEST')
  })
})

describe('debug reset', () => {
  it('restores cash to 1000 and clears the office', async () => {
    await post('/offices/select', { office_id: 'office-small-01' })
    await post('/employees/hire')
    await post('/debug/reset')
    const game = (await readJson(await apiFetch('/game'))) as Record<string, unknown>
    expect((game.player as Record<string, unknown>).cash).toBe(1000)
    expect(game.office).toBeNull()
  })
})
