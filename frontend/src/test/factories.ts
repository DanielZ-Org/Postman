import { vi } from 'vitest'
import type {
  ClockState,
  Employee,
  FinanceStatement,
  GameState,
  HiringState,
  Office,
  Package,
  Transaction,
} from '../api/types'
import type { ErrorEntry, GameApi } from '../hooks/useGame'

export function makeClock(overrides: Partial<ClockState> = {}): ClockState {
  return {
    game_datetime: '1980-02-01T09:00:00.000Z',
    day_of_week: 'saturday',
    speed: 1,
    paused: false,
    office_open: true,
    days_until_next_payroll: 4,
    days_until_next_rent: 18,
    ...overrides,
  }
}

export function makeOffice(overrides: Partial<Office> = {}): Office {
  return {
    id: 'office-small-01',
    type: 'small',
    is_head_office: true,
    down_payment: 350,
    weekly_rent: 50,
    rent_prepaid_weeks: 4,
    next_rent_due: '1980-02-29T00:00:00.000Z',
    storage: {
      base_capacity: 100,
      current_capacity: 100,
      maximum_capacity: 150,
      used_units: 6,
    },
    employee_capacity: 5,
    bicycle_capacity: 5,
    vehicle_capacity: 1,
    accepted_package_sizes: ['small', 'medium'],
    contract_status: 'active',
    missed_rent_payments: 0,
    ...overrides,
  }
}

export function makeGameStateOffice(overrides: Partial<GameState['office']> = {}): NonNullable<GameState['office']> {
  return {
    id: 'office-small-01',
    storage_used: 6,
    storage_capacity: 100,
    employee_count: 1,
    employee_capacity: 5,
    ...overrides,
  }
}

export function makeGameState(overrides: Partial<GameState> = {}): GameState {
  const office = overrides.office === undefined ? makeGameStateOffice() : overrides.office
  return {
    game: {
      status: 'running',
      game_datetime: '1980-02-01T09:00:00.000Z',
      speed: 1,
      ...overrides.game,
    },
    player: {
      id: 'player-1',
      cash: 1000,
      trait: 'financial',
      ...overrides.player,
    },
    office,
    operations: {
      stored_packages: 6,
      out_for_delivery: 0,
      delivered_today: 0,
      ...overrides.operations,
    },
    finance: {
      accrued_wages: 0,
      next_rent: 50,
      loan_principal: 0,
      ...overrides.finance,
    },
  }
}

export function makePackage(overrides: Partial<Package> = {}): Package {
  return {
    id: 'pkg-001',
    size: 'small',
    service_type: 'normal',
    destination_type: 'local',
    storage_units: 1,
    delivery_capacity_units: 1,
    base_fee: 8,
    received_at: '1980-02-01T09:05:00.000Z',
    due_at: '1980-02-01T17:00:00.000Z',
    status: 'stored',
    assigned_employee_id: null,
    delivered_at: null,
    final_revenue: null,
    ...overrides,
  }
}

export function makeEmployee(overrides: Partial<Employee> = {}): Employee {
  return {
    id: 'emp-0001',
    name: 'Test Courier',
    speed_trait: 'snail',
    skills: [],
    mood: 'neutral',
    current_delivery_mode: 'foot',
    packages_delivered_this_week: 0,
    accrued_wages: 0,
    status: 'ready',
    ...overrides,
  }
}

export function makeHiring(overrides: Partial<HiringState> = {}): HiringState {
  return {
    current_employee_count: 1,
    total_hires_lifetime: 1,
    next_hiring_fee: 100,
    ...overrides,
  }
}

export function makeFinance(overrides: Partial<FinanceStatement> = {}): FinanceStatement {
  const base: FinanceStatement = {
    cash_balance: 1000,
    period: {
      from: '1980-02-01T00:00:00.000Z',
      to: '1980-02-28T23:59:59.999Z',
    },
    income: {
      package_revenue: 0,
      trait_bonus: 0,
      total: 0,
    },
    expenses: {
      employee_wages: 0,
      rent: 0,
      loan_interest: 0,
      hiring: 0,
      other: 0,
      total: 0,
    },
    net_change: 0,
    liabilities: {
      loan_principal: 0,
      accrued_employee_wages: 0,
      next_rent_amount: 50,
      next_interest_estimate: 0,
    },
  }
  return {
    ...base,
    ...overrides,
    period: overrides.period ?? base.period,
    income: overrides.income ?? base.income,
    expenses: overrides.expenses ?? base.expenses,
    liabilities: overrides.liabilities ?? base.liabilities,
  }
}

export function makeTransaction(overrides: Partial<Transaction> = {}): Transaction {
  return {
    id: 'txn-000001',
    game_datetime: '1980-02-01T09:00:00.000Z',
    category: 'office_down_payment',
    amount: -350,
    description: 'Office down payment',
    reference_id: 'office-small-01',
    ...overrides,
  }
}

function mockFn<T>(returns = false): T {
  return vi.fn(async () => (returns ? true : undefined)) as unknown as T
}

export function makeGameApi(overrides: Partial<GameApi> = {}): GameApi {
  return {
    clock: makeClock(),
    state: makeGameState(),
    offices: [
      makeOffice({ id: 'office-small-01', is_head_office: false, contract_status: 'available' }),
      makeOffice({
        id: 'office-large-01',
        type: 'large',
        down_payment: 450,
        weekly_rent: 75,
        is_head_office: false,
        contract_status: 'available',
        employee_capacity: 7,
        storage: {
          base_capacity: 150,
          current_capacity: 150,
          maximum_capacity: 250,
          used_units: 0,
        },
      }),
    ],
    packages: [
      makePackage({ id: 'pkg-001' }),
      makePackage({ id: 'pkg-002', status: 'assigned' }),
      makePackage({ id: 'pkg-003', status: 'out_for_delivery', assigned_employee_id: 'emp-0001' }),
      makePackage({
        id: 'pkg-004',
        status: 'delivered',
        final_revenue: 12,
        delivered_at: '1980-02-01T14:00:00.000Z',
      }),
    ],
    employees: [makeEmployee()],
    hiring: makeHiring(),
    finance: makeFinance(),
    transactions: [makeTransaction()],
    error: null,
    loaded: true,
    dismissError: mockFn<GameApi['dismissError']>(),
    refresh: mockFn<GameApi['refresh']>(),
    setSpeed: mockFn<GameApi['setSpeed']>(),
    setPaused: mockFn<GameApi['setPaused']>(),
    togglePause: mockFn<GameApi['togglePause']>(),
    skipToNextOpening: mockFn<GameApi['skipToNextOpening']>(),
    selectOffice: mockFn<GameApi['selectOffice']>(true),
    hireEmployee: mockFn<GameApi['hireEmployee']>(true),
    assignDelivery: mockFn<GameApi['assignDelivery']>(true),
    resetGame: mockFn<GameApi['resetGame']>(true),
    ...overrides,
  }
}

export function makeErrorEntry(code = 'TEST_ERROR', message = 'Something went wrong'): ErrorEntry {
  return {
    err: {
      name: 'ApiError',
      message,
      code,
      status: 400,
      details: undefined,
      stack: undefined,
      cause: undefined,
    } as ErrorEntry['err'],
    sticky: true,
  }
}
