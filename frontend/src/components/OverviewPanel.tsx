import { useMemo } from 'react'
import type { GameApi } from '../hooks/useGame'
import type { Package } from '../api/types'
import { formatGameDateTime, formatMoney } from '../lib/format'

const STAGES = [
  { id: 'stored', label: 'In warehouse', hint: 'Waiting to be picked up' },
  { id: 'assigned', label: 'Packing', hint: 'Employee preparing the batch' },
  { id: 'out_for_delivery', label: 'Out for delivery', hint: 'Courier on the street' },
  { id: 'delivered', label: 'Delivered', hint: 'Revenue collected' },
] as const

type StageId = (typeof STAGES)[number]['id']

function PackageChip({ pkg, employeeName }: { pkg: Package; employeeName?: string }) {
  return (
    <div className={`pkg-chip pkg-${pkg.status}`} title={`${pkg.id} · ${pkg.size} · ${pkg.service_type}`}>
      <span className={`pkg-size-dot size-${pkg.size}`} />
      <span className="pkg-chip-id">{pkg.id.replace('pkg-', '#')}</span>
      {pkg.service_type === 'express' && <span className="pkg-express-tag">EXP</span>}
      {employeeName && <span className="pkg-chip-emp">{employeeName}</span>}
      {pkg.status === 'delivered' && pkg.final_revenue !== null && (
        <span className="pkg-chip-rev">{formatMoney(pkg.final_revenue)}</span>
      )}
    </div>
  )
}

function CourierCard({ game }: { game: GameApi }) {
  const active = game.employees.filter((e) => e.status !== 'ready')
  const ready = game.employees.filter((e) => e.status === 'ready')

  if (game.employees.length === 0) {
    return (
      <div className="courier-empty">
        <span className="courier-empty-icon">🚶</span>
        <span>No couriers yet — hire one on the Employees tab</span>
      </div>
    )
  }

  return (
    <div className="courier-list">
      {active.map((emp) => (
        <div key={emp.id} className={`courier-card is-${emp.status}`}>
          <span className="courier-avatar" aria-hidden>
            {emp.speed_trait === 'cheetah' ? '🐆' : emp.speed_trait === 'chicken' ? '🐔' : '🐌'}
          </span>
          <div className="courier-meta">
            <strong>{emp.name}</strong>
            <span className={`status-chip status-${emp.status}`}>
              {emp.status === 'packing' ? 'Packing batch' : 'On the street'}
            </span>
          </div>
        </div>
      ))}
      {ready.map((emp) => (
        <div key={emp.id} className="courier-card is-ready">
          <span className="courier-avatar" aria-hidden>
            {emp.speed_trait === 'cheetah' ? '🐆' : emp.speed_trait === 'chicken' ? '🐔' : '🐌'}
          </span>
          <div className="courier-meta">
            <strong>{emp.name}</strong>
            <span className="status-chip status-ready">Ready</span>
          </div>
        </div>
      ))}
    </div>
  )
}

