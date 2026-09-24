import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { EmployeesPanel } from './EmployeesPanel'
import type { GameApi } from '../hooks/useGame'
import { makeGameApi, makeGameState, makeHiring, makeGameStateOffice } from '../test/factories'

describe('EmployeesPanel', () => {
  it('renders empty state and hire fee', () => {
    render(
      <EmployeesPanel
        game={makeGameApi({
          employees: [],
          hiring: makeHiring({ current_employee_count: 0, total_hires_lifetime: 0, next_hiring_fee: 50 }),
        })}
      />,
    )
    expect(screen.getByText('No employees yet. Hire your first walking courier above.')).toBeInTheDocument()
    expect(screen.getByText('£50.00')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Hire walking employee' })).toBeEnabled()
  })

  it('calls hireEmployee on click', async () => {
    const user = userEvent.setup()
    const game = makeGameApi({ hireEmployee: vi.fn(async () => true) as unknown as GameApi['hireEmployee'] })
    render(<EmployeesPanel game={game} />)
    await user.click(screen.getByRole('button', { name: 'Hire walking employee' }))
    expect(game.hireEmployee).toHaveBeenCalledTimes(1)
  })

  it('disables hire when not enough cash', () => {
    render(
      <EmployeesPanel
        game={makeGameApi({
          state: makeGameState({ player: { id: 'p', cash: 10, trait: 'financial' } }),
          hiring: makeHiring({ next_hiring_fee: 100 }),
          employees: [],
        })}
      />,
    )
    expect(screen.getByRole('button', { name: 'Hire walking employee' })).toBeDisabled()
    expect(screen.getByText('Not enough cash for the hiring fee.')).toBeInTheDocument()
  })

  it('disables hire at employee capacity', () => {
    render(
      <EmployeesPanel
        game={makeGameApi({
          state: makeGameState({
            office: makeGameStateOffice({
              employee_count: 5,
              employee_capacity: 5,
            }),
          }),
          hiring: makeHiring({ current_employee_count: 5 }),
        })}
      />,
    )
    expect(screen.getByRole('button', { name: 'Hire walking employee' })).toBeDisabled()
    expect(screen.getByText('Office employee capacity reached.')).toBeInTheDocument()
  })

  it('renders employee table rows', () => {
    render(<EmployeesPanel game={makeGameApi()} />)
    expect(screen.getByText('Test Courier')).toBeInTheDocument()
    expect(screen.getByText('snail')).toBeInTheDocument()
    expect(screen.getByText('neutral')).toBeInTheDocument()
  })
})
