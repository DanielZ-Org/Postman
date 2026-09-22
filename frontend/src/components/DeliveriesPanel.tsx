import { useMemo, useState } from 'react'
import type { GameApi } from '../hooks/useGame'
import { formatStatus } from '../lib/format'

export function DeliveriesPanel({ game }: { game: GameApi }) {
  const [selectedEmployee, setSelectedEmployee] = useState<string>('')
  const [batchSize, setBatchSize] = useState<number>(1)
  const [submitting, setSubmitting] = useState(false)

  const readyEmployees = useMemo(
    () => game.employees.filter((emp) => emp.status === 'ready'),
    [game.employees],
  )
  const storedPackages = useMemo(
    () => game.packages.filter((pkg) => pkg.status === 'stored'),
    [game.packages],
  )
  const busyEmployees = useMemo(
    () => game.employees.filter((emp) => emp.status !== 'ready'),
    [game.employees],
  )

  const maxBatch = storedPackages.length
  const effectiveSelected = readyEmployees.some((emp) => emp.id === selectedEmployee)
    ? selectedEmployee
    : (readyEmployees[0]?.id ?? '')
  const canSubmit = effectiveSelected !== '' && batchSize >= 1 && batchSize <= maxBatch && !submitting

  const handleAssign = () => {
    if (!canSubmit) return
    setSubmitting(true)
    void game.assignDelivery(effectiveSelected, batchSize).finally(() => setSubmitting(false))
  }

  return (
    <div className="panel-grid">
      <section className="panel">
        <div className="panel-head">
          <h2>Assign delivery batch</h2>
          <span className="muted">{storedPackages.length} stored package(s) available</span>
        </div>

        {readyEmployees.length === 0 ? (
          <p className="empty-state">
            {game.employees.length === 0
              ? 'No employees yet. Hire someone on the Employees tab first.'
              : 'All employees are busy right now.'}
          </p>
        ) : maxBatch === 0 ? (
          <p className="empty-state">No stored packages available for delivery.</p>
        ) : (
          <div className="assign-form">
            <label className="field">
              <span className="field-label">Employee</span>
              <select
                value={effectiveSelected}
                onChange={(event) => setSelectedEmployee(event.target.value)}
              >
                {readyEmployees.map((emp) => (
                  <option key={emp.id} value={emp.id}>
                    {emp.name} ({emp.id}) — {emp.speed_trait}, {emp.current_delivery_mode}
                  </option>
                ))}
              </select>
            </label>

            <label className="field">
              <span className="field-label">
                Batch size
                <span className="field-hint">max {maxBatch} stored · walking capacity 10</span>
              </span>
              <div className="batch-controls">
                <button
                  type="button"
                  className="btn btn-step"
                  onClick={() => setBatchSize((size) => Math.max(1, size - 1))}
                  disabled={batchSize <= 1}
                >
                  −
                </button>
                <input
                  type="number"
                  min={1}
                  max={maxBatch}
                  value={batchSize}
                  onChange={(event) => {
                    const value = Number(event.target.value)
                    setBatchSize(Number.isFinite(value) ? value : 1)
                  }}
                />
                <button
                  type="button"
                  className="btn btn-step"
                  onClick={() => setBatchSize((size) => Math.min(maxBatch, size + 1))}
                  disabled={batchSize >= maxBatch}
                >
                  +
                </button>
              </div>
            </label>

            <button type="button" className="btn btn-primary btn-assign" onClick={handleAssign} disabled={!canSubmit}>
              {submitting ? 'Assigning…' : `Assign ${batchSize} package(s)`}
            </button>
          </div>
        )}
      </section>

      <section className="panel">
        <h2>Active runs</h2>
        {busyEmployees.length === 0 ? (
          <p className="empty-state">No deliveries in progress.</p>
        ) : (
          <ul className="run-list">
            {busyEmployees.map((emp) => (
              <li key={emp.id} className="run-item">
                <div className="run-head">
                  <strong>{emp.name}</strong>
                  <span className={`status-chip status-${emp.status}`}>{formatStatus(emp.status)}</span>
                </div>
                <span className="muted mono">{emp.id}</span>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="panel panel-wide">
        <h2>Stored packages waiting</h2>
        {storedPackages.length === 0 ? (
          <p className="empty-state">Warehouse is empty.</p>
        ) : (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Size</th>
                  <th>Service</th>
                  <th>Capacity units</th>
                  <th>Fee</th>
                  <th>Due</th>
                </tr>
              </thead>
              <tbody>
                {storedPackages.map((pkg) => (
                  <tr key={pkg.id}>
                    <td className="mono">{pkg.id}</td>
                    <td>
                      <span className={`size-chip size-${pkg.size}`}>{pkg.size}</span>
                    </td>
                    <td>
                      <span className={`service-chip service-${pkg.service_type}`}>{pkg.service_type}</span>
                    </td>
                    <td>{pkg.delivery_capacity_units}</td>
                    <td>{pkg.base_fee}</td>
                    <td className="mono">{pkg.due_at}</td>
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
