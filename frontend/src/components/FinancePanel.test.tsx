import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { FinancePanel } from './FinancePanel'
import { makeFinance, makeGameApi, makeGameState, makeTransaction } from '../test/factories'

describe('FinancePanel', () => {
  it('renders income, expenses, liabilities, and cash', () => {
    render(
      <FinancePanel
        game={makeGameApi({
          state: makeGameState({ player: { id: 'player-1', cash: 650, trait: 'financial' } }),
          finance: makeFinance({
            cash_balance: 650,
            income: { package_revenue: 120, trait_bonus: 12, total: 132 },
            expenses: { employee_wages: 40, rent: 50, loan_interest: 0, hiring: 100, other: 0, total: 190 },
            net_change: -58,
          }),
        })}
      />,
    )

    expect(screen.getByText('Finance statement')).toBeInTheDocument()
    expect(screen.getByText('Package revenue')).toBeInTheDocument()
    expect(screen.getByText('£120.00')).toBeInTheDocument()
    expect(screen.getByText('Trait bonus')).toBeInTheDocument()
    expect(screen.getByText('£12.00')).toBeInTheDocument()
    expect(screen.getByText('Employee wages')).toBeInTheDocument()
    expect(screen.getByText('Net change')).toBeInTheDocument()
    expect(screen.getByText('Loan principal')).toBeInTheDocument()
    expect(screen.getByText('Cash balance')).toBeInTheDocument()
    expect(screen.getByText('£650.00')).toBeInTheDocument()
  })

  it('shows empty transactions state', () => {
    render(<FinancePanel game={makeGameApi({ transactions: [] })} />)
    expect(screen.getByText('No transactions recorded yet.')).toBeInTheDocument()
  })

  it('renders transaction rows', () => {
    render(
      <FinancePanel
        game={makeGameApi({
          transactions: [
            makeTransaction({ id: 'txn-1', amount: -350, category: 'office_down_payment' }),
            makeTransaction({
              id: 'txn-2',
              amount: 12,
              category: 'package_revenue',
              description: 'Delivery revenue',
            }),
          ],
        })}
      />,
    )
    expect(screen.getByText('office_down_payment')).toBeInTheDocument()
    expect(screen.getByText('package_revenue')).toBeInTheDocument()
    expect(screen.getByText('-£350.00')).toBeInTheDocument()
    expect(screen.getByText('£12.00')).toBeInTheDocument()
  })

  it('marks negative cash', () => {
    render(
      <FinancePanel
        game={makeGameApi({
          state: makeGameState({ player: { id: 'p', cash: -10, trait: 'financial' } }),
        })}
      />,
    )
    const negatives = screen.getAllByText('-£10.00')
    expect(negatives.length).toBeGreaterThan(0)
    expect(negatives[0]).toHaveClass('is-negative')
  })
})
