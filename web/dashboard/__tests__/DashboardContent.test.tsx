import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'

// Hook mocks must be declared before the component import.
const ok = <T,>(data: T) => ({
  data: { success: true as const, data, timestamp: '' },
  isLoading: false,
  isError: false,
  error: null,
})
const err = (message: string) => ({
  data: undefined,
  isLoading: false,
  isError: true,
  error: new Error(message),
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

describe('DashboardContent', () => {
  it('mounts without throwing and renders core sections', () => {
    render(wrap(<DashboardContent />))

    // Stat card titles
    expect(screen.getByText('Total P&L')).toBeInTheDocument()
    expect(screen.getByText('Win Rate')).toBeInTheDocument()
    expect(screen.getByText('Active Positions')).toBeInTheDocument()

    // Section headings
    expect(screen.getByText('Equity Curve')).toBeInTheDocument()
    expect(screen.getByText('Recent Trades')).toBeInTheDocument()

    // Healthy status renders
    expect(screen.getByText(/healthy/i)).toBeInTheDocument()

    // No error banner on success
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})

describe('DashboardContent error surfacing', () => {
  it('renders an error banner when a query reports success:false', () => {
    vi.resetModules()
    vi.doMock('@/hooks/useTradeData', () => ({
      useDashboard: () => err('HTTP 401 unauthorized — check NEXT_PUBLIC_API_KEY'),
      useDashboardPnl: () => ok({ daily: 0, total: 0, equity: [] }),
      useTrades: () => ok([]),
      useUnifiedPortfolio: () => ({ data: undefined }),
      useSystemStatus: () => err('down'),
    }))
    vi.doMock('@/hooks/useAgents', () => ({
      useAgents: () => ok([]),
    }))

    return import('@/components/dashboard/DashboardContent').then(({ default: Comp }) => {
      render(wrap(<Comp />))
      const alert = screen.getByRole('alert')
      expect(alert).toHaveTextContent(/dashboard:.*401/)
      expect(alert).toHaveTextContent(/status:.*down/)
      // System status falls back to 'unknown', not 'healthy'
      expect(screen.getByText(/unknown/i)).toBeInTheDocument()
    })
  })
})
