import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { TopBar } from './TopBar'
import type { GameApi } from '../hooks/useGame'
import { makeClock, makeGameApi, makeGameState } from '../test/factories'

describe('TopBar', () => {
  it('renders clock, cash, trait, and countdowns', () => {
    const game = makeGameApi()
    render(<TopBar game={game} />)

    expect(screen.getByText('Delivery Office')).toBeInTheDocument()
    expect(screen.getByText('1 Feb 1980, 09:00')).toBeInTheDocument()
    expect(screen.getByText('£1,000.00')).toBeInTheDocument()
    expect(screen.getByText('Financial')).toBeInTheDocument()
    expect(screen.getByText('Payroll')).toBeInTheDocument()
    expect(screen.getByText('Rent')).toBeInTheDocument()
  })

  it('shows Open/Closed badge from clock.office_open', () => {
    render(<TopBar game={makeGameApi({ clock: makeClock({ office_open: false, day_of_week: 'sunday' }) })} />)
    expect(screen.getByText(/Closed · Sunday/)).toBeInTheDocument()
  })

  it('highlights pause control when paused and calls togglePause', async () => {
    const user = userEvent.setup()
    const game = makeGameApi({
      clock: makeClock({ paused: true }),
      togglePause: vi.fn(async () => undefined) as GameApi['togglePause'],
    })
    render(<TopBar game={game} />)

    const pauseBtn = screen.getByTitle('Resume')
    expect(pauseBtn).toHaveClass('is-active')
    await user.click(pauseBtn)
    expect(game.togglePause).toHaveBeenCalledTimes(1)
  })

  it('calls setSpeed when speed buttons are clicked', async () => {
    const user = userEvent.setup()
    const game = makeGameApi({ setSpeed: vi.fn(async () => undefined) as GameApi['setSpeed'] })
    render(<TopBar game={game} />)

    await user.click(screen.getByRole('button', { name: '3×' }))
    expect(game.setSpeed).toHaveBeenCalledWith(3)
  })

  it('calls skipToNextOpening', async () => {
    const user = userEvent.setup()
    const game = makeGameApi({
      skipToNextOpening: vi.fn(async () => undefined) as GameApi['skipToNextOpening'],
    })
    render(<TopBar game={game} />)

    await user.click(screen.getByTitle('Skip to next opening'))
    expect(game.skipToNextOpening).toHaveBeenCalledTimes(1)
  })

  it('shows reset control only in mock mode and calls resetGame', async () => {
    vi.stubEnv('MODE', 'mock')
    const user = userEvent.setup()
    const game = makeGameApi({
      resetGame: vi.fn(async () => true) as unknown as GameApi['resetGame'],
    })
    render(<TopBar game={game} />)

    const resetBtn = screen.getByTitle('Reset mock game to a fresh save')
    await user.click(resetBtn)
    expect(game.resetGame).toHaveBeenCalledTimes(1)
    vi.unstubAllEnvs()
  })

  it('hides reset control outside mock mode', () => {
    render(<TopBar game={makeGameApi()} />)
    expect(screen.queryByTitle('Reset mock game to a fresh save')).not.toBeInTheDocument()
  })

  it('marks cash negative', () => {
    render(
      <TopBar
        game={makeGameApi({ state: makeGameState({ player: { id: 'p', cash: -120, trait: 'financial' } }) })}
      />,
    )
    const cash = screen.getByText('-£120.00')
    expect(cash).toHaveClass('is-negative')
  })

  it('replaces the clock controls with a game over badge once the run ends', () => {
    render(
      <TopBar
        game={makeGameApi({
          gameOver: true,
          state: makeGameState({ game: { status: 'game_over', game_datetime: '1980-03-08T16:30:00.000Z', speed: 1 } }),
        })}
      />,
    )

    expect(screen.getByText('Game over')).toBeInTheDocument()
    expect(screen.queryByTitle('Pause')).not.toBeInTheDocument()
    expect(screen.queryByTitle('Skip to next opening')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '3×' })).not.toBeInTheDocument()
    expect(screen.queryByText('Payroll')).not.toBeInTheDocument()
    expect(screen.queryByText('Rent')).not.toBeInTheDocument()
  })
})
