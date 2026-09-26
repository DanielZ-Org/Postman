import { describe, expect, it, vi } from 'vitest'
import { ApiError, api, toApiError } from './client'
import {
  makeClock,
  makeEmployee,
  makeFinance,
  makeGameState,
  makeHiring,
  makeOffice,
  makePackage,
  makeTransaction,
} from '../test/factories'

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('ApiError / toApiError', () => {
  it('passes through ApiError instances', () => {
    const original = new ApiError('X', 'msg', 400)
    expect(toApiError(original)).toBe(original)
  })

  it('wraps plain errors as UNEXPECTED_ERROR', () => {
    const wrapped = toApiError(new Error('boom'))
    expect(wrapped.code).toBe('UNEXPECTED_ERROR')
    expect(wrapped.message).toBe('boom')
    expect(wrapped.status).toBe(0)
  })

  it('wraps non-error values', () => {
    expect(toApiError('nope').message).toBe('nope')
  })
})

describe('api.getClock', () => {
  it('parses a valid flat clock payload', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, makeClock()))
    vi.stubGlobal('fetch', fetchMock)

    const clock = await api.getClock()
    expect(clock.game_datetime).toBe('1980-02-01T09:00:00.000Z')
    expect(clock.speed).toBe(1)
    expect(clock.paused).toBe(false)
    expect(fetchMock).toHaveBeenCalledWith(expect.stringContaining('/clock'), expect.anything())
  })

  it('unwraps a Go {clock:{...}} payload', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, { clock: makeClock() })))
    const clock = await api.getClock()
    expect(clock.game_datetime).toBe('1980-02-01T09:00:00.000Z')
    expect(clock.speed).toBe(1)
  })

  it('throws ApiError when a required field is missing', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, { speed: 1 })))
    await expect(api.getClock()).rejects.toMatchObject({
      name: 'ApiError',
      code: 'UNEXPECTED_RESPONSE',
    })
  })

  it('throws ApiError on network failure', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('failed to fetch')))
    await expect(api.getGame()).rejects.toMatchObject({ code: 'BACKEND_UNREACHABLE' })
  })

  it('throws INVALID_JSON when body is not JSON', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(new Response('<html>oops</html>', { status: 200 })),
    )
    await expect(api.getGame()).rejects.toMatchObject({ code: 'INVALID_JSON' })
  })
})

describe('api error envelope parsing', () => {
  it('extracts code/message/status/details from {error:{...}}', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        jsonResponse(409, {
          error: { code: 'EMPLOYEE_BUSY', message: 'Employee is busy', details: { employee_id: 'emp-1' } },
        }),
      ),
    )

    await expect(api.hireEmployee()).rejects.toMatchObject({
      code: 'EMPLOYEE_BUSY',
      status: 409,
      message: 'Employee is busy',
      details: { employee_id: 'emp-1' },
    })
  })

  it('falls back to HTTP_<status> for non-JSON error bodies', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(new Response('gateway sad', { status: 502, statusText: 'Bad Gateway' })),
    )
    await expect(api.getFinance()).rejects.toMatchObject({ code: 'HTTP_502', status: 502 })
  })
})

describe('api.getGameState', () => {
  it('parses a full game state', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, makeGameState())))
    const state = await api.getGame()
    expect(state).not.toBeNull()
    expect(state?.player.cash).toBe(1000)
    expect(state?.office?.id).toBe('office-small-01')
    expect(state?.operations.stored_packages).toBe(6)
  })

  it('unwraps a game-state wrapper', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(jsonResponse(200, { 'game-state': makeGameState({ office: null }) })),
    )
    const state = await api.getGame()
    expect(state).not.toBeNull()
    expect(state?.office).toBeNull()
  })

  it('returns null when the route is missing', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        jsonResponse(404, { error: { code: 'HTTP_404', message: 'not found' } }),
      ),
    )
    await expect(api.getGame()).resolves.toBeNull()
  })

  it('allows a null office (pre-select)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(jsonResponse(200, makeGameState({ office: null }))),
    )
    const state = await api.getGame()
    expect(state?.office).toBeNull()
  })
})

describe('api.getOffices', () => {
  it('parses offices from {offices:[...]}', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(jsonResponse(200, { offices: [makeOffice()] })),
    )
    const offices = await api.getOffices()
    expect(offices).toHaveLength(1)
    expect(offices[0].storage.base).toBe(100)
    expect(offices[0].storage.max).toBe(150)
  })

  it('parses Go catalogue shape {storage:{base,max}}', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        jsonResponse(200, {
          offices: [
            {
              id: 'office-small-01',
              type: 'small',
              down_payment: 350,
              weekly_rent: 50,
              rent_prepaid_weeks: 4,
              storage: { base: 100, max: 150 },
              employee_capacity: 5,
              bicycle_capacity: 5,
              vehicle_capacity: 1,
              accepted_package_sizes: ['small', 'medium'],
            },
          ],
        }),
      ),
    )
    const offices = await api.getOffices()
    expect(offices[0].storage).toEqual({ base: 100, max: 150 })
    expect(offices[0].down_payment).toBe(350)
  })

  it('throws when payload is not a list', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, { nope: true })))
    await expect(api.getOffices()).rejects.toMatchObject({ code: 'UNEXPECTED_RESPONSE' })
  })
})

