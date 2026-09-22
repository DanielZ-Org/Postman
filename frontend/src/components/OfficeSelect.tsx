import { useState } from 'react'
import type { GameApi } from '../hooks/useGame'
import type { Office } from '../api/types'
import { formatMoney } from '../lib/format'

function OfficeCard({
  office,
  onSelect,
  disabled,
}: {
  office: Office
  onSelect: () => void
  disabled: boolean
}) {
  const [confirming, setConfirming] = useState(false)
  const label = office.type === 'large' ? 'Large Office' : 'Small Office'

  const handlePrimary = () => {
    if (!confirming) {
      setConfirming(true)
      return
    }
    setConfirming(false)
    onSelect()
  }

  return (
    <article className={`office-card office-${office.type}`}>
      <header className="office-card-head">
        <h3>{label}</h3>
        <span className="office-rent">{formatMoney(office.weekly_rent)}/wk</span>
      </header>
      <dl className="office-specs">
        <div>
          <dt>Down payment</dt>
          <dd>{formatMoney(office.down_payment)}</dd>
        </div>
        <div>
          <dt>First rent due</dt>
          <dd>after {office.rent_prepaid_weeks} weeks</dd>
        </div>
        <div>
          <dt>Storage</dt>
          <dd>
            {office.storage.base_capacity} units
            {office.storage.maximum_capacity > office.storage.base_capacity && (
              <span className="muted"> → max {office.storage.maximum_capacity}</span>
            )}
          </dd>
        </div>
        <div>
          <dt>Employees</dt>
          <dd>{office.employee_capacity}</dd>
        </div>
        <div>
          <dt>Bicycles</dt>
          <dd>{office.bicycle_capacity}</dd>
        </div>
        <div>
          <dt>Vehicles</dt>
          <dd>{office.vehicle_capacity}</dd>
        </div>
        <div>
          <dt>Accepts</dt>
          <dd>{office.accepted_package_sizes.join(', ')}</dd>
        </div>
      </dl>
      <button
        type="button"
        className={`btn ${confirming ? 'btn-confirm' : 'btn-primary'} btn-block`}
        onClick={handlePrimary}
        disabled={disabled}
      >
        {confirming ? `Confirm — pay ${formatMoney(office.down_payment)}` : 'Select office'}
      </button>
    </article>
  )
}

export function OfficeSelect({ game }: { game: GameApi }) {
  const cash = game.state?.player.cash ?? 0

  const handleSelect = (officeId: string) => {
    void game.selectOffice(officeId)
  }

  if (game.offices.length === 0) {
    return (
      <section className="panel">
        <h2>Choose your head office</h2>
        <p className="empty-state">Loading available offices…</p>
      </section>
    )
  }

  return (
    <section className="panel office-select">
      <div className="panel-head">
        <div>
          <h2>Choose your head office</h2>
          <p className="muted">
            The down payment includes the first {game.offices[0]?.rent_prepaid_weeks ?? 4} weeks of rent.
          </p>
        </div>
        <div className="panel-head-cash">
          <span className="cash-label">Available cash</span>
          <span className="cash-value">{formatMoney(cash)}</span>
        </div>
      </div>
      <div className="office-grid">
        {game.offices.map((office) => (
          <OfficeCard
            key={office.id}
            office={office}
            onSelect={() => handleSelect(office.id)}
            disabled={cash < office.down_payment}
          />
        ))}
      </div>
      {game.offices.every((office) => cash < office.down_payment) && (
        <p className="empty-state">Not enough cash for any office contract.</p>
      )}
    </section>
  )
}
