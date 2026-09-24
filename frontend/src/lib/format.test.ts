import { describe, expect, it } from 'vitest'
import { capitalize, formatGameDate, formatGameDateTime, formatMoney, formatStatus } from './format'

describe('formatMoney', () => {
  it('formats positive pounds with two decimals', () => {
    expect(formatMoney(1000)).toBe('£1,000.00')
    expect(formatMoney(12.5)).toBe('£12.50')
  })

  it('formats negative amounts with a leading minus', () => {
    expect(formatMoney(-350)).toBe('-£350.00')
  })

  it('formats zero', () => {
    expect(formatMoney(0)).toBe('£0.00')
  })
})

describe('formatGameDateTime', () => {
  it('renders date and HH:MM from a full ISO timestamp', () => {
    expect(formatGameDateTime('1980-02-01T09:30:00.000Z')).toBe('1 Feb 1980, 09:30')
  })

  it('renders date only when there is no time part', () => {
    expect(formatGameDateTime('1980-12-25')).toBe('25 Dec 1980')
  })

  it('falls back to the raw date when parsing fails', () => {
    expect(formatGameDateTime('not-a-date')).toBe('not-a-date')
  })
})

describe('formatGameDate', () => {
  it('returns only the date portion', () => {
    expect(formatGameDate('1980-02-01T09:30:00.000Z')).toBe('1 Feb 1980')
  })
})

describe('capitalize', () => {
  it('uppercases the first character', () => {
    expect(capitalize('monday')).toBe('Monday')
  })

  it('returns empty string unchanged', () => {
    expect(capitalize('')).toBe('')
  })
})

describe('formatStatus', () => {
  it('converts snake_case to Title Case', () => {
    expect(formatStatus('out_for_delivery')).toBe('Out For Delivery')
    expect(formatStatus('stored')).toBe('Stored')
  })
})
