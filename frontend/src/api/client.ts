import type {
  ClockState,
  Employee,
  FinanceStatement,
  GameState,
  HiringState,
  Office,
  Package,
  Transaction,
} from './types'

function apiBase(): string {
  const configured = (import.meta as { env?: Record<string, string | undefined> }).env?.VITE_API_BASE
  return configured && configured.length > 0 ? configured : '/api/v1'
}

export class ApiError extends Error {
  readonly code: string
  readonly status: number
  readonly details: Record<string, unknown> | undefined

  constructor(code: string, message: string, status: number, details?: Record<string, unknown>) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
    this.details = details
  }
}

export function toApiError(err: unknown): ApiError {
  if (err instanceof ApiError) return err
  const message = err instanceof Error ? err.message : String(err)
  return new ApiError('UNEXPECTED_ERROR', message, 0)
}

function describe(value: unknown): string {
  if (value === null) return 'null'
  if (Array.isArray(value)) return 'array'
  return typeof value
}

function requireObject(value: unknown, path: string): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    throw new ApiError('UNEXPECTED_RESPONSE', `${path} must be an object, got ${describe(value)}`, 200)
  }
  return value as Record<string, unknown>
}

function requireNumber(obj: Record<string, unknown>, key: string, path: string): number {
  const value = obj[key]
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    throw new ApiError('UNEXPECTED_RESPONSE', `${path}.${key} must be a number, got ${describe(value)}`, 200)
  }
  return value
}

function requireString(obj: Record<string, unknown>, key: string, path: string): string {
  const value = obj[key]
  if (typeof value !== 'string') {
    throw new ApiError('UNEXPECTED_RESPONSE', `${path}.${key} must be a string, got ${describe(value)}`, 200)
  }
  return value
}

function requireBoolean(obj: Record<string, unknown>, key: string, path: string): boolean {
  const value = obj[key]
  if (typeof value !== 'boolean') {
    throw new ApiError('UNEXPECTED_RESPONSE', `${path}.${key} must be a boolean, got ${describe(value)}`, 200)
  }
  return value
}

function requireStringArray(obj: Record<string, unknown>, key: string, path: string): string[] {
  const value = obj[key]
  if (!Array.isArray(value)) {
    throw new ApiError('UNEXPECTED_RESPONSE', `${path}.${key} must be an array, got ${describe(value)}`, 200)
  }
  return value.map((item, index) => {
    if (typeof item !== 'string') {
      throw new ApiError('UNEXPECTED_RESPONSE', `${path}.${key}[${index}] must be a string, got ${describe(item)}`, 200)
    }
    return item
  })
}

function optionalString(obj: Record<string, unknown>, key: string, path: string): string | null {
  const value = obj[key]
  if (value === null || value === undefined) return null
  if (typeof value !== 'string') {
    throw new ApiError('UNEXPECTED_RESPONSE', `${path}.${key} must be a string or null, got ${describe(value)}`, 200)
  }
  return value
}

function optionalNumber(obj: Record<string, unknown>, key: string, path: string): number | null {
  const value = obj[key]
  if (value === null || value === undefined) return null
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    throw new ApiError('UNEXPECTED_RESPONSE', `${path}.${key} must be a number or null, got ${describe(value)}`, 200)
  }
  return value
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const base = apiBase()
  const headers = new Headers(init?.headers)
  if (init?.body) headers.set('Content-Type', 'application/json')

  let res: Response
  try {
    res = await fetch(`${base}${path}`, {
      ...init,
      headers,
      signal: init?.signal ?? AbortSignal.timeout(10_000),
    })
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    throw new ApiError('BACKEND_UNREACHABLE', `Cannot reach backend at ${base}${path}: ${message}`, 0)
  }

  const text = await res.text()

  if (!res.ok) {
    let apiError: { code?: unknown; message?: unknown; details?: unknown } | null = null
    if (text.length > 0) {
      try {
        const parsed: unknown = JSON.parse(text)
        if (parsed !== null && typeof parsed === 'object' && 'error' in parsed) {
          const candidate = (parsed as { error: unknown }).error
          if (candidate !== null && typeof candidate === 'object') {
            apiError = candidate as { code?: unknown; message?: unknown; details?: unknown }
          }
        }
      } catch {
        apiError = null
      }
    }
    if (apiError && typeof apiError.code === 'string' && typeof apiError.message === 'string') {
      const details =
        apiError.details !== null && typeof apiError.details === 'object' && !Array.isArray(apiError.details)
          ? (apiError.details as Record<string, unknown>)
          : undefined
      throw new ApiError(apiError.code, apiError.message, res.status, details)
    }
    const snippet = text.length > 0 ? `: ${text.slice(0, 300)}` : ''
    throw new ApiError(
      `HTTP_${res.status}`,
      `HTTP ${res.status} ${res.statusText || 'error'} from ${path}${snippet}`,
      res.status,
    )
  }

  if (text.length === 0) {
    return undefined as T
  }

  let parsed: unknown
  try {
    parsed = JSON.parse(text)
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    throw new ApiError('INVALID_JSON', `Invalid JSON from ${path}: ${message}. Body: ${text.slice(0, 300)}`, res.status)
  }
  if (parsed === null || parsed === undefined) {
    throw new ApiError('EMPTY_RESPONSE', `Null JSON response from ${path}`, res.status)
  }
  return parsed as T
}

