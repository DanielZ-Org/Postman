import { useState } from 'react'
import type { GameApi } from '../hooks/useGame'
import type { FinanceStatement } from '../api/types'
import { formatGameDateTime, formatMoney } from '../lib/format'

type RowTone = 'positive' | 'negative'

// formatDelta renders the signed week-over-week difference: "+£5.00", "-£5.00" or
// an em dash when nothing changed. Direction is spelled out in the block heading, so
// the sign stays literal for both income and expenses.
function formatDelta(delta: number): string {
  if (delta > 0) return `+${formatMoney(delta)}`
  if (delta < 0) return formatMoney(delta)
  return '—'
}

function Row({
  label,
  value,
  tone,
  share,
  delta,
}: {
  label: string
  value: number
  tone?: RowTone
  share?: number
  delta?: number
}) {
  return (
    <div className="finance-row">
      <span>{label}</span>
      <span className="finance-row-values">
        <span className={`mono ${tone ? `is-${tone}` : ''}`}>{formatMoney(value)}</span>
        {share !== undefined && <span className="finance-share muted">{share}%</span>}
        {delta !== undefined && <span className="finance-delta muted">{formatDelta(delta)}</span>}
      </span>
    </div>
  )
}

// ObligationRow shows an upcoming liability with its calendar countdown (issue #27):
// "£75.00 · in 3 days". The countdown disappears until the clock has loaded.
function ObligationRow({ label, amount, days }: { label: string; amount: number; days?: number }) {
  const when = days === undefined ? null : days <= 0 ? 'due today' : days === 1 ? 'in 1 day' : `in ${days} days`
  return (
    <div className="finance-row">
      <span>{label}</span>
      <span className="mono">
        {formatMoney(amount)}
        {when ? ` · ${when}` : ''}
      </span>
    </div>
  )
}

// runwayText estimates how many weeks the cash lasts if last week's net burn repeats.
// A non-negative last week means no burn to extrapolate from (issue #27).
function runwayText(cash: number, lastNet: number): string {
  if (lastNet >= 0) return 'covered — no burn last week'
  const burn = -lastNet
  const weeks = Math.max(0, Math.floor(cash / burn))
  return `~${weeks} week${weeks === 1 ? '' : 's'} at last week's pace`
}

// expenseShare is the category's percentage of this week's total spending. Zero-value
// categories and an expense-free week render no share (issue #27).
function expenseShare(amount: number, total: number): number | undefined {
  if (total <= 0 || amount <= 0) return undefined
  return Math.round((amount / total) * 100)
}

function hasLastWeekActivity(prev: FinanceStatement['previous_period']): boolean {
  return prev.income.total !== 0 || prev.expenses.total !== 0 || prev.net_change !== 0
}

// RepayForm drives POST /finance/repay (SPEC 11.1): integer pence with
// 0 < amount <= min(cash, principal). The button stays disabled until the input is a
// valid amount; a successful repayment refreshes every panel through the game hook.
function RepayForm({ game }: { game: GameApi }) {
  const [amountStr, setAmountStr] = useState('')
  const [submitting, setSubmitting] = useState(false)

  const principal = game.finance?.liabilities.loan_principal ?? 0
  const cash = game.state?.player.cash ?? 0
  const available = Math.max(0, Math.min(cash, principal))
  const amount = Number(amountStr)
  const valid = amountStr !== '' && Number.isInteger(amount) && amount > 0 && amount <= available

  if (principal <= 0) {
    return <p className="empty-state">No outstanding loan.</p>
  }

  const handleRepay = () => {
    if (!valid || submitting) return
    setSubmitting(true)
    void game
      .repayLoan(amount)
      .then((ok) => {
        if (ok) setAmountStr('')
      })
      .finally(() => setSubmitting(false))
  }

  return (
    <div className="assign-form">
      <label className="field">
        <span className="field-label">
          Amount
          <span className="field-hint">integer pence, up to {formatMoney(available)} available</span>
        </span>
        <input
          type="number"
          min={1}
          max={available}
          value={amountStr}
          onChange={(event) => setAmountStr(event.target.value)}
          aria-label="Repayment amount in pence"
        />
      </label>
      <button
        type="button"
        className="btn btn-primary"
        onClick={handleRepay}
        disabled={!valid || submitting}
      >
        {submitting ? 'Repaying.' : 'Repay loan'}
      </button>
    </div>
  )
}