describe('api.getOffice', () => {
  it('parses the SPEC 4.1 runtime office with its contract status', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, { office: makeOffice() })))
    const office = await api.getOffice()
    expect(office?.id).toBe('office-small-01')
    expect(office?.contract_status).toBe('active')
    expect(office?.missed_rent_payments).toBe(0)
    expect(office?.storage.current_capacity).toBe(100)
  })

  it('parses the Go storage key set (base/current/max/used)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        jsonResponse(200, {
          office: {
            id: 'office-small-01',
            type: 'small',
            is_head_office: true,
            down_payment: 350,
            weekly_rent: 50,
            rent_prepaid_weeks: 4,
            next_rent_due: '',
            storage: { base: 100, current: 100, max: 150, used: 6 },
            employee_capacity: 5,
            bicycle_capacity: 5,
            vehicle_capacity: 1,
            accepted_package_sizes: ['small'],
            contract_status: 'terminated',
            missed_rent_payments: 2,
          },
        }),
      ),
    )
    const office = await api.getOffice()
    expect(office?.storage.used_units).toBe(6)
    expect(office?.storage.maximum_capacity).toBe(150)
    expect(office?.contract_status).toBe('terminated')
    expect(office?.missed_rent_payments).toBe(2)
  })

  it('returns null before an office is selected', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, { office: null })))
    await expect(api.getOffice()).resolves.toBeNull()
  })

  it('returns null when the route is missing', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(404, '404 page not found')))
    await expect(api.getOffice()).resolves.toBeNull()
  })

  it('rejects a payload without contract_status', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(jsonResponse(200, { office: { id: 'office-small-01' } })),
    )
    await expect(api.getOffice()).rejects.toMatchObject({ code: 'UNEXPECTED_RESPONSE' })
  })

  it('rejects a payload without the office wrapper', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, { nope: true })))
    await expect(api.getOffice()).rejects.toMatchObject({ code: 'UNEXPECTED_RESPONSE' })
  })
})

describe('api.selectOffice', () => {
  it('parses Go {office, cash_balance} response', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        jsonResponse(200, {
          office: { id: 'office-small-01', type: 'small' },
          cash_balance: 650,
        }),
      ),
    )
    const result = await api.selectOffice('office-small-01')
    expect(result.cash_balance).toBe(650)
    expect(result.office_id).toBe('office-small-01')
  })

  it('parses mock full game-state response', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        jsonResponse(200, makeGameState({ player: { id: 'p', cash: 650, trait: 'financial' } })),
      ),
    )
    const result = await api.selectOffice('office-small-01')
    expect(result.cash_balance).toBe(650)
    expect(result.state?.player.cash).toBe(650)
  })
})

describe('api.getPackages', () => {
  it('parses package rows', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(jsonResponse(200, { packages: [makePackage()] })),
    )
    const packages = await api.getPackages()
    expect(packages[0].destination_type).toBe('local')
    expect(packages[0].final_revenue).toBeNull()
  })

  it('returns [] when the route is missing', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(404, '404 page not found')))
    await expect(api.getPackages()).resolves.toEqual([])
  })
})

describe('api.getEmployees', () => {
  it('parses employees with hiring metadata', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        jsonResponse(200, { employees: [makeEmployee()], hiring: makeHiring() }),
      ),
    )
    const result = await api.getEmployees()
    expect(result.employees).toHaveLength(1)
    expect(result.hiring?.next_hiring_fee).toBe(100)
  })

  it('supports a bare array payload', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, [makeEmployee()])))
    const result = await api.getEmployees()
    expect(result.employees).toHaveLength(1)
    expect(result.hiring).toBeNull()
  })
})

describe('api.getFinance / getTransactions', () => {
  it('parses a finance statement', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(200, makeFinance())))
    const finance = await api.getFinance()
    expect(finance?.income.total).toBe(0)
    expect(finance?.liabilities.next_rent_amount).toBe(50)
  })

  it('returns null when finance route is missing', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(404, '404 page not found')))
    await expect(api.getFinance()).resolves.toBeNull()
  })

  it('parses transactions', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(jsonResponse(200, { transactions: [makeTransaction()] })),
    )
    const txns = await api.getTransactions()
    expect(txns[0].category).toBe('office_down_payment')
  })
})

describe('mutation bodies', () => {
  it('POST /clock/speed sends {speed}', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { ok: true }))
    vi.stubGlobal('fetch', fetchMock)
    await api.setSpeed(3)
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toContain('/clock/speed')
    expect(init.method).toBe('POST')
    expect(JSON.parse(String(init.body))).toEqual({ speed: 3 })
  })

  it('POST /deliveries/assign sends employee_id and package_count', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { ok: true }))
    vi.stubGlobal('fetch', fetchMock)
    await api.assignDelivery('emp-1', 4)
    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(JSON.parse(String(init.body))).toEqual({ employee_id: 'emp-1', package_count: 4 })
  })

  it('POST /offices/select sends office_id', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { ok: true }))
    vi.stubGlobal('fetch', fetchMock)
    await api.selectOffice('large')
    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(JSON.parse(String(init.body))).toEqual({ office_id: 'large' })
  })
})