function toList<T>(payload: unknown, keys: string[], source: string): T[] {
  if (Array.isArray(payload)) return payload as T[]
  if (payload !== null && typeof payload === 'object') {
    const record = payload as Record<string, unknown>
    for (const key of keys) {
      const value = record[key]
      if (Array.isArray(value)) return value as T[]
    }
  }
  throw new ApiError(
    'UNEXPECTED_RESPONSE',
    `Expected a list from ${source} (array or one of keys: ${keys.join(', ')}), got ${describe(payload)}`,
    200,
  )
}

function parseClock(payload: unknown): ClockState {
  const o = requireObject(payload, 'clock')
  return {
    game_datetime: requireString(o, 'game_datetime', 'clock'),
    day_of_week: requireString(o, 'day_of_week', 'clock'),
    speed: requireNumber(o, 'speed', 'clock'),
    paused: requireBoolean(o, 'paused', 'clock'),
    office_open: requireBoolean(o, 'office_open', 'clock'),
    days_until_next_payroll: requireNumber(o, 'days_until_next_payroll', 'clock'),
    days_until_next_rent: requireNumber(o, 'days_until_next_rent', 'clock'),
  }
}

function parseGameState(payload: unknown): GameState {
  const root = requireObject(payload, 'game-state')
  const game = requireObject(root.game, 'game-state.game')
  const player = requireObject(root.player, 'game-state.player')
  const operations = requireObject(root.operations, 'game-state.operations')
  const finance = requireObject(root.finance, 'game-state.finance')

  let office: GameState['office'] = null
  if (root.office !== null && root.office !== undefined) {
    const o = requireObject(root.office, 'game-state.office')
    office = {
      id: requireString(o, 'id', 'game-state.office'),
      storage_used: requireNumber(o, 'storage_used', 'game-state.office'),
      storage_capacity: requireNumber(o, 'storage_capacity', 'game-state.office'),
      employee_count: requireNumber(o, 'employee_count', 'game-state.office'),
      employee_capacity: requireNumber(o, 'employee_capacity', 'game-state.office'),
    }
  }

  return {
    game: {
      status: requireString(game, 'status', 'game-state.game'),
      game_datetime: requireString(game, 'game_datetime', 'game-state.game'),
      speed: requireNumber(game, 'speed', 'game-state.game'),
    },
    player: {
      id: requireString(player, 'id', 'game-state.player'),
      cash: requireNumber(player, 'cash', 'game-state.player'),
      trait: requireString(player, 'trait', 'game-state.player'),
    },
    office,
    operations: {
      stored_packages: requireNumber(operations, 'stored_packages', 'game-state.operations'),
      out_for_delivery: requireNumber(operations, 'out_for_delivery', 'game-state.operations'),
      delivered_today: requireNumber(operations, 'delivered_today', 'game-state.operations'),
    },
    finance: {
      accrued_wages: requireNumber(finance, 'accrued_wages', 'game-state.finance'),
      next_rent: requireNumber(finance, 'next_rent', 'game-state.finance'),
      loan_principal: requireNumber(finance, 'loan_principal', 'game-state.finance'),
    },
  }
}

