export type PackageSize = 'small' | 'medium' | 'large'
export type ServiceType = 'normal' | 'express'
export type DestinationType = 'local' | 'far'
export type PackageStatus = 'stored' | 'assigned' | 'out_for_delivery' | 'delivered'
export type EmployeeStatus = 'ready' | 'packing' | 'out_for_delivery'
export type SpeedTrait = 'snail' | 'chicken' | 'cheetah'
export type Mood = 'happy' | 'neutral' | 'unhappy'
export type DeliveryMode = 'foot' | 'bicycle' | 'car'
export type PlayerTrait = 'financial' | 'storage' | 'logistics'
export type OfficeType = 'small' | 'large'
export type ClockSpeed = 1 | 2 | 3

export interface ClockState {
  game_datetime: string
  day_of_week: string
  speed: ClockSpeed | number
  paused: boolean
  office_open: boolean
  days_until_next_payroll: number
  days_until_next_rent: number
}

export interface Player {
  id: string
  name: string
  logo_id?: string | null
  avatar_id?: string | null
  trait: PlayerTrait | string
  cash: number
  loan_principal: number
  head_office_id?: string | null
}

export interface OfficeStorage {
  base_capacity: number
  current_capacity: number
  maximum_capacity: number
  used_units: number
}

export interface Office {
  id: string
  type: OfficeType | string
  is_head_office: boolean
  down_payment: number
  weekly_rent: number
  rent_prepaid_weeks: number
  next_rent_due?: string | null
  storage: OfficeStorage
  employee_capacity: number
  bicycle_capacity: number
  vehicle_capacity: number
  accepted_package_sizes: PackageSize[] | string[]
  contract_status: string
  missed_rent_payments: number
}

export interface Package {
  id: string
  size: PackageSize | string
  service_type: ServiceType | string
  destination_type: DestinationType | string
  storage_units: number
  delivery_capacity_units: number
  base_fee: number
  received_at: string
  due_at: string
  status: PackageStatus | string
  assigned_employee_id: string | null
  delivered_at: string | null
  final_revenue: number | null
}

export interface Employee {
  id: string
  name: string
  speed_trait: SpeedTrait | string
  skills: string[]
  mood: Mood | string
  current_delivery_mode: DeliveryMode | string
  packages_delivered_this_week: number
  accrued_wages: number
  status: EmployeeStatus | string
}

export interface HiringState {
  current_employee_count: number
  total_hires_lifetime: number
  next_hiring_fee: number
}

export interface FinancePeriod {
  from: string
  to: string
}

export interface FinanceStatement {
  cash_balance: number
  period: FinancePeriod
  income: {
    package_revenue: number
    trait_bonus: number
    total: number
  }
  expenses: {
    employee_wages: number
    rent: number
    loan_interest: number
    hiring: number
    other: number
    total: number
  }
  net_change: number
  liabilities: {
    loan_principal: number
    accrued_employee_wages: number
    next_rent_amount: number
    next_interest_estimate: number
  }
}

export interface Transaction {
  id: string
  game_datetime: string
  category: string
  amount: number
  description: string
  reference_id?: string | null
}

export interface GameStateOffice {
  id: string
  storage_used: number
  storage_capacity: number
  employee_count: number
  employee_capacity: number
}

export interface GameState {
  game: {
    status: string
    game_datetime: string
    speed: ClockSpeed | number
  }
  player: {
    id: string
    cash: number
    trait: PlayerTrait | string
  }
  office: GameStateOffice | null
  operations: {
    stored_packages: number
    out_for_delivery: number
    delivered_today: number
  }
  finance: {
    accrued_wages: number
    next_rent: number
    loan_principal: number
  }
}
