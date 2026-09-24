import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { DeliveriesPanel } from './DeliveriesPanel'
import type { GameApi } from '../hooks/useGame'
import { makeEmployee, makeGameApi, makePackage } from '../test/factories'

describe('DeliveriesPanel', () => {
  it('tells the player to hire first when there are no employees', () => {
    render(
      <DeliveriesPanel
        game={makeGameApi({
          employees: [],
          packages: [makePackage({ status: 'stored' })],
        })}
      />,
    )
    expect(screen.getByText(/No employees yet\. Hire someone on the Employees tab first\./)).toBeInTheDocument()
  })

  it('shows busy message when all employees are busy', () => {
    render(
      <DeliveriesPanel
        game={makeGameApi({
          employees: [makeEmployee({ status: 'out_for_delivery' })],
          packages: [makePackage({ status: 'stored' })],
        })}
      />,
    )
    expect(screen.getByText('All employees are busy right now.')).toBeInTheDocument()
  })

  it('assigns a batch via the API', async () => {
    const user = userEvent.setup()
    const game = makeGameApi({
      employees: [makeEmployee()],
      packages: [
        makePackage({ id: 'pkg-001', status: 'stored' }),
        makePackage({ id: 'pkg-002', status: 'stored' }),
        makePackage({ id: 'pkg-003', status: 'stored' }),
      ],
      assignDelivery: vi.fn(async () => true) as unknown as GameApi['assignDelivery'],
    })
    render(<DeliveriesPanel game={game} />)

    await user.click(screen.getByRole('button', { name: '+' }))
    expect(screen.getByRole('button', { name: 'Assign 2 package(s)' })).toBeEnabled()
    await user.click(screen.getByRole('button', { name: 'Assign 2 package(s)' }))
    expect(game.assignDelivery).toHaveBeenCalledWith('emp-0001', 2)
  })

  it('lists active runs for busy employees', () => {
    render(
      <DeliveriesPanel
        game={makeGameApi({
          employees: [makeEmployee({ status: 'packing', name: 'Busy Bee' })],
          packages: [makePackage()],
        })}
      />,
    )
    expect(screen.getByText('Busy Bee')).toBeInTheDocument()
    expect(screen.getByText('Packing')).toBeInTheDocument()
  })

  it('shows stored packages waiting table', () => {
    render(
      <DeliveriesPanel
        game={makeGameApi({
          employees: [makeEmployee()],
          packages: [makePackage({ id: 'pkg-wait', status: 'stored', size: 'medium', service_type: 'express' })],
        })}
      />,
    )
    expect(screen.getByText('pkg-wait')).toBeInTheDocument()
    expect(screen.getByText('medium')).toBeInTheDocument()
    expect(screen.getByText('express')).toBeInTheDocument()
  })
})
