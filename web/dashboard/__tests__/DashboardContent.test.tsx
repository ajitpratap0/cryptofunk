import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'

const ok = <T,>(data: T) => ({
  data: { success: true as const, data, timestamp: '' },
  isLoading: false,
  isError: false,
  error: null,
})

vi.mock('@/hooks/useTradeData', () => ({
  useDashboard: () =>
    ok({
      totalPnl: 0,
      winRate: 0,
      activePositions: 0,
      totalTrades: 0,
      equity: 0,
      availableBalance: 0,
      marginUsed: 0,
    }),
  useDashboardPnl: () => ok({ daily: 0, total: 0, equity: [] }),
  useTrades: () => ok([]),
  useUnifiedPortfolio: () => ({ data: undefined }),
  useSystemStatus: () => ok({ status: 'healthy', services: {} }),
}))

vi.mock('@/hooks/useAgents', () => ({
  useAgents: () => ok([]),
}))

import DashboardContent from '@/components/dashboard/DashboardContent'

function wrap(children: ReactNode) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>
}

describe('DashboardContent (happy path)', () => {
  it('mounts and renders core sections', () => {
    render(wrap(<DashboardContent />))

    expect(screen.getByText('Total P&L')).toBeInTheDocument()
    expect(screen.getByText('Win Rate')).toBeInTheDocument()
    expect(screen.getByText('Active Positions')).toBeInTheDocument()
    expect(screen.getByText('Equity Curve')).toBeInTheDocument()
    expect(screen.getByText('Recent Trades')).toBeInTheDocument()
    expect(screen.getByText(/healthy/i)).toBeInTheDocument()

    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})
