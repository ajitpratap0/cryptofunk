import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { TradesTable } from '@/components/trades/TradesTable'
import type { Trade } from '@/lib/types'

const mockTrades: Trade[] = [
  {
    id: '1',
    symbol: 'BTC/USDT',
    side: 'long',
    entryPrice: 45000,
    currentPrice: 46000,
    quantity: 0.5,
    pnl: 500,
    pnlPercent: 2.2,
    agent: 'momentum-v1',
    confidence: 0.85,
    timestamp: '2024-01-15T10:00:00Z',
    status: 'open',
  },
]

describe('TradesTable', () => {
  it('renders trades data', () => {
    render(<TradesTable trades={mockTrades} />)
    expect(screen.getByText('BTC/USDT')).toBeInTheDocument()
    expect(screen.getByText('momentum-v1')).toBeInTheDocument()
  })

  it('shows loading skeleton', () => {
    const { container } = render(<TradesTable trades={[]} loading={true} />)
    expect(container.querySelector('.animate-pulse')).toBeInTheDocument()
  })

  it('shows empty state', () => {
    render(<TradesTable trades={[]} />)
    expect(screen.getByText('No trades to display')).toBeInTheDocument()
  })
})
