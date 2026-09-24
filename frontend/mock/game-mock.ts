import type { IncomingMessage, ServerResponse } from 'node:http'

const START_MS = Date.UTC(1980, 1, 1, 9, 0, 0)
const GAME_MS_PER_REAL_MS = 120
const WALK_CAPACITY = 10
const WAGE_FOOT = 2
const PACK_MS = 60 * 60 * 1000
const DELIVER_MS = 3 * 60 * 60 * 1000
const OPEN_SLOTS: { dow: number; openMin: number; closeMin: number }[] = [
  { dow: 1, openMin: 9 * 60, closeMin: 17 * 60 },
  { dow: 2, openMin: 9 * 60, closeMin: 17 * 60 },
  { dow: 3, openMin: 9 * 60, closeMin: 17 * 60 },
  { dow: 4, openMin: 9 * 60, closeMin: 17 * 60 },
  { dow: 5, openMin: 9 * 60, closeMin: 17 * 60 },
  { dow: 6, openMin: 10 * 60, closeMin: 13 * 60 },
]

interface Txn {
  id: string
  game_datetime: string
  category: string
  amount: number
  description: string
  reference_id: string | null
}

interface Pkg {
  id: string
  size: 'small' | 'medium'
  service_type: 'normal' | 'express'
  destination_type: 'local'
  storage_units: 1
  delivery_capacity_units: number
  base_fee: number
  received_at: string
  due_at: string
  status: 'stored' | 'assigned' | 'out_for_delivery' | 'delivered'
  assigned_employee_id: string | null
  delivered_at: string | null
  final_revenue: number | null
}

interface Emp {
  id: string
  name: string
  speed_trait: 'snail' | 'chicken' | 'cheetah'
  skills: string[]
  mood: 'happy' | 'neutral' | 'unhappy'
  current_delivery_mode: 'foot'
  packages_delivered_this_week: number
  accrued_wages: number
  status: 'ready' | 'packing' | 'out_for_delivery'
}

interface Run {
  employee_id: string
  package_ids: string[]
  phase: 'packing' | 'out_for_delivery'
  phase_ends_ms: number
}

interface OwnedOffice {
  id: string
  type: 'small' | 'large'
  down_payment: number
  weekly_rent: number
  rent_prepaid_weeks: number
  rent_prepaid_until_ms: number
  next_rent_due_ms: number
  base_capacity: number
  current_capacity: number
  maximum_capacity: number
  employee_capacity: number
  bicycle_capacity: number
  vehicle_capacity: number
  accepted_package_sizes: string[]
  missed_rent_payments: number
  contract_status: 'active' | 'terminated'
}

const OFFERS: {
  id: string
  type: 'small' | 'large'
  is_head_office: boolean
  down_payment: number
  weekly_rent: number
  rent_prepaid_weeks: number
  next_rent_due: string | null
  storage: { base_capacity: number; current_capacity: number; maximum_capacity: number; used_units: number }
  employee_capacity: number
  bicycle_capacity: number
  vehicle_capacity: number
  accepted_package_sizes: string[]
  contract_status: 'available' | 'active' | 'terminated'
  missed_rent_payments: number
}[] = [
  {
    id: 'office-small-01',
    type: 'small',
    is_head_office: false,
    down_payment: 350,
    weekly_rent: 50,
    rent_prepaid_weeks: 4,
    next_rent_due: null,
    storage: { base_capacity: 100, current_capacity: 100, maximum_capacity: 150, used_units: 0 },
    employee_capacity: 5,
    bicycle_capacity: 5,
    vehicle_capacity: 1,
    accepted_package_sizes: ['small', 'medium'],
    contract_status: 'available',
    missed_rent_payments: 0,
  },
  {
    id: 'office-large-01',
    type: 'large',
    is_head_office: false,
    down_payment: 450,
    weekly_rent: 75,
    rent_prepaid_weeks: 4,
    next_rent_due: null,
    storage: { base_capacity: 150, current_capacity: 150, maximum_capacity: 250, used_units: 0 },
    employee_capacity: 7,
    bicycle_capacity: 7,
    vehicle_capacity: 2,
    accepted_package_sizes: ['small', 'medium'],
    contract_status: 'available',
    missed_rent_payments: 0,
  },
]