function parseOffice(value: unknown, path: string): Office {
  const o = requireObject(value, path)
  const storage = requireObject(o.storage, `${path}.storage`)
  return {
    id: requireString(o, 'id', path),
    type: requireString(o, 'type', path),
    is_head_office: requireBoolean(o, 'is_head_office', path),
    down_payment: requireNumber(o, 'down_payment', path),
    weekly_rent: requireNumber(o, 'weekly_rent', path),
    rent_prepaid_weeks: requireNumber(o, 'rent_prepaid_weeks', path),
    next_rent_due: optionalString(o, 'next_rent_due', path),
    storage: {
      base_capacity: requireNumber(storage, 'base_capacity', `${path}.storage`),
      current_capacity: requireNumber(storage, 'current_capacity', `${path}.storage`),
      maximum_capacity: requireNumber(storage, 'maximum_capacity', `${path}.storage`),
      used_units: requireNumber(storage, 'used_units', `${path}.storage`),
    },
    employee_capacity: requireNumber(o, 'employee_capacity', path),
    bicycle_capacity: requireNumber(o, 'bicycle_capacity', path),
    vehicle_capacity: requireNumber(o, 'vehicle_capacity', path),
    accepted_package_sizes: requireStringArray(o, 'accepted_package_sizes', path),
    contract_status: requireString(o, 'contract_status', path),
    missed_rent_payments: requireNumber(o, 'missed_rent_payments', path),
  }
}

function parsePackage(value: unknown, path: string): Package {
  const o = requireObject(value, path)
  return {
    id: requireString(o, 'id', path),
    size: requireString(o, 'size', path),
    service_type: requireString(o, 'service_type', path),
    destination_type: requireString(o, 'destination_type', path),
    storage_units: requireNumber(o, 'storage_units', path),
    delivery_capacity_units: requireNumber(o, 'delivery_capacity_units', path),
    base_fee: requireNumber(o, 'base_fee', path),
    received_at: requireString(o, 'received_at', path),
    due_at: requireString(o, 'due_at', path),
    status: requireString(o, 'status', path),
    assigned_employee_id: optionalString(o, 'assigned_employee_id', path),
    delivered_at: optionalString(o, 'delivered_at', path),
    final_revenue: optionalNumber(o, 'final_revenue', path),
  }
}

function parseEmployee(value: unknown, path: string): Employee {
  const o = requireObject(value, path)
  return {
    id: requireString(o, 'id', path),
    name: requireString(o, 'name', path),
    speed_trait: requireString(o, 'speed_trait', path),
    skills: requireStringArray(o, 'skills', path),
    mood: requireString(o, 'mood', path),
    current_delivery_mode: requireString(o, 'current_delivery_mode', path),
    packages_delivered_this_week: requireNumber(o, 'packages_delivered_this_week', path),
    accrued_wages: requireNumber(o, 'accrued_wages', path),
    status: requireString(o, 'status', path),
  }
}

function parseHiring(value: unknown, path: string): HiringState {
  const o = requireObject(value, path)
  return {
    current_employee_count: requireNumber(o, 'current_employee_count', path),
    total_hires_lifetime: requireNumber(o, 'total_hires_lifetime', path),
    next_hiring_fee: requireNumber(o, 'next_hiring_fee', path),
  }
}

function parseFinance(payload: unknown): FinanceStatement {
  const root = requireObject(payload, 'finance')
  const period = requireObject(root.period, 'finance.period')
  const income = requireObject(root.income, 'finance.income')
  const expenses = requireObject(root.expenses, 'finance.expenses')
  const liabilities = requireObject(root.liabilities, 'finance.liabilities')
  return {
    cash_balance: requireNumber(root, 'cash_balance', 'finance'),
    period: {
      from: requireString(period, 'from', 'finance.period'),
      to: requireString(period, 'to', 'finance.period'),
    },
    income: {
      package_revenue: requireNumber(income, 'package_revenue', 'finance.income'),
      trait_bonus: requireNumber(income, 'trait_bonus', 'finance.income'),
      total: requireNumber(income, 'total', 'finance.income'),
    },
    expenses: {
      employee_wages: requireNumber(expenses, 'employee_wages', 'finance.expenses'),
      rent: requireNumber(expenses, 'rent', 'finance.expenses'),
      loan_interest: requireNumber(expenses, 'loan_interest', 'finance.expenses'),
      hiring: requireNumber(expenses, 'hiring', 'finance.expenses'),
      other: requireNumber(expenses, 'other', 'finance.expenses'),
      total: requireNumber(expenses, 'total', 'finance.expenses'),
    },
    net_change: requireNumber(root, 'net_change', 'finance'),
    liabilities: {
      loan_principal: requireNumber(liabilities, 'loan_principal', 'finance.liabilities'),
      accrued_employee_wages: requireNumber(liabilities, 'accrued_employee_wages', 'finance.liabilities'),
      next_rent_amount: requireNumber(liabilities, 'next_rent_amount', 'finance.liabilities'),
      next_interest_estimate: requireNumber(liabilities, 'next_interest_estimate', 'finance.liabilities'),
    },
  }
}

