import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { FinancePanel } from './FinancePanel'
import { formatMoney } from '../lib/format'
import { makeFinance, makeGameApi, makeGameState, makeTransaction } from '../test/factories'

describe('FinancePanel', () => {
  it('renders income, expenses, liabilities, and cash', () => {
    render(
      <FinancePanel
        game={makeGameApi({
          state: makeGameState({ player: { id: 'player-1', cash: 65000, trait: 'financial' } }), // pence
          finance: makeFinance({
            cash_balance: 65000,
            income: { package_revenue: 12000, trait_bonus: 1200, total: 13200 },
            expenses: {
              employee_wages: 4000,
              rent: 5000,
              loan_interest: 0,
              hiring: 10000,
              vehicle_fuel: 150,
              vehicle_maintenance: 600,
              other: 0,
              total: 19750,
            },
            net_change: -6550,
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
    expect(screen.getByText('Vehicle fuel')).toBeInTheDocument()
    expect(screen.getByText('Vehicle maintenance')).toBeInTheDocument()
    expect(screen.getByText('£1.50')).toBeInTheDocument() // fuel row (150p)
    expect(screen.getByText('£6.00')).toBeInTheDocument() // maintenance row (600p)
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
            makeTransaction({ id: 'txn-1', amount: -35000, category: 'office_down_payment' }),
            makeTransaction({
              id: 'txn-2',
              amount: 1200,
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
          state: makeGameState({ player: { id: 'p', cash: -1000, trait: 'financial' } }),
        })}
      />,
    )
    const negatives = screen.getAllByText('-£10.00')
    expect(negatives.length).toBeGreaterThan(0)
    expect(negatives[0]).toHaveClass('is-negative')
  })

  it('shows a no-loan state when the principal is zero', () => {
    render(<FinancePanel game={makeGameApi()} />)
    expect(screen.getByText('No outstanding loan.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Repay loan' })).not.toBeInTheDocument()
  })

  it('caps the repayable amount at min(cash, principal)', () => {
    render(
      <FinancePanel
        game={makeGameApi({
          state: makeGameState({ player: { id: 'p', cash: 30000, trait: 'financial' } }),
          finance: makeFinance({
            liabilities: {
              loan_principal: 60000,
              accrued_employee_wages: 0,
              next_rent_amount: 5000,
              next_interest_estimate: 3000,
            },
          }),
        })}
      />,
    )
    expect(screen.getByText(`integer pence, up to ${formatMoney(30000)} available`)).toBeInTheDocument()
  })

  it('enables the repay button only for a valid integer amount within the cap', () => {
    render(
      <FinancePanel
        game={makeGameApi({
          state: makeGameState({ player: { id: 'p', cash: 30000, trait: 'financial' } }),
          finance: makeFinance({
            liabilities: {
              loan_principal: 60000,
              accrued_employee_wages: 0,
              next_rent_amount: 5000,
              next_interest_estimate: 3000,
            },
          }),
        })}
      />,
    )
    const input = screen.getByLabelText('Repayment amount in pence')
    const button = screen.getByRole('button', { name: 'Repay loan' })
    expect(button).toBeDisabled()

    for (const invalid of ['0', '-1', '1.5', '30001']) {
      fireEvent.change(input, { target: { value: invalid } })
      expect(button, `amount=${invalid}`).toBeDisabled()
    }

    fireEvent.change(input, { target: { value: '20000' } })
    expect(button).toBeEnabled()
  })

  it('repays the entered amount and clears the input on success', async () => {
    const repayLoan = vi.fn(async (_amount: number) => true)
    render(
      <FinancePanel
        game={makeGameApi({
          finance: makeFinance({
            liabilities: {
              loan_principal: 60000,
              accrued_employee_wages: 0,
              next_rent_amount: 5000,
              next_interest_estimate: 3000,
            },
          }),
          repayLoan,
        })}
      />,
    )
    const input = screen.getByLabelText('Repayment amount in pence')
    fireEvent.change(input, { target: { value: '20000' } })
    fireEvent.click(screen.getByRole('button', { name: 'Repay loan' }))

    expect(repayLoan).toHaveBeenCalledWith(20000)
    await waitFor(() => expect((input as HTMLInputElement).value).toBe(''))
  })

  it('keeps the input when the repayment fails', async () => {
    const repayLoan = vi.fn(async (_amount: number) => false)
    render(
      <FinancePanel
        game={makeGameApi({
          finance: makeFinance({
            liabilities: {
              loan_principal: 60000,
              accrued_employee_wages: 0,
              next_rent_amount: 5000,
              next_interest_estimate: 3000,
            },
          }),
          repayLoan,
        })}
      />,
    )
    const input = screen.getByLabelText('Repayment amount in pence')
    fireEvent.change(input, { target: { value: '20000' } })
    fireEvent.click(screen.getByRole('button', { name: 'Repay loan' }))

    expect(repayLoan).toHaveBeenCalledWith(20000)
    await waitFor(() => expect((input as HTMLInputElement).value).toBe('20000'))
  })
})