const HIRE_NAMES = [
  'Bob Snail',
  'Sally Snail',
  'Chuck Chicken',
  'Rita Chicken',
  'Chester Cheetah',
  'Cleo Cheetah',
  'Marty Snail',
  'Clara Chicken',
  'Sam Cheetah',
]

function pad(n: number): string {
  return String(n).padStart(2, '0')
}

function isoFromMs(ms: number): string {
  const d = new Date(ms)
  return `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}T${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}:${pad(d.getUTCSeconds())}`
}

function msFromIso(iso: string): number {
  return Date.parse(`${iso}Z`)
}

function dowIndex(ms: number): number {
  return new Date(ms).getUTCDay()
}

const DOW_NAMES = ['sunday', 'monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday']

function dayKey(ms: number): string {
  return isoFromMs(ms).slice(0, 10)
}

function nextOpeningMs(fromMs: number): number {
  for (let dayOffset = 0; dayOffset < 14; dayOffset++) {
    const dayBase = fromMs - (fromMs % 86_400_000) + dayOffset * 86_400_000
    const slot = OPEN_SLOTS.find((s) => s.dow === dowIndex(dayBase))
    if (!slot) continue
    const openMs = dayBase + slot.openMin * 60_000
    if (openMs > fromMs) return openMs
    if (fromMs < dayBase + slot.closeMin * 60_000) return fromMs
  }
  return fromMs + 86_400_000
}

function slotFor(ms: number) {
  return OPEN_SLOTS.find((s) => s.dow === dowIndex(ms))
}

function isOpenAt(ms: number): boolean {
  const slot = slotFor(ms)
  if (!slot) return false
  const minutes = (ms % 86_400_000) / 60_000
  return minutes >= slot.openMin && minutes < slot.closeMin
}

function closeMsToday(ms: number): number | null {
  const slot = slotFor(ms)
  if (!slot) return null
  const dayBase = ms - (ms % 86_400_000)
  return dayBase + slot.closeMin * 60_000
}

interface MockState {
  epochRealMs: number
  epochGameMs: number
  speed: number
  paused: boolean
  cash: number
  loanPrincipal: number
  trait: string
  office: OwnedOffice | null
  packages: Pkg[]
  employees: Emp[]
  runs: Run[]
  txns: Txn[]
  totalHires: number
  nextPkg: number
  nextEmp: number
  nextTxn: number
  lastGenMs: number
  lastDaySeen: string
  interestDueMs: number
  deliveredToday: number
  deliveredTodayKey: string
}

function createState(): MockState {
  return {
    epochRealMs: Date.now(),
    epochGameMs: START_MS,
    speed: 1,
    paused: false,
    cash: 1000,
    loanPrincipal: 1000,
    trait: 'financial',
    office: null,
    packages: [],
    employees: [],
    runs: [],
    txns: [
      {
        id: 'txn-000001',
        game_datetime: isoFromMs(START_MS),
        category: 'loan_disbursement',
        amount: 1000,
        description: 'Starting loan',
        reference_id: 'loan-1',
      },
    ],
    totalHires: 0,
    nextPkg: 1,
    nextEmp: 1,
    nextTxn: 2,
    lastGenMs: START_MS,
    lastDaySeen: dayKey(START_MS),
    interestDueMs: START_MS + 28 * 86_400_000,
    deliveredToday: 0,
    deliveredTodayKey: dayKey(START_MS),
  }
}

function gameNowMs(state: MockState): number {
  if (state.paused) return state.epochGameMs
  const elapsed = Date.now() - state.epochRealMs
  return state.epochGameMs + elapsed * GAME_MS_PER_REAL_MS * state.speed
}

function foldTime(state: MockState): void {
  const now = gameNowMs(state)
  state.epochGameMs = now
  state.epochRealMs = Date.now()
}

function addTxn(state: MockState, category: string, amount: number, description: string, ref: string | null, atMs: number): void {
  state.txns.push({
    id: `txn-${String(state.nextTxn).padStart(6, '0')}`,
    game_datetime: isoFromMs(atMs),
    category,
    amount,
    description,
    reference_id: ref,
  })
  state.nextTxn += 1
  state.cash = Math.round((state.cash + amount) * 100) / 100
}

