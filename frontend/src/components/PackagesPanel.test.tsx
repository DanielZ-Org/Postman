import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { PackagesPanel } from './PackagesPanel'
import { makeGameApi, makePackage } from '../test/factories'

describe('PackagesPanel', () => {
  const packages = [
    makePackage({ id: 'pkg-001', status: 'stored', size: 'small', service_type: 'normal' }),
    makePackage({ id: 'pkg-002', status: 'assigned', size: 'medium' }),
    makePackage({
      id: 'pkg-003',
      status: 'delivered',
      final_revenue: 15,
      delivered_at: '1980-02-01T15:00:00.000Z',
      assigned_employee_id: 'emp-001',
    }),
  ]

  it('renders all packages by default', () => {
    render(<PackagesPanel game={makeGameApi({ packages })} />)
    expect(screen.getByText('pkg-001')).toBeInTheDocument()
    expect(screen.getByText('pkg-002')).toBeInTheDocument()
    expect(screen.getByText('pkg-003')).toBeInTheDocument()
    expect(screen.getByText('£15.00')).toBeInTheDocument()
  })

  it('filters by status', async () => {
    const user = userEvent.setup()
    render(<PackagesPanel game={makeGameApi({ packages })} />)

    await user.click(screen.getByRole('button', { name: /Stored/ }))
    expect(screen.getByText('pkg-001')).toBeInTheDocument()
    expect(screen.queryByText('pkg-002')).not.toBeInTheDocument()
    expect(screen.queryByText('pkg-003')).not.toBeInTheDocument()
  })

  it('shows empty state for a filter with no matches', async () => {
    const user = userEvent.setup()
    render(
      <PackagesPanel
        game={makeGameApi({
          packages: [makePackage({ status: 'stored' })],
        })}
      />,
    )
    await user.click(screen.getByRole('button', { name: /Out For Delivery/ }))
    expect(screen.getByText(/No packages with status/)).toBeInTheDocument()
  })

  it('shows status filter counts', () => {
    render(<PackagesPanel game={makeGameApi({ packages })} />)
    const allBtn = screen.getByRole('button', { name: /All/ })
    expect(within(allBtn).getByText('3')).toBeInTheDocument()
  })
})
