const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

// Monetary wire values are integer pence (SPEC 4.3); display is pounds with two
// decimals.
export function formatMoney(pence: number): string {
  const sign = pence < 0 ? '-' : ''
  const pounds = Math.abs(pence) / 100
  return `${sign}£${pounds.toLocaleString('en-GB', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`
}

export function formatGameDateTime(iso: string): string {
  const tIndex = iso.indexOf('T')
  const datePart = tIndex >= 0 ? iso.slice(0, tIndex) : iso
  const timePart = tIndex >= 0 ? iso.slice(tIndex + 1) : ''
  const [rawYear, rawMonth, rawDay] = datePart.split('-')
  const year = Number(rawYear)
  const monthIndex = Number(rawMonth) - 1
  const day = Number(rawDay)
  const month = MONTHS[monthIndex] ?? rawMonth
  const dateLabel =
    Number.isFinite(year) && Number.isFinite(day) && month
      ? `${day} ${month} ${year}`
      : datePart
  const timeLabel = timePart.length >= 5 ? timePart.slice(0, 5) : timePart
  return timeLabel ? `${dateLabel}, ${timeLabel}` : dateLabel
}

export function formatGameDate(iso: string): string {
  return formatGameDateTime(iso).split(',')[0]
}

export function capitalize(value: string): string {
  if (value.length === 0) return value
  return value[0].toUpperCase() + value.slice(1)
}

export function formatStatus(value: string): string {
  return value.replace(/_/g, ' ').replace(/\b\w/g, (char) => char.toUpperCase())
}

export function formatCategory(value: string): string {
  return formatStatus(value)
}