function storageUsed(state: MockState): number {
  return state.packages
    .filter((p) => p.status === 'stored' || p.status === 'assigned' || p.status === 'out_for_delivery')
    .reduce((sum, p) => sum + p.storage_units, 0)
}

function spawnPackage(state: MockState, atMs: number): void {
  const office = state.office
  if (!office || office.contract_status !== 'active') return
  const capacity = office.current_capacity
  if (storageUsed(state) >= capacity) return

  const isExpressMonth = atMs >= START_MS + 28 * 86_400_000
  const express = isExpressMonth && Math.random() < 0.03
  const size: 'small' | 'medium' = Math.random() < 0.7 ? 'small' : 'medium'
  const baseFee = express ? (size === 'small' ? 12 : 15) : size === 'small' ? 5 : 7
  const deadline = express ? 2 * 86_400_000 : 5 * 86_400_000

  state.packages.push({
    id: `pkg-${String(state.nextPkg).padStart(6, '0')}`,
    size,
    service_type: express ? 'express' : 'normal',
    destination_type: 'local',
    storage_units: 1,
    delivery_capacity_units: express ? 2 : 1,
    base_fee: baseFee,
    received_at: isoFromMs(atMs),
    due_at: isoFromMs(atMs + deadline),
    status: 'stored',
    assigned_employee_id: null,
    delivered_at: null,
    final_revenue: null,
  })
  state.nextPkg += 1
}

function completeRun(state: MockState, run: Run, atMs: number): void {
  const emp = state.employees.find((e) => e.id === run.employee_id)
  let revenue = 0
  let count = 0

  for (const pkgId of run.package_ids) {
    const pkg = state.packages.find((p) => p.id === pkgId)
    if (!pkg) continue
    const late = pkg.due_at < isoFromMs(atMs)
    const factor = late ? (pkg.service_type === 'express' ? 0.25 : 0.75) : 1
    const paid = Math.round(pkg.base_fee * factor * 100) / 100
    pkg.status = 'delivered'
    pkg.delivered_at = isoFromMs(atMs)
    pkg.final_revenue = paid
    revenue += paid
    count += 1
  }

  revenue = Math.round(revenue * 100) / 100
  if (revenue > 0) {
    addTxn(state, 'package_revenue', revenue, `Delivered ${count} package(s)`, run.employee_id, atMs)
    if (state.trait === 'financial') {
      const bonus = Math.round(revenue * 0.1 * 100) / 100
      addTxn(state, 'financial_trait_bonus', bonus, 'Financial trait +10% revenue', 'trait-financial', atMs)
    }
  }

  const wage = Math.round(count * WAGE_FOOT * 100) / 100
  if (emp) {
    emp.status = 'ready'
    emp.packages_delivered_this_week += count
    emp.accrued_wages = Math.round((emp.accrued_wages + wage) * 100) / 100
  }

  if (dayKey(atMs) !== state.deliveredTodayKey) {
    state.deliveredTodayKey = dayKey(atMs)
    state.deliveredToday = 0
  }
  state.deliveredToday += count
}

function settlePayroll(state: MockState, atMs: number): void {
  const total = state.employees.reduce((sum, e) => sum + e.accrued_wages, 0)
  const rounded = Math.round(total * 100) / 100
  if (rounded > 0) {
    addTxn(state, 'employee_wages', -rounded, 'Tuesday payroll', null, atMs)
    for (const e of state.employees) e.accrued_wages = 0
  }
}

function chargeRent(state: MockState, atMs: number): void {
  const office = state.office
  if (!office || office.contract_status !== 'active') return
  if (atMs < office.rent_prepaid_until_ms) {
    office.next_rent_due_ms = office.rent_prepaid_until_ms
    return
  }
  if (state.cash >= office.weekly_rent) {
    office.missed_rent_payments = 0
    office.next_rent_due_ms += 7 * 86_400_000
    addTxn(state, 'rent', -office.weekly_rent, `${office.type} office weekly rent`, office.id, atMs)
    return
  }
  office.missed_rent_payments += 1
  if (office.missed_rent_payments >= 2) {
    office.contract_status = 'terminated'
    addTxn(state, 'rent_late_fee', 0, 'Office contract terminated after missed rent', office.id, atMs)
    return
  }
  const fee = Math.round(office.weekly_rent * 0.2 * 100) / 100
  office.next_rent_due_ms += 7 * 86_400_000
  addTxn(state, 'rent_late_fee', -fee, 'First missed rent — 20% late fee', office.id, atMs)
}

