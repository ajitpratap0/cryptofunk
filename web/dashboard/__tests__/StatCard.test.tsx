import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { StatCard } from '@/components/ui/StatCard'

describe('StatCard', () => {
  it('renders title and value', () => {
    render(<StatCard title="Total P&L" value={1234.56} formatAs="currency" />)
    expect(screen.getByText('Total P&L')).toBeInTheDocument()
  })

  it('shows loading state', () => {
    const { container } = render(<StatCard title="Test" value={0} loading={true} />)
    expect(container.querySelector('.skeleton')).toBeInTheDocument()
  })
})
