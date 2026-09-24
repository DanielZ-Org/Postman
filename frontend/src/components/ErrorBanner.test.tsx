import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { ErrorBanner } from './ErrorBanner'
import { ApiError } from '../api/client'
import { makeErrorEntry } from '../test/factories'

describe('ErrorBanner', () => {
  it('renders error code and message', () => {
    render(<ErrorBanner entry={makeErrorEntry('EMPLOYEE_BUSY', 'Employee is busy')} onDismiss={vi.fn()} />)
    expect(screen.getByText('EMPLOYEE_BUSY')).toBeInTheDocument()
    expect(screen.getByText('Employee is busy')).toBeInTheDocument()
    expect(screen.getByRole('alert')).toBeInTheDocument()
  })

  it('renders details when present', () => {
    const entry = {
      err: new ApiError('EMPLOYEE_BUSY', 'busy', 409, { employee_id: 'emp-1' }),
      sticky: true,
    }
    render(<ErrorBanner entry={entry} onDismiss={vi.fn()} />)
    expect(screen.getByText('Details')).toBeInTheDocument()
    expect(screen.getByText(/emp-1/)).toBeInTheDocument()
  })

  it('calls onDismiss when × is clicked', async () => {
    const user = userEvent.setup()
    const onDismiss = vi.fn()
    render(<ErrorBanner entry={makeErrorEntry()} onDismiss={onDismiss} />)
    await user.click(screen.getByRole('button', { name: 'Dismiss error' }))
    expect(onDismiss).toHaveBeenCalledTimes(1)
  })

  it('hides details section when there are no details', () => {
    render(<ErrorBanner entry={makeErrorEntry()} onDismiss={vi.fn()} />)
    expect(screen.queryByText('Details')).not.toBeInTheDocument()
  })
})