function chargeInterest(state: MockState, atMs: number): void {
  if (atMs < state.interestDueMs) return
  const interest = Math.round(state.loanPrincipal * 0.05 * 100) / 100
  addTxn(state, 'loan_interest', -interest, 'Four-week loan interest (5%)', 'loan-1', atMs)
  state.interestDueMs += 28 * 86_400_000
}

function processRuns(state: MockState, nowMs: number): void {
  const remaining: Run[] = []
  for (const run of state.runs) {
    if (nowMs < run.phase_ends_ms) {
      remaining.push(run)
      continue
    }
    if (run.phase === 'packing') {
      run.phase = 'out_for_delivery'
      run.phase_ends_ms = run.phase_ends_ms + DELIVER_MS
      for (const id of run.package_ids) {
        const pkg = state.packages.find((p) => p.id === id)
        if (pkg) pkg.status = 'out_for_delivery'
      }
      remaining.push(run)
    } else {
      completeRun(state, run, run.phase_ends_ms)
    }
  }
  state.runs = remaining

  for (const emp of state.employees) {
    const active = state.runs.find((r) => r.employee_id === emp.id)
    if (active) emp.status = active.phase
    else if (emp.status !== 'ready') emp.status = 'ready'
  }
}

function processCalendar(state: MockState, nowMs: number): void {
  const key = dayKey(nowMs)
  if (key === state.lastDaySeen) return

  const prevMs = msFromIso(`${state.lastDaySeen}T12:00:00`)
  if (!Number.isFinite(prevMs)) {
    state.lastDaySeen = key
    return
  }

  const curr = new Date(nowMs)
  const prev = new Date(prevMs)
  const rawDays = Math.round(
    (Date.UTC(curr.getUTCFullYear(), curr.getUTCMonth(), curr.getUTCDate()) - Date.UTC(prev.getUTCFullYear(), prev.getUTCMonth(), prev.getUTCDate())) / 86_400_000,
  )
  if (rawDays <= 0) {
    state.lastDaySeen = key
    return
  }

  const daysBetween = Math.min(rawDays, 30)
  for (let i = 1; i <= daysBetween; i++) {
    const stepMs = prevMs + i * 86_400_000 + 9 * 60 * 60 * 1000
    if (stepMs > nowMs) break
    const dow = dowIndex(stepMs)
    if (dow === 2) settlePayroll(state, stepMs)
    if (dow === 5 && state.office && stepMs >= state.office.next_rent_due_ms) chargeRent(state, stepMs)
    chargeInterest(state, stepMs)
  }

  state.lastDaySeen = key
}

function generatePackages(state: MockState, nowMs: number): void {
  if (!isOpenAt(nowMs)) {
    state.lastGenMs = nowMs
    return
  }
  let guard = 0
  while (nowMs - state.lastGenMs >= 30 * 60_000 && guard < 48) {
    state.lastGenMs += 30 * 60_000
    spawnPackage(state, state.lastGenMs)
    guard += 1
  }
}

function simulate(state: MockState): number {
  const nowMs = gameNowMs(state)
  processRuns(state, nowMs)
  processCalendar(state, nowMs)
  generatePackages(state, nowMs)
  if (state.office && state.office.contract_status !== 'active') {
    foldTime(state)
    state.office = null
    state.paused = true
  }
  return gameNowMs(state)
}

function httpError(status: number, code: string, message: string, details?: Record<string, unknown>) {
  return {
    status,
    body: { error: { code, message, ...(details ? { details } : {}) } },
  }
}

