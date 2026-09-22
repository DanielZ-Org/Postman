import type { GameApi } from '../hooks/useGame'
import { formatGameDateTime, formatMoney } from '../lib/format'

function Row({ label, value, tone }: { label: string; value: number; tone?: 'positive' | 'negative' }) {
  return (
    <div className="finance-row">
      <span>{label}</span>
      <span className={`mono ${tone ? `is-${tone}` : ''}`}>{formatMoney(value)}</span>
    </div>
  )
}

export function FinancePanel({ game }: { game: GameApi }) {
  const finance = game.finance

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
        {!finance ? (
          <p className="empty-state">Loading finance data…</p>
        ) : (
          <>
            <div className="finance-block">
              <h3>Income</h3>
              <Row label="Package revenue" value={finance.income.package_revenue} tone="positive" />
              <Row label="Trait bonus" value={finance.income.trait_bonus} tone="positive" />
              <Row label="Total income" value={finance.income.total} tone="positive" />
            </div>
            <div className="finance-block">
              <h3>Expenses</h3>
              <Row label="Employee wages" value={finance.expenses.employee_wages} />
              <Row label="Rent" value={finance.expenses.rent} />
              <Row label="Loan interest" value={finance.expenses.loan_interest} />
              <Row label="Hiring" value={finance.expenses.hiring} />
              <Row label="Other" value={finance.expenses.other} />
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
