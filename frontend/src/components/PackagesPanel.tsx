import { useMemo, useState } from 'react'
import type { GameApi } from '../hooks/useGame'
import { formatGameDateTime, formatMoney, formatStatus } from '../lib/format'

const STATUS_FILTERS = ['all', 'stored', 'assigned', 'out_for_delivery', 'delivered'] as const
type StatusFilter = (typeof STATUS_FILTERS)[number]

export function PackagesPanel({ game }: { game: GameApi }) {
  const [filter, setFilter] = useState<StatusFilter>('all')

  const filtered = useMemo(() => {
    if (filter === 'all') return game.packages
    return game.packages.filter((pkg) => pkg.status === filter)
  }, [game.packages, filter])

  const counts = useMemo(() => {
    const map: Record<string, number> = { all: game.packages.length }
    for (const pkg of game.packages) {
      map[pkg.status] = (map[pkg.status] ?? 0) + 1
    }
    return map
  }, [game.packages])

  return (
    <section className="panel">
      <div className="panel-head">
        <h2>Packages</h2>
        <div className="filter-group" role="group" aria-label="Filter by status">
          {STATUS_FILTERS.map((status) => (
            <button
              key={status}
              type="button"
              className={`filter-btn ${filter === status ? 'is-active' : ''}`}
              onClick={() => setFilter(status)}
            >
              {status === 'all' ? 'All' : formatStatus(status)}
              <span className="filter-count">{counts[status] ?? 0}</span>
            </button>
          ))}
        </div>
      </div>

      {filtered.length === 0 ? (
        <p className="empty-state">No packages{filter !== 'all' ? ` with status "${formatStatus(filter)}"` : ''}.</p>
      ) : (
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>ID</th>
                <th>Size</th>
                <th>Service</th>
                <th>Received</th>
                <th>Due</th>
                <th>Status</th>
                <th>Fee</th>
                <th>Revenue</th>
                <th>Employee</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((pkg) => (
                <tr key={pkg.id}>
                  <td className="mono">{pkg.id}</td>
                  <td>
                    <span className={`size-chip size-${pkg.size}`}>{pkg.size}</span>
                  </td>
                  <td>
                    <span className={`service-chip service-${pkg.service_type}`}>{pkg.service_type}</span>
                  </td>
                  <td className="mono">{formatGameDateTime(pkg.received_at)}</td>
                  <td className="mono">{formatGameDateTime(pkg.due_at)}</td>
                  <td>
                    <span className={`status-chip status-${pkg.status}`}>{formatStatus(pkg.status)}</span>
                  </td>
                  <td>{formatMoney(pkg.base_fee)}</td>
                  <td>{pkg.final_revenue !== null ? formatMoney(pkg.final_revenue) : '—'}</td>
                  <td className="mono">{pkg.assigned_employee_id ?? '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}
