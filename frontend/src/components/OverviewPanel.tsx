import type { GameApi } from '../hooks/useGame'
import { formatGameDateTime, formatMoney } from '../lib/format'

function Stat({ label, value, sub, tone }: { label: string; value: string; sub?: string; tone?: 'positive' | 'negative' }) {
  return (
    <div className="stat-card">
      <span className="stat-label">{label}</span>
      <span className={`stat-value ${tone ? `is-${tone}` : ''}`}>{value}</span>
      {sub && <span className="stat-sub">{sub}</span>}
    </div>
  )
}

export function OverviewPanel({ game }: { game: GameApi }) {
  const state = game.state
  if (!state) return <p className="empty-state">Loading game state…</p>

  const office = state.office
  const ops = state.operations
  const fin = state.finance
  const storagePct =
    office && office.storage_capacity > 0
      ? Math.round((office.storage_used / office.storage_capacity) * 100)
      : 0

  return (
    <div className="panel-grid">
      <section className="panel">
        <h2>Operations</h2>
        <div className="stat-row">
          <Stat label="Stored packages" value={String(ops.stored_packages)} />
          <Stat label="Out for delivery" value={String(ops.out_for_delivery)} />
          <Stat label="Delivered today" value={String(ops.delivered_today)} tone="positive" />
        </div>
        {office && (
          <div className="storage-meter">
            <div className="storage-meter-head">
              <span>Warehouse</span>
              <span>
                {office.storage_used} / {office.storage_capacity} units ({storagePct}%)
              </span>
            </div>
            <div className="meter-track">
              <div
                className={`meter-fill ${storagePct >= 90 ? 'is-critical' : storagePct >= 70 ? 'is-warning' : ''}`}
                style={{ width: `${Math.min(storagePct, 100)}%` }}
              />
            </div>
          </div>
        )}
      </section>

      <section className="panel">
        <h2>Finance</h2>
        <div className="stat-row">
          <Stat label="Cash" value={formatMoney(state.player.cash)} tone={state.player.cash < 0 ? 'negative' : undefined} />
          <Stat label="Accrued wages" value={formatMoney(fin.accrued_wages)} />
          <Stat label="Next rent" value={formatMoney(fin.next_rent)} />
          <Stat label="Loan principal" value={formatMoney(fin.loan_principal)} />
        </div>
      </section>

      {office && (
        <section className="panel">
          <h2>Office</h2>
          <dl className="kv-list">
            <div>
              <dt>ID</dt>
              <dd className="mono">{office.id}</dd>
            </div>
            <div>
              <dt>Employees</dt>
              <dd>
                {office.employee_count} / {office.employee_capacity}
              </dd>
            </div>
            <div>
              <dt>Storage</dt>
              <dd>
                {office.storage_used} / {office.storage_capacity}
              </dd>
            </div>
          </dl>
        </section>
      )}

      <section className="panel">
        <h2>Upcoming</h2>
        <ul className="upcoming-list">
          {game.clock && (
            <>
              <li>
                <span>Payroll</span>
                <strong>
                  {game.clock.days_until_next_payroll === 0
                    ? 'today'
                    : `in ${game.clock.days_until_next_payroll} day(s)`}
                </strong>
              </li>
              <li>
                <span>Rent</span>
                <strong>
                  {game.clock.days_until_next_rent === 0 ? 'today' : `in ${game.clock.days_until_next_rent} day(s)`}
                </strong>
              </li>
            </>
          )}
          <li>
            <span>Game time</span>
            <strong className="mono">{game.clock ? formatGameDateTime(game.clock.game_datetime) : '—'}</strong>
          </li>
        </ul>
      </section>
    </div>
  )
}