function readJson(req: IncomingMessage): Promise<Record<string, unknown>> {
  return new Promise((resolve, reject) => {
    let raw = ''
    req.on('data', (chunk: Buffer) => {
      raw += chunk.toString('utf8')
      if (raw.length > 1_000_000) reject(new Error('body too large'))
    })
    req.on('end', () => {
      if (!raw) {
        resolve({})
        return
      }
      try {
        const parsed: unknown = JSON.parse(raw)
        if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
          reject(new Error('body must be an object'))
          return
        }
        resolve(parsed as Record<string, unknown>)
      } catch {
        reject(new Error('invalid JSON'))
      }
    })
    req.on('error', reject)
  })
}

function gameResponse(state: MockState, nowMs: number) {
  const office = state.office
  const used = storageUsed(state)
  return {
    game: { status: 'running', game_datetime: isoFromMs(nowMs), speed: state.speed },
    player: { id: 'player-1', cash: Math.round(state.cash * 100) / 100, trait: state.trait },
    office: office
      ? {
          id: office.id,
          storage_used: used,
          storage_capacity: office.current_capacity,
          employee_count: state.employees.length,
          employee_capacity: office.employee_capacity,
        }
      : null,
    operations: {
      stored_packages: state.packages.filter((p) => p.status === 'stored').length,
      out_for_delivery: state.packages.filter((p) => p.status === 'out_for_delivery').length,
      delivered_today: state.deliveredToday,
    },
    finance: {
      accrued_wages: Math.round(state.employees.reduce((s, e) => s + e.accrued_wages, 0) * 100) / 100,
      next_rent: office ? office.weekly_rent : 0,
      loan_principal: state.loanPrincipal,
    },
  }
}

function clockResponse(state: MockState, nowMs: number) {
  const now = nowMs
  let daysPayroll = 0
  {
    const d = new Date(now)
    const currentDow = d.getUTCDay()
    daysPayroll = currentDow <= 2 ? 2 - currentDow : 2 + (7 - currentDow)
    const minutes = (now % 86_400_000) / 60_000
    const slot = slotFor(now)
    if (slot && currentDow === 2 && minutes > 17 * 60) daysPayroll = 7
  }
  let daysRent = 0
  if (state.office) {
    const due = state.office.next_rent_due_ms
    const dueDay = Date.UTC(new Date(due).getUTCFullYear(), new Date(due).getUTCMonth(), new Date(due).getUTCDate())
    const nowDay = Date.UTC(new Date(now).getUTCFullYear(), new Date(now).getUTCMonth(), new Date(now).getUTCDate())
    daysRent = Math.max(0, Math.round((dueDay - nowDay) / 86_400_000))
  }
  return {
    game_datetime: isoFromMs(now),
    day_of_week: DOW_NAMES[dowIndex(now)],
    speed: state.speed,
    paused: state.paused,
    office_open: isOpenAt(now),
    days_until_next_payroll: daysPayroll,
    days_until_next_rent: daysRent,
  }
}

function financeResponse(state: MockState, nowMs: number) {
  const weekIndex = Math.floor((nowMs - START_MS) / (7 * 86_400_000))
  const fromMs = START_MS + weekIndex * 7 * 86_400_000
  const toMs = fromMs + 7 * 86_400_000 - 1000
  const fromIso = isoFromMs(fromMs)
  const toIso = isoFromMs(toMs)
  const inPeriod = state.txns.filter((t) => t.game_datetime >= fromIso && t.game_datetime <= toIso)

  let packageRevenue = 0
  let traitBonus = 0
  let wages = 0
  let rent = 0
  let interest = 0
  let hiring = 0
  let other = 0

  for (const t of inPeriod) {
    if (t.amount <= 0) continue
    if (t.category === 'package_revenue') packageRevenue += t.amount
    else if (t.category === 'financial_trait_bonus') traitBonus += t.amount
  }
  for (const t of inPeriod) {
    if (t.amount >= 0) continue
    const abs = -t.amount
    if (t.category === 'employee_wages') wages += abs
    else if (t.category === 'rent') rent += abs
    else if (t.category === 'loan_interest') interest += abs
    else if (t.category === 'hiring_bonus') hiring += abs
    else other += abs
  }

  const round = (n: number) => Math.round(n * 100) / 100
  const incomeTotal = round(packageRevenue + traitBonus)
  const expenseTotal = round(wages + rent + interest + hiring + other)
  const accrued = round(state.employees.reduce((s, e) => s + e.accrued_wages, 0))

  return {
    cash_balance: round(state.cash),
    period: { from: fromIso, to: toIso },
    income: { package_revenue: round(packageRevenue), trait_bonus: round(traitBonus), total: incomeTotal },
    expenses: {
      employee_wages: round(wages),
      rent: round(rent),
      loan_interest: round(interest),
      hiring: round(hiring),
      other: round(other),
      total: expenseTotal,
    },
    net_change: round(incomeTotal - expenseTotal),
    liabilities: {
      loan_principal: round(state.loanPrincipal),
      accrued_employee_wages: accrued,
      next_rent_amount: state.office ? state.office.weekly_rent : 0,
      next_interest_estimate: round(state.loanPrincipal * 0.05),
    },
  }
}

