import type { GameApi } from '../hooks/useGame'
import { capitalize, formatGameDateTime, formatMoney } from '../lib/format'

const SPEEDS = [1, 2, 3] as const

export function TopBar({ game }: { game: GameApi }) {
  const clock = game.clock
  const cash = game.state?.player.cash ?? 0
  const trait = game.state?.player.trait
  // Once the run is over the clock is frozen by the backend and every clock mutation
  // is rejected, so the controls are replaced by a status badge instead of left to fail.
  const finished = game.gameOver

  const handleSpeed = (speed: (typeof SPEEDS)[number]) => {
    void game.setSpeed(speed)
  }

  const handlePause = () => {
    void game.togglePause()
  }

  const handleSkip = () => {
    void game.skipToNextOpening()
  }

  return (
    <header className="topbar">
      <div className="topbar-brand">
        <img src="/logo-icon.png" alt="" className="topbar-logo" width={28} height={28} />
        <span className="topbar-title">Delivery Office</span>
      </div>

      <div className="topbar-clock" aria-live="polite">
        <span className="topbar-datetime">
          {clock ? formatGameDateTime(clock.game_datetime) : '—'}
        </span>
        {finished ? (
          <span className="open-badge is-game-over">Game over</span>
        ) : (
          clock && (
            <span className={`open-badge ${clock.office_open ? 'is-open' : 'is-closed'}`}>
              {clock.office_open ? 'Open' : 'Closed'} · {capitalize(clock.day_of_week)}
            </span>
          )
        )}
      </div>

      <div className="topbar-controls">
        {!finished && (
          <>
            <button
              type="button"
              className={`ctl-btn ${clock?.paused ? 'is-active' : ''}`}
              onClick={handlePause}
              title={clock?.paused ? 'Resume' : 'Pause'}
            >
              {clock?.paused ? '▶' : '❚❚'}
            </button>
            {SPEEDS.map((speed) => (
              <button
                key={speed}
                type="button"
                className={`ctl-btn ${clock && !clock.paused && clock.speed === speed ? 'is-active' : ''}`}
                onClick={() => handleSpeed(speed)}
              >
                {speed}×
              </button>
            ))}
            <button type="button" className="ctl-btn ctl-skip" onClick={handleSkip} title="Skip to next opening">
              Skip →
            </button>
          </>
        )}
        {import.meta.env.MODE === 'mock' && (
          <button
            type="button"
            className="ctl-btn ctl-reset"
            onClick={() => void game.resetGame()}
            title="Reset mock game to a fresh save"
          >
            ↺ New game
          </button>
        )}
      </div>

      <div className="topbar-cash">
        <span className="cash-label">Cash</span>
        <span className={`cash-value ${cash < 0 ? 'is-negative' : ''}`}>{formatMoney(cash)}</span>
        {trait && <span className="trait-chip">{capitalize(String(trait))}</span>}
      </div>

      {clock && !finished && (
        <div className="topbar-countdowns">
          <span className="countdown">
            Payroll <strong>{clock.days_until_next_payroll}d</strong>
          </span>
          <span className="countdown">
            Rent <strong>{clock.days_until_next_rent}d</strong>
          </span>
        </div>
      )}
    </header>
  )
}
