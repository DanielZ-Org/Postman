import type { GameApi } from '../hooks/useGame'
import { formatGameDateTime, formatMoney } from '../lib/format'

// GameOverPanel is the terminal screen (SPEC 4.2). The run ends when the office
// contract is not active and the remaining cash cannot cover a new down payment.
// It replaces the whole dashboard: there is nothing left to play, so the clock
// controls and the tabs are hidden rather than left to fail with 409s.
export function GameOverPanel({ game }: { game: GameApi }) {
  const state = game.state
  const office = game.office
  const endedAt = state?.game.game_datetime ?? game.clock?.game_datetime ?? null

  const delivered = game.packages.filter((pkg) => pkg.status === 'delivered')
  const revenue = delivered.reduce((sum, pkg) => sum + (pkg.final_revenue ?? 0), 0)
  const missedRents = office?.missed_rent_payments ?? 0
  const cash = formatMoney(state?.player.cash ?? 0)
  const couriers = game.employees.length

  return (
    <section className="panel game-over" aria-labelledby="game-over-title">
      <header className="game-over-head">
        <span className="game-over-mark" aria-hidden>
          ⛔
        </span>
        <div>
          <h2 id="game-over-title">Game over</h2>
          <p className="muted game-over-reason">
            {missedRents >= 2 ? (
              <>
                The head-office contract was terminated after {missedRents} missed rent payments
                {office ? <> ({office.type} office)</> : null}, and {cash} is not enough to sign a new one.
                {couriers > 0 && <> Your {couriers === 1 ? 'courier is' : `${couriers} couriers are`} let go.</>}
              </>
            ) : (
              <>
                The head-office contract ended and {cash} is not enough to sign a new one.
                {couriers > 0 && <> Your {couriers === 1 ? 'courier is' : `${couriers} couriers are`} let go.</>}
              </>
            )}
          </p>
        </div>
      </header>

      <div>
        <h3>Final standings</h3>
        <dl className="kv-list game-over-stats">
          <div>
            <dt>Ended</dt>
            <dd className="mono">{endedAt ? formatGameDateTime(endedAt) : '—'}</dd>
          </div>
          <div>
            <dt>Cash</dt>
            <dd className={state && state.player.cash < 0 ? 'is-negative' : undefined}>
              {formatMoney(state?.player.cash ?? 0)}
            </dd>
          </div>
          <div>
            <dt>Packages delivered</dt>
            <dd>{delivered.length}</dd>
          </div>
          <div>
            <dt>Delivery revenue</dt>
            <dd className="is-positive">{formatMoney(revenue)}</dd>
          </div>
          <div>
            <dt>Loan principal</dt>
            <dd>{formatMoney(state?.finance.loan_principal ?? 0)}</dd>
          </div>
          <div>
            <dt>Accrued wages</dt>
            <dd>{formatMoney(state?.finance.accrued_wages ?? 0)}</dd>
          </div>
        </dl>
      </div>

      <footer className="game-over-actions">
        {import.meta.env.MODE === 'mock' ? (
          <button type="button" className="btn btn-primary" onClick={() => void game.resetGame()}>
            Start a new game
          </button>
        ) : (
          <p className="muted game-over-reset-note">
            This build has no new-game reset (SPEC 1 defers it), so the save stays finished. Delete{' '}
            <code className="mono">backend/data/postman.db</code> and restart the backend to play again.
          </p>
        )}
      </footer>
    </section>
  )
}
