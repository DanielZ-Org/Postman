import type { GameApi } from '../hooks/useGame'
import { formatMoney, formatStatus } from '../lib/format'

export function EmployeesPanel({ game }: { game: GameApi }) {
  const office = game.state?.office
  const hiring = game.hiring
  const employeeCount = office?.employee_count ?? game.employees.length
  const employeeCapacity = office?.employee_capacity
  const atCapacity = employeeCapacity !== undefined && employeeCount >= employeeCapacity
  const nextFee = hiring?.next_hiring_fee
  const cash = game.state?.player.cash ?? 0
  const cannotAfford = nextFee !== undefined && cash < nextFee
  const hireDisabled = atCapacity || cannotAfford || !office

  const handleHire = () => {
    void game.hireEmployee()
  }

  return (
    <section className="panel">
      <div className="panel-head">
        <h2>Employees</h2>
        <span className="muted">
          {employeeCount}
          {employeeCapacity !== undefined ? ` / ${employeeCapacity}` : ''} employed
          {hiring && ` · lifetime hires: ${hiring.total_hires_lifetime}`}
        </span>
      </div>

      <div className="hire-box">
        <div className="hire-info">
          <span className="hire-label">Next hiring fee</span>
          <span className="hire-fee">{nextFee !== undefined ? formatMoney(nextFee) : '—'}</span>
        </div>
        <button type="button" className="btn btn-primary" onClick={handleHire} disabled={hireDisabled}>
          Hire walking employee
        </button>
        {atCapacity && <p className="hire-hint">Office employee capacity reached.</p>}
        {!atCapacity && cannotAfford && <p className="hire-hint">Not enough cash for the hiring fee.</p>}
      </div>

      {game.employees.length === 0 ? (
        <p className="empty-state">No employees yet. Hire your first walking courier above.</p>
      ) : (
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>ID</th>
                <th>Name</th>
                <th>Speed</th>
                <th>Skills</th>
                <th>Mood</th>
                <th>Mode</th>
                <th>Status</th>
                <th>Delivered (week)</th>
                <th>Accrued wages</th>
              </tr>
            </thead>
            <tbody>
              {game.employees.map((emp) => (
                <tr key={emp.id}>
                  <td className="mono">{emp.id}</td>
                  <td>{emp.name}</td>
                  <td>
                    <span className={`speed-chip speed-${emp.speed_trait}`}>{emp.speed_trait}</span>
                  </td>
                  <td>{emp.skills.length > 0 ? emp.skills.join(', ') : '—'}</td>
                  <td>
                    <span className={`mood-chip mood-${emp.mood}`}>{emp.mood}</span>
                  </td>
                  <td>{emp.current_delivery_mode}</td>
                  <td>
                    <span className={`status-chip status-${emp.status}`}>{formatStatus(emp.status)}</span>
                  </td>
                  <td>{emp.packages_delivered_this_week}</td>
                  <td>{formatMoney(emp.accrued_wages)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  )
}
