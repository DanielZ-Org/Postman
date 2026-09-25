import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { OfficeSelect } from './OfficeSelect'
import type { GameApi } from '../hooks/useGame'
import { makeGameApi, makeGameState, makeOfficeOffer } from '../test/factories'

describe('OfficeSelect', () => {
  it('shows loading state when offices are empty', () => {
    render(<OfficeSelect game={makeGameApi({ offices: [] })} />)
    expect(screen.getByText('Loading available offices…')).toBeInTheDocument()
  })

  it('renders both office cards with down payment and rent', () => {
    render(<OfficeSelect game={makeGameApi()} />)
    expect(screen.getByRole('heading', { name: 'Small Office' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Large Office' })).toBeInTheDocument()
    expect(screen.getByText('£350.00')).toBeInTheDocument()
    expect(screen.getByText('£50.00/wk')).toBeInTheDocument()
    expect(screen.getByText('£450.00')).toBeInTheDocument()
  })

  it('requires two clicks to confirm selection', async () => {
    const user = userEvent.setup()
    const game = makeGameApi({ selectOffice: vi.fn(async () => true) as unknown as GameApi['selectOffice'] })
    render(<OfficeSelect game={game} />)

    const buttons = screen.getAllByRole('button', { name: 'Select office' })
    await user.click(buttons[0])
    expect(game.selectOffice).not.toHaveBeenCalled()

    const confirm = screen.getByRole('button', { name: /Confirm — pay £350.00/ })
    await user.click(confirm)
    expect(game.selectOffice).toHaveBeenCalledWith('office-small-01')
  })

  it('disables offices the player cannot afford', () => {
    const game = makeGameApi({
      state: makeGameState({
        player: { id: 'p', cash: 100, trait: 'financial' },
        office: null,
      }),
      offices: [makeOfficeOffer({ id: 'small' })],
    })
    render(<OfficeSelect game={game} />)
    expect(screen.getByRole('button', { name: 'Select office' })).toBeDisabled()
    expect(screen.getByText('Not enough cash for any office contract.')).toBeInTheDocument()
  })

  it('hides the reset outside mock mode', () => {
    const game = makeGameApi({
      state: makeGameState({
        player: { id: 'p', cash: 0, trait: 'financial' },
        office: null,
      }),
      resetGame: vi.fn(async () => true) as unknown as GameApi['resetGame'],
    })
    render(<OfficeSelect game={game} />)
    expect(screen.getByText('Not enough cash for any office contract.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Start a new game' })).not.toBeInTheDocument()
    expect(game.resetGame).not.toHaveBeenCalled()
  })

  it('offers a reset in mock mode when player cannot afford any office', async () => {
    vi.stubEnv('MODE', 'mock')
    const user = userEvent.setup()
    const game = makeGameApi({
      state: makeGameState({
        player: { id: 'p', cash: 0, trait: 'financial' },
        office: null,
      }),
      resetGame: vi.fn(async () => true) as unknown as GameApi['resetGame'],
    })
    render(<OfficeSelect game={game} />)
    await user.click(screen.getByRole('button', { name: 'Start a new game' }))
    expect(game.resetGame).toHaveBeenCalledTimes(1)
    vi.unstubAllEnvs()
  })
})