export function FinancePanel({ game }: { game: GameApi }) {
  const finance = game.finance
  const prev = finance?.previous_period
  const clock = game.clock
  const cash = game.state?.player.cash ?? 0

  return (
    <div className="panel-grid">
      <section className="panel">
        <div className="panel-head">
          <h2>Finance statement</h2>
          {finance && (
            <span className="muted mono">
              {formatGameDateTime(finance.period.from)} — {formatGameDateTime(finance.period.to)}
            </span>
          )}
        </div>
        {!finance || !prev ? (
          <p className="empty-state">No finance statement yet.</p>
        ) : (
          <>
            <div className="finance-block">
              <h3>This week vs last week</h3>
              <Row
                label="Week income"
                value={finance.income.total}
                tone="positive"
                delta={finance.income.total - prev.income.total}
              />
              <Row
                label="Week expenses"
                value={finance.expenses.total}
                delta={finance.expenses.total - prev.expenses.total}
              />
              <Row
                label="Week net"
                value={finance.net_change}
                tone={finance.net_change >= 0 ? 'positive' : 'negative'}
                delta={finance.net_change - prev.net_change}
              />
              {!hasLastWeekActivity(prev) && (
                <p className="empty-state">No activity recorded last week.</p>
              )}
            </div>
            <div className="finance-block">
              <h3>Income</h3>
              <Row label="Package revenue" value={finance.income.package_revenue} tone="positive" />
              <Row label="Trait bonus" value={finance.income.trait_bonus} tone="positive" />
              <Row label="Total income" value={finance.income.total} tone="positive" />
            </div>
            <div className="finance-block">
              <h3>Expenses</h3>
              <Row
                label="Employee wages"
                value={finance.expenses.employee_wages}
                share={expenseShare(finance.expenses.employee_wages, finance.expenses.total)}
              />
              <Row label="Rent" value={finance.expenses.rent} share={expenseShare(finance.expenses.rent, finance.expenses.total)} />
              <Row
                label="Loan interest"
                value={finance.expenses.loan_interest}
                share={expenseShare(finance.expenses.loan_interest, finance.expenses.total)}
              />
              <Row label="Hiring" value={finance.expenses.hiring} share={expenseShare(finance.expenses.hiring, finance.expenses.total)} />
              <Row
                label="Vehicle fuel"
                value={finance.expenses.vehicle_fuel}
                share={expenseShare(finance.expenses.vehicle_fuel, finance.expenses.total)}
              />
              <Row
                label="Vehicle maintenance"
                value={finance.expenses.vehicle_maintenance}
                share={expenseShare(finance.expenses.vehicle_maintenance, finance.expenses.total)}
              />
              <Row label="Other" value={finance.expenses.other} share={expenseShare(finance.expenses.other, finance.expenses.total)} />
              <Row label="Total expenses" value={finance.expenses.total} />
            </div>
            <div className="finance-block finance-net">
              <Row label="Net change" value={finance.net_change} tone={finance.net_change >= 0 ? 'positive' : 'negative'} />
            </div>
            <div className="finance-block">
              <h3>Liabilities</h3>
              <Row label="Loan principal" value={finance.liabilities.loan_principal} />
              <Row label="Accrued employee wages" value={finance.liabilities.accrued_employee_wages} />
              <Row label="Next rent amount" value={finance.liabilities.next_rent_amount} />
              <Row label="Next interest estimate" value={finance.liabilities.next_interest_estimate} />
            </div>
            <div className="finance-block">
              <h3>Repay loan</h3>
              <RepayForm game={game} />
            </div>
          </>
        )}
      </section>

      <section className="panel">
        <div className="panel-head">
          <h2>Cash balance</h2>
          <span className={`cash-value big ${game.state && game.state.player.cash < 0 ? 'is-negative' : ''}`}>
            {game.state ? formatMoney(game.state.player.cash) : '—'}
          </span>
        </div>
        {finance && prev && (
          <>
            <div className="finance-block">
              <h3>Cash runway</h3>
              <Row
                label="This week net"
                value={finance.net_change}
                tone={finance.net_change >= 0 ? 'positive' : 'negative'}
              />
              <Row
                label="Last week net"
                value={prev.net_change}
                tone={prev.net_change >= 0 ? 'positive' : 'negative'}
              />
              <div className="finance-row">
                <span>Runway</span>
                <span className="mono">{runwayText(cash, prev.net_change)}</span>
              </div>
            </div>
            <div className="finance-block">
              <h3>Next obligations</h3>
              {finance.liabilities.next_rent_amount > 0 && (
                <ObligationRow
                  label="Rent"
                  amount={finance.liabilities.next_rent_amount}
                  days={clock?.days_until_next_rent}
                />
              )}
              <ObligationRow
                label="Payroll (accrued)"
                amount={finance.liabilities.accrued_employee_wages}
                days={clock?.days_until_next_payroll}
              />
              {finance.liabilities.next_interest_estimate > 0 && (
                <ObligationRow
                  label="Loan interest"
                  amount={finance.liabilities.next_interest_estimate}
                  days={clock?.days_until_next_interest}
                />
              )}
            </div>
          </>
        )}
      </section>

      <section className="panel panel-wide">
        <h2>Transactions</h2>
        {game.transactions.length === 0 ? (
          <p className="empty-state">No transactions recorded yet.</p>
        ) : (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Time</th>
                  <th>Category</th>
                  <th>Amount</th>
                  <th>Description</th>
                  <th>Reference</th>
                </tr>
              </thead>
              <tbody>
                {game.transactions.map((txn) => (
                  <tr key={txn.id}>
                    <td className="mono">{formatGameDateTime(txn.game_datetime)}</td>
                    <td>{txn.category}</td>
                    <td className={`mono ${txn.amount >= 0 ? 'is-positive' : 'is-negative'}`}>
                      {formatMoney(txn.amount)}
                    </td>
                    <td>{txn.description}</td>
                    <td className="mono">{txn.reference_id ?? '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  )
}