function parseTransaction(value: unknown, path: string): Transaction {
  const o = requireObject(value, path)
  return {
    id: requireString(o, 'id', path),
    game_datetime: requireString(o, 'game_datetime', path),
    category: requireString(o, 'category', path),
    amount: requireNumber(o, 'amount', path),
    description: requireString(o, 'description', path),
    reference_id: optionalString(o, 'reference_id', path),
  }
}

export const api = {
  async getGame(): Promise<GameState> {
    return parseGameState(await request<unknown>('/game'))
  },

  async getClock(): Promise<ClockState> {
    return parseClock(await request<unknown>('/clock'))
  },

  setSpeed(speed: number): Promise<unknown> {
    return request<unknown>('/clock/speed', { method: 'POST', body: JSON.stringify({ speed }) })
  },

  setPaused(paused: boolean): Promise<unknown> {
    return request<unknown>('/clock/pause', { method: 'POST', body: JSON.stringify({ paused }) })
  },

  skipToNextOpening(): Promise<unknown> {
    return request<unknown>('/clock/skip-to-next-opening', { method: 'POST', body: JSON.stringify({}) })
  },

  async getOffices(): Promise<Office[]> {
    const list = toList<unknown>(await request<unknown>('/offices'), ['offices', 'items', 'results'], 'GET /offices')
    return list.map((item, index) => parseOffice(item, `offices[${index}]`))
  },

  selectOffice(officeId: string): Promise<unknown> {
    return request<unknown>('/offices/select', { method: 'POST', body: JSON.stringify({ office_id: officeId }) })
  },

  async getPackages(): Promise<Package[]> {
    const list = toList<unknown>(await request<unknown>('/packages'), ['packages', 'items', 'results'], 'GET /packages')
    return list.map((item, index) => parsePackage(item, `packages[${index}]`))
  },

  async getEmployees(): Promise<{ employees: Employee[]; hiring: HiringState | null }> {
    const payload = await request<unknown>('/employees')
    if (Array.isArray(payload)) {
      return { employees: payload.map((item, index) => parseEmployee(item, `employees[${index}]`)), hiring: null }
    }
    if (payload !== null && typeof payload === 'object') {
      const record = payload as Record<string, unknown>
      const rawList =
        Array.isArray(record.employees) ? record.employees
        : Array.isArray(record.items) ? record.items
        : null
      if (rawList !== null) {
        const hiring =
          record.hiring !== null && record.hiring !== undefined
            ? parseHiring(record.hiring, 'employees.hiring')
            : null
        return {
          employees: rawList.map((item, index) => parseEmployee(item, `employees[${index}]`)),
          hiring,
        }
      }
    }
    throw new ApiError('UNEXPECTED_RESPONSE', `Expected an employee list from GET /employees, got ${describe(payload)}`, 200)
  },

  hireEmployee(): Promise<unknown> {
    return request<unknown>('/employees/hire', { method: 'POST', body: JSON.stringify({}) })
  },

  assignDelivery(employeeId: string, packageCount: number): Promise<unknown> {
    return request<unknown>('/deliveries/assign', {
      method: 'POST',
      body: JSON.stringify({ employee_id: employeeId, package_count: packageCount }),
    })
  },

  async getFinance(): Promise<FinanceStatement> {
    return parseFinance(await request<unknown>('/finance'))
  },

  async getTransactions(): Promise<Transaction[]> {
    const list = toList<unknown>(
      await request<unknown>('/finance/transactions'),
      ['transactions', 'items', 'results'],
      'GET /finance/transactions',
    )
    return list.map((item, index) => parseTransaction(item, `transactions[${index}]`))
  },

  resetGame(): Promise<unknown> {
    return request<unknown>('/debug/reset', { method: 'POST', body: JSON.stringify({}) })
  },
}
