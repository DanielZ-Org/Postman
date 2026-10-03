import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { OverviewPanel } from './OverviewPanel'
import { formatMoney } from '../lib/format'
import { makeGameState, makeGameApi, makeOfficeOffer, makePackage } from '../test/factories'

describe('OverviewPanel (delivery flow)', () => {
  it('shows all four pipeline stages with counts', () => {
    render(<OverviewPanel game={makeGameApi()} />)

    expect(screen.getByRole('heading', { name: 'In warehouse' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Packing' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Out for delivery' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Delivered' })).toBeInTheDocument()

    expect(screen.getByText('Delivery flow')).toBeInTheDocument()
    expect(screen.getByText(/All destinations local/)).toBeInTheDocument()
    expect(screen.getByText(/one office/)).toBeInTheDocument()
  })

  it('groups packages into the correct columns', () => {
    const game = makeGameApi({
      packages: [
        makePackage({ id: 'pkg-001', status: 'stored' }),
        makePackage({ id: 'pkg-002', status: 'stored' }),
        makePackage({ id: 'pkg-003', status: 'out_for_delivery', assigned_employee_id: 'emp-001' }),
        makePackage({ id: 'pkg-004', status: 'delivered', final_revenue: 900, delivered_at: '1980-02-01T14:00:00.000Z' }),
      ],
    })
    render(<OverviewPanel game={game} />)

    expect(screen.getAllByText('#001')).toHaveLength(1)
    expect(screen.getAllByText('#002')).toHaveLength(1)
    expect(screen.getAllByText('#003')).toHaveLength(1)
    expect(screen.getAllByText('#004')).toHaveLength(1)
    expect(screen.getByText('£9.00')).toBeInTheDocument()
  })

  it('lists couriers under out for delivery', () => {
    render(<OverviewPanel game={makeGameApi()} />)
    expect(screen.getAllByText('Test Courier').length).toBeGreaterThan(0)
    expect(screen.getByText('Ready')).toBeInTheDocument()
  })

  it('shows hire hint when there are no employees', () => {
    render(<OverviewPanel game={makeGameApi({ employees: [] })} />)
    expect(screen.getByText(/No couriers yet/)).toBeInTheDocument()
  })

  it('renders quick stats and how-it-works legend', () => {
    render(<OverviewPanel game={makeGameApi()} />)
    expect(screen.getByText('Quick stats')).toBeInTheDocument()
    expect(screen.getByText('How a package moves')).toBeInTheDocument()
    expect(screen.getByText(/Arrives/)).toBeInTheDocument()
    expect(screen.getByText(/local destinations only/)).toBeInTheDocument()
  })

  it('shows storage meter units', () => {
    render(<OverviewPanel game={makeGameApi()} />)
    expect(screen.getByText('6/100 units')).toBeInTheDocument()
  })
})

describe('OverviewPanel head office upgrade (SPEC 4.4)', () => {
  it('shows the head office summary without an upgrade button when the fee is 0', () => {
    render(<OverviewPanel game={makeGameApi()} />)
    expect(screen.getByRole('heading', { name: 'Head office' })).toBeInTheDocument()
    expect(screen.getByText('Small')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Upgrade to large office/ })).not.toBeInTheDocument()
  })

  it('upgrades after a two-step confirm', () => {
    const upgradeOffice = vi.fn(async () => true)
    render(
      <OverviewPanel
        game={makeGameApi({
          offices: [makeOfficeOffer({ upgrade_cost_pence: 10000 })],
          upgradeOffice,
        })}
      />,
    )
    const button = screen.getByRole('button', { name: /Upgrade to large office/ })
    expect(button).toHaveTextContent(formatMoney(10000))

    // First click only arms the confirmation.
    fireEvent.click(button)
    expect(upgradeOffice).not.toHaveBeenCalled()
    const confirm = screen.getByRole('button', { name: /Confirm/ })
    expect(confirm).toHaveTextContent(formatMoney(10000))

    fireEvent.click(confirm)
    expect(upgradeOffice).toHaveBeenCalledTimes(1)
  })

  it('disables the upgrade while cash is below the fee', () => {
    render(
      <OverviewPanel
        game={makeGameApi({
          state: makeGameState({ player: { id: 'p', cash: 5000, trait: 'financial' } }),
          offices: [makeOfficeOffer({ upgrade_cost_pence: 10000 })],
        })}
      />,
    )
    expect(screen.getByRole('button', { name: /Upgrade to large office/ })).toBeDisabled()
    expect(screen.getByText('Not enough cash for the upgrade.')).toBeInTheDocument()
  })
})
