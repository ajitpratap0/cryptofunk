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
const err = (message: string) => ({
  data: undefined,
  isLoading: false,
  isError: true,
  error: new Error(message),
})

vi.mock('@/hooks/useTradeData', () => ({
  useDashboard: () => err('HTTP 401 unauthorized — check NEXT_PUBLIC_API_KEY'),
  useDashboardPnl: () => ok({ daily: 0, total: 0, equity: [] }),
  useTrades: () => ok([]),
  useUnifiedPortfolio: () => ({ data: undefined }),
  useSystemStatus: () => err('down'),
}))

vi.mock('@/hooks/useAgents', () => ({
  useAgents: () => ok([]),
}))

import DashboardContent from '@/components/dashboard/DashboardContent'

function wrap(children: ReactNode) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>
}

describe('DashboardContent (error path)', () => {
  it('renders an error banner and falls back to unknown status', () => {
    render(wrap(<DashboardContent />))

    const alert = screen.getByRole('alert')
    expect(alert).toHaveTextContent(/dashboard:.*401/)
    expect(alert).toHaveTextContent(/status:.*down/)
    expect(screen.getByText(/unknown/i)).toBeInTheDocument()
  })
})