export function OverviewPanel({ game }: { game: GameApi }) {
  const state = game.state
  const employeeNameById = useMemo(() => {
    const map = new Map<string, string>()
    for (const emp of game.employees) map.set(emp.id, emp.name)
    return map
  }, [game.employees])

  const byStage = useMemo(() => {
    const map: Record<StageId, Package[]> = { stored: [], assigned: [], out_for_delivery: [], delivered: [] }
    for (const pkg of game.packages) {
      const stage = pkg.status as StageId
      if (map[stage]) map[stage].push(pkg)
    }
    for (const key of Object.keys(map) as StageId[]) {
      map[key].sort((a, b) => {
        if (key === 'delivered') return (b.delivered_at ?? '').localeCompare(a.delivered_at ?? '')
        if (a.service_type !== b.service_type) return a.service_type === 'express' ? -1 : 1
        return a.due_at.localeCompare(b.due_at)
      })
    }
    return map
  }, [game.packages])

  if (!state) return <p className="empty-state">Loading game state…</p>

  const office = state.office
  const ops = state.operations
  const fin = state.finance

  const storagePct =
    office && office.storage_capacity > 0
      ? Math.round((office.storage_used / office.storage_capacity) * 100)
      : 0

  const deliveredTodayRevenue = byStage.delivered
    .filter((p) => p.delivered_at?.slice(0, 10) === state.game.game_datetime.slice(0, 10))
    .reduce((sum, p) => sum + (p.final_revenue ?? 0), 0)

  const stageCounts: Record<StageId, number> = {
    stored: byStage.stored.length,
    assigned: byStage.assigned.length,
    out_for_delivery: byStage.out_for_delivery.length,
    delivered: byStage.delivered.length,
  }

  return (
    <div className="overview">
      <section className="panel flow-panel">
        <div className="panel-head">
          <div>
            <h2>Delivery flow</h2>
            <p className="muted flow-sub">
              Packages arrive at your warehouse → you assign a courier → they walk local deliveries → revenue lands in cash.
              <span className="flow-local-tag">All destinations local · one office · this city</span>
            </p>
          </div>
          <div className="panel-head-cash">
            <span className="cash-label">Cash</span>
            <span className={`cash-value ${state.player.cash < 0 ? 'is-negative' : ''}`}>
              {formatMoney(state.player.cash)}
            </span>
          </div>
        </div>

        <div className="flow-board" role="list" aria-label="Package delivery pipeline">
          {STAGES.map((stage, index) => (
            <div key={stage.id} className="flow-column-wrap">
              <div className={`flow-column stage-${stage.id}`} role="listitem">
                <header className="flow-col-head">
                  <span className="flow-col-icon" aria-hidden>
                    {stage.id === 'stored' ? '📦' : stage.id === 'assigned' ? '📋' : stage.id === 'out_for_delivery' ? '🚶' : '✓'}
                  </span>
                  <div>
                    <h3>{stage.label}</h3>
                    <p className="muted flow-hint">{stage.hint}</p>
                  </div>
                  <span className="flow-count">{stageCounts[stage.id]}</span>
                </header>
                <div className="flow-chips">
                  {byStage[stage.id].length === 0 ? (
                    <p className="flow-empty">Empty</p>
                  ) : (
                    byStage[stage.id]
                      .slice(0, 12)
                      .map((pkg) => (
                        <PackageChip
                          key={pkg.id}
                          pkg={pkg}
                          employeeName={
                            pkg.assigned_employee_id ? employeeNameById.get(pkg.assigned_employee_id) : undefined
                          }
                        />
                      ))
                  )}
                  {byStage[stage.id].length > 12 && (
                    <p className="flow-more">+{byStage[stage.id].length - 12} more…</p>
                  )}
                </div>
                {stage.id === 'stored' && office && (
                  <footer className="flow-col-foot">
                    <div className="meter-track">
                      <div
                        className={`meter-fill ${storagePct >= 90 ? 'is-critical' : storagePct >= 70 ? 'is-warning' : ''}`}
                        style={{ width: `${Math.min(storagePct, 100)}%` }}
                      />
                    </div>
                    <span className="muted">
                      {office.storage_used}/{office.storage_capacity} units
                    </span>
                  </footer>
                )}
                {stage.id === 'out_for_delivery' && (
                  <footer className="flow-col-foot flow-couriers">
                    <CourierCard game={game} />
                  </footer>
                )}
                {stage.id === 'delivered' && (
                  <footer className="flow-col-foot">
                    <span className="muted">Today: {formatMoney(deliveredTodayRevenue)} · {ops.delivered_today} done</span>
                  </footer>
                )}
              </div>
              {index < STAGES.length - 1 && (
                <span className="flow-arrow" aria-hidden>
                  →
                </span>
              )}
            </div>
          ))}
        </div>
      </section>

      <div className="overview-side">
        <section className="panel">
          <h2>Quick stats</h2>
          <div className="stat-row">
            <div className="stat-card">
              <span className="stat-label">Stored</span>
              <span className="stat-value">{ops.stored_packages}</span>
            </div>
            <div className="stat-card">
              <span className="stat-label">In transit</span>
              <span className="stat-value">{ops.out_for_delivery}</span>
            </div>
            <div className="stat-card">
              <span className="stat-label">Delivered today</span>
              <span className="stat-value is-positive">{ops.delivered_today}</span>
            </div>
            <div className="stat-card">
              <span className="stat-label">Accrued wages</span>
              <span className="stat-value">{formatMoney(fin.accrued_wages)}</span>
            </div>
          </div>
        </section>

        <section className="panel">
          <h2>How a package moves</h2>
          <ol className="flow-legend">
            <li>
              <strong>Arrives</strong> — generated during opening hours into your warehouse (<code>stored</code>).
            </li>
            <li>
              <strong>Assigned</strong> — you pick a ready courier and a batch size (<code>assigned</code>).
            </li>
            <li>
              <strong>Packing</strong> — courier loads for 1 game hour, then walks out (<code>out_for_delivery</code>).
            </li>
            <li>
              <strong>Delivered</strong> — after ~3 game hours locally; revenue + wages recorded (<code>delivered</code>).
            </li>
          </ol>
          <p className="muted flow-note">
            One head office · local destinations only · no inter-city transfers in M1.
          </p>
        </section>

        {game.clock && (
          <section className="panel">
            <h2>Upcoming</h2>
            <ul className="upcoming-list">
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
              <li>
                <span>Game time</span>
                <strong className="mono">{formatGameDateTime(game.clock.game_datetime)}</strong>
              </li>
            </ul>
          </section>
        )}
      </div>
    </div>
  )
}
