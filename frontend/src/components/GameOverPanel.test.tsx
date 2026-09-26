import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { GameOverPanel } from './GameOverPanel'
import type { GameApi } from '../hooks/useGame'
import { makeEmployee, makeGameApi, makeGameState, makeOffice, makePackage } from '../test/factories'

function finishedGame(overrides: Partial<GameApi> = {}): GameApi {
  return makeGameApi({
    state: makeGameState({
      game: { status: 'game_over', game_datetime: '1980-03-08T16:30:00.000Z', speed: 1 },
      player: { id: 'player-1', cash: 42, trait: 'financial' },
      office: null,
      finance: { accrued_wages: 18, next_rent: 0, loan_principal: 1000 },
    }),
    office: makeOffice({ contract_status: 'terminated', missed_rent_payments: 2 }),
    packages: [
      makePackage({ id: 'pkg-1', status: 'delivered', final_revenue: 12, delivered_at: '1980-02-01T14:00:00.000Z' }),
      makePackage({ id: 'pkg-2', status: 'delivered', final_revenue: 8, delivered_at: '1980-02-02T14:00:00.000Z' }),
      makePackage({ id: 'pkg-3', status: 'stored' }),
    ],
    gameOver: true,
    ...overrides,
  })
}

describe('GameOverPanel', () => {
  it('states the outcome and the SPEC 4.2 reason', () => {
    render(<GameOverPanel game={finishedGame()} />)

    expect(screen.getByRole('heading', { name: 'Game over' })).toBeInTheDocument()
    expect(screen.getByText(/2 missed rent payments/)).toBeInTheDocument()
    expect(screen.getByText(/not enough to sign a new one/)).toBeInTheDocument()
  })

  it('mentions the couriers only when there are any', () => {
    const { unmount } = render(<GameOverPanel game={finishedGame()} />)
    expect(document.querySelector('.game-over-reason')?.textContent).toMatch(/Your courier is let go/)
    unmount()

    const many = finishedGame({
      employees: [makeEmployee(), makeEmployee({ id: 'emp-0002', name: 'Second Courier' })],
    })
    const { unmount: unmountMany } = render(<GameOverPanel game={many} />)
    expect(document.querySelector('.game-over-reason')?.textContent).toMatch(/Your 2 couriers are let go/)
    unmountMany()

    render(<GameOverPanel game={finishedGame({ employees: [] })} />)
    expect(document.querySelector('.game-over-reason')?.textContent).not.toMatch(/let go/)
  })

  it('shows the final standings from the last game state', () => {
    render(<GameOverPanel game={finishedGame()} />)

    expect(screen.getByText('8 Mar 1980, 16:30')).toBeInTheDocument()
    expect(screen.getByText('£42.00')).toBeInTheDocument()
    expect(screen.getByText('Packages delivered')).toBeInTheDocument()
    expect(screen.getByText('2')).toBeInTheDocument()
    expect(screen.getByText('£20.00')).toBeInTheDocument()
    expect(screen.getByText('£1,000.00')).toBeInTheDocument()
    expect(screen.getByText('£18.00')).toBeInTheDocument()
  })

  it('falls back to a reason without a missed-rent count', () => {
    render(<GameOverPanel game={finishedGame({ office: null })} />)
    expect(screen.getByText(/The head-office contract ended/)).toBeInTheDocument()
  })

  it('offers a new game in mock mode and calls resetGame', async () => {
    vi.stubEnv('MODE', 'mock')
    const user = userEvent.setup()
    const game = finishedGame({ resetGame: vi.fn(async () => true) as unknown as GameApi['resetGame'] })
    render(<GameOverPanel game={game} />)

    await user.click(screen.getByRole('button', { name: 'Start a new game' }))
    expect(game.resetGame).toHaveBeenCalledTimes(1)
    vi.unstubAllEnvs()
  })

  it('explains the missing reset outside mock mode (SPEC 1 defers it)', () => {
    render(<GameOverPanel game={finishedGame()} />)
    expect(screen.queryByRole('button', { name: 'Start a new game' })).not.toBeInTheDocument()
    expect(screen.getByText(/no new-game reset/)).toBeInTheDocument()
  })
})