function officeOffers(state: MockState) {
  return OFFERS.map((offer) => {
    const owned = state.office && state.office.id === offer.id ? state.office : null
    return {
      ...offer,
      is_head_office: Boolean(owned),
      contract_status: owned ? owned.contract_status : 'available',
      next_rent_due: owned ? isoFromMs(owned.next_rent_due_ms) : null,
      storage: {
        ...offer.storage,
        used_units: owned ? storageUsed(state) : 0,
      },
    }
  })
}

async function handle(state: MockState, req: IncomingMessage, res: ServerResponse, url: URL): Promise<boolean> {
  const path = url.pathname.replace(/\/$/, '') || '/'
  const method = req.method ?? 'GET'

  if (!path.startsWith('/api/v1')) return false

  const respond = (status: number, body: unknown) => {
    res.statusCode = status
    res.setHeader('Content-Type', 'application/json')
    res.end(JSON.stringify(body))
  }

  try {
    if (method === 'GET' && path === '/api/v1/game') {
      const now = simulate(state)
      respond(200, gameResponse(state, now))
      return true
    }
    if (method === 'GET' && path === '/api/v1/clock') {
      const now = simulate(state)
      respond(200, clockResponse(state, now))
      return true
    }
    if (method === 'POST' && path === '/api/v1/clock/speed') {
      const body = await readJson(req)
      const speed = body.speed
      if (speed !== 1 && speed !== 2 && speed !== 3) {
        const err = httpError(400, 'INVALID_SPEED', 'speed must be 1, 2 or 3')
        respond(err.status, err.body)
        return true
      }
      foldTime(state)
      state.speed = speed
      state.paused = false
      const now = simulate(state)
      respond(200, clockResponse(state, now))
      return true
    }
    if (method === 'POST' && path === '/api/v1/clock/pause') {
      const body = await readJson(req)
      if (typeof body.paused !== 'boolean') {
        const err = httpError(400, 'INVALID_PAUSE', 'paused must be a boolean')
        respond(err.status, err.body)
        return true
      }
      foldTime(state)
      state.paused = body.paused
      const now = simulate(state)
      respond(200, clockResponse(state, now))
      return true
    }
    if (method === 'POST' && path === '/api/v1/clock/skip-to-next-opening') {
      await readJson(req)
      foldTime(state)
      const target = nextOpeningMs(state.epochGameMs + 60_000)
      state.epochGameMs = target
      state.epochRealMs = Date.now()
      const now = simulate(state)
      respond(200, clockResponse(state, now))
      return true
    }
    if (method === 'GET' && path === '/api/v1/offices') {
      simulate(state)
      respond(200, { offices: officeOffers(state) })
      return true
    }
    if (method === 'POST' && path === '/api/v1/offices/select') {
      const body = await readJson(req)
      const officeId = body.office_id
      if (typeof officeId !== 'string') {
        const err = httpError(400, 'INVALID_OFFICE_ID', 'office_id must be a string')
        respond(err.status, err.body)
        return true
      }
      if (state.office) {
        const err = httpError(409, 'OFFICE_ALREADY_SELECTED', 'A head office is already selected.')
        respond(err.status, err.body)
        return true
      }
      const offer = OFFERS.find((o) => o.id === officeId)
      if (!offer) {
        const err = httpError(404, 'OFFICE_NOT_FOUND', `Unknown office: ${officeId}`)
        respond(err.status, err.body)
        return true
      }
      const now0 = simulate(state)
      if (state.cash < offer.down_payment) {
        const err = httpError(400, 'INSUFFICIENT_FUNDS', 'Not enough cash for the down payment.', {
          cash: state.cash,
          required: offer.down_payment,
        })
        respond(err.status, err.body)
        return true
      }
      addTxn(
        state,
        'office_down_payment',
        -offer.down_payment,
        `${offer.type === 'small' ? 'Small' : 'Large'} head office contract`,
        offer.id,
        now0,
      )
      state.office = {
        id: offer.id,
        type: offer.type,
        down_payment: offer.down_payment,
        weekly_rent: offer.weekly_rent,
        rent_prepaid_weeks: offer.rent_prepaid_weeks,
        rent_prepaid_until_ms: now0 + offer.rent_prepaid_weeks * 7 * 86_400_000,
        next_rent_due_ms: now0 + offer.rent_prepaid_weeks * 7 * 86_400_000,
        base_capacity: offer.storage.base_capacity,
        current_capacity: offer.storage.current_capacity,
        maximum_capacity: offer.storage.maximum_capacity,
        employee_capacity: offer.employee_capacity,
        bicycle_capacity: offer.bicycle_capacity,
        vehicle_capacity: offer.vehicle_capacity,
        accepted_package_sizes: [...offer.accepted_package_sizes],
        missed_rent_payments: 0,
        contract_status: 'active',
      }
      for (let i = 0; i < 6; i++) spawnPackage(state, now0)
      state.lastGenMs = now0
      respond(200, gameResponse(state, now0))
      return true
    }
    if (method === 'GET' && path === '/api/v1/packages') {
      const now = simulate(state)
      void now
      respond(200, { packages: state.packages })
      return true
    }
    if (method === 'GET' && path === '/api/v1/employees') {
      const now = simulate(state)
      void now
      respond(200, {
        employees: state.employees,
        hiring: {
          current_employee_count: state.employees.length,
          total_hires_lifetime: state.totalHires,
          next_hiring_fee: (state.totalHires + 1) * 50,
        },
      })
      return true
    }
    if (method === 'POST' && path === '/api/v1/employees/hire') {
      await readJson(req)
      const now = simulate(state)
      if (!state.office || state.office.contract_status !== 'active') {
        const err = httpError(409, 'NO_OFFICE', 'Select a head office before hiring.')
        respond(err.status, err.body)
        return true
      }
      if (state.employees.length >= state.office.employee_capacity) {
        const err = httpError(409, 'OFFICE_FULL', 'Employee capacity reached.', {
          employee_count: state.employees.length,
          employee_capacity: state.office.employee_capacity,
        })
        respond(err.status, err.body)
        return true
      }
      const fee = (state.totalHires + 1) * 50
      if (state.cash < fee) {
        const err = httpError(400, 'INSUFFICIENT_FUNDS', 'Not enough cash for the hiring fee.', {
          cash: state.cash,
          required: fee,
        })
        respond(err.status, err.body)
        return true
      }
      addTxn(state, 'hiring_bonus', -fee, `Hiring fee #${state.totalHires + 1}`, null, now)
      state.totalHires += 1
      const traitCycle: Emp['speed_trait'][] = ['snail', 'chicken', 'cheetah']
      const emp: Emp = {
        id: `emp-${String(state.nextEmp).padStart(4, '0')}`,
        name: HIRE_NAMES[(state.nextEmp - 1) % HIRE_NAMES.length],
        speed_trait: traitCycle[(state.totalHires - 1) % 3],
        skills: [],
        mood: 'neutral',
        current_delivery_mode: 'foot',
        packages_delivered_this_week: 0,
        accrued_wages: 0,
        status: 'ready',
      }
      state.nextEmp += 1
      state.employees.push(emp)
      respond(200, { employee: emp, hiring: {
        current_employee_count: state.employees.length,
        total_hires_lifetime: state.totalHires,
        next_hiring_fee: (state.totalHires + 1) * 50,
      } })
      return true
    }
    if (method === 'POST' && path === '/api/v1/deliveries/assign') {
      const body = await readJson(req)
      const employeeId = body.employee_id
      const count = body.package_count
      if (typeof employeeId !== 'string') {
        const err = httpError(400, 'INVALID_EMPLOYEE_ID', 'employee_id must be a string')
        respond(err.status, err.body)
        return true
      }
      if (typeof count !== 'number' || !Number.isInteger(count) || count < 1) {
        const err = httpError(400, 'INVALID_PACKAGE_COUNT', 'package_count must be a positive integer')
        respond(err.status, err.body)
        return true
      }
      const now = simulate(state)
      const emp = state.employees.find((e) => e.id === employeeId)
      if (!emp) {
        const err = httpError(404, 'EMPLOYEE_NOT_FOUND', `Unknown employee: ${employeeId}`)
        respond(err.status, err.body)
        return true
      }
      if (emp.status !== 'ready') {
        const err = httpError(409, 'EMPLOYEE_BUSY', 'Employee is not available for a new assignment.', {
          employee_id: emp.id,
          status: emp.status,
        })
        respond(err.status, err.body)
        return true
      }
      const stored = state.packages
        .filter((p) => p.status === 'stored')
        .sort((a, b) => {
          if (a.service_type !== b.service_type) return a.service_type === 'express' ? -1 : 1
          return a.due_at.localeCompare(b.due_at)
        })
      if (stored.length === 0) {
        const err = httpError(409, 'NO_STORED_PACKAGES', 'No stored packages available for assignment.')
        respond(err.status, err.body)
        return true
      }
      const batch = stored.slice(0, count)
      const units = batch.reduce((sum, p) => sum + p.delivery_capacity_units, 0)
      if (batch.length < count || units > WALK_CAPACITY) {
        const err = httpError(
          400,
          'INSUFFICIENT_DELIVERY_CAPACITY',
          'Employee does not have enough capacity for this assignment.',
          {
            employee_id: emp.id,
            available_capacity: WALK_CAPACITY,
            requested_capacity: units,
            stored_available: stored.length,
          },
        )
        respond(err.status, err.body)
        return true
      }
      const close = closeMsToday(now)
      if (close === null || now + PACK_MS + DELIVER_MS > close) {
        const err = httpError(
          409,
          'CYCLE_WOULD_NOT_FINISH',
          'A full walking cycle (1h packing + 3h delivery) must finish before closing time.',
          { requested_start: isoFromMs(now), closing_time: close !== null ? isoFromMs(close) : null },
        )
        respond(err.status, err.body)
        return true
      }
      for (const p of batch) {
        p.status = 'assigned'
        p.assigned_employee_id = emp.id
      }
      emp.status = 'packing'
      state.runs.push({
        employee_id: emp.id,
        package_ids: batch.map((p) => p.id),
        phase: 'packing',
        phase_ends_ms: now + PACK_MS,
      })
      respond(200, { employee: emp, assigned: batch.map((p) => p.id) })
      return true
    }
    if (method === 'GET' && path === '/api/v1/finance') {
      const now = simulate(state)
      respond(200, financeResponse(state, now))
      return true
    }
    if (method === 'GET' && path === '/api/v1/finance/transactions') {
      simulate(state)
      respond(200, { transactions: [...state.txns].reverse() })
      return true
    }

    if (method === 'POST' && path === '/api/v1/debug/reset') {
      await readJson(req)
      Object.assign(state, createState())
      const now = simulate(state)
      respond(200, gameResponse(state, now))
      return true
    }

    const err = httpError(404, 'NOT_FOUND', `No mock handler for ${method} ${path}`)
    respond(err.status, err.body)
    return true
  } catch (e) {
    const message = e instanceof Error ? e.message : String(e)
    const err = httpError(400, 'INVALID_REQUEST', message)
    respond(err.status, err.body)
    return true
  }
}

export function createMockApiMiddleware() {
  const state = createState()
  return (req: IncomingMessage, res: ServerResponse, next: () => void) => {
    const url = new URL(req.url ?? '/', 'http://localhost')
    if (!url.pathname.startsWith('/api/v1')) {
      next()
      return
    }
    void handle(state, req, res, url).catch(() => {
      if (!res.headersSent) {
        res.statusCode = 500
        res.setHeader('Content-Type', 'application/json')
        res.end(JSON.stringify({ error: { code: 'MOCK_INTERNAL', message: 'Mock server failure' } }))
      }
    })
  }
}
