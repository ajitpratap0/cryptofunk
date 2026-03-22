import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { useTrades } from '@/hooks/useTradeData'
import type { Trade } from '@/lib/types'

// ── Mock apiClient ──────────────────────────────────────────────────

vi.mock('@/lib/api', () => ({
  apiClient: {
    getTrades: vi.fn(),
  },
}))

// ── Mock mock-data module so USE_MOCK_DATA = false ──────────────────

vi.mock('@/lib/mock-data', () => ({
  USE_MOCK_DATA: false,
  getMockTrades: vi.fn(() => []),
  getMockDashboard: vi.fn(() => ({})),
  getMockEquityPoints: vi.fn(() => []),
  getMockOrders: vi.fn(() => []),
  getMockPositionsFromTrades: vi.fn(() => []),
  getMockUnifiedPortfolio: vi.fn(() => ({})),
}))

// ── Test helpers ────────────────────────────────────────────────────

function makeWrapper() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  )
}

const fakeTrade: Trade = {
  id: 'trade-abc',
  symbol: 'BTCUSDT',
  side: 'long',
  entryPrice: 45045,
  currentPrice: 45045,
  quantity: 0.1,
  pnl: 0,
  pnlPercent: 0,
  agent: 'paper',
  confidence: 1,
  timestamp: '2026-03-19T16:00:00Z',
  status: 'open',
}

// ── Tests ───────────────────────────────────────────────────────────

describe('useTrades', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('returns trade array from real API when USE_MOCK_DATA is false', async () => {
    const { apiClient } = await import('@/lib/api')
    vi.mocked(apiClient.getTrades).mockResolvedValueOnce({
      success: true,
      data: { trades: [fakeTrade], count: 1 } as unknown as { trades: Trade[]; count: number },
      timestamp: new Date().toISOString(),
    })

    const { result } = renderHook(() => useTrades(), { wrapper: makeWrapper() })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(apiClient.getTrades).toHaveBeenCalledOnce()
    expect(apiClient.getTrades).toHaveBeenCalledWith(50, 0)
    expect(result.current.data?.data).toHaveLength(1)
    expect(result.current.data?.data[0].symbol).toBe('BTCUSDT')
    expect(result.current.data?.data[0].entryPrice).toBe(45045)
  })

  it('throws when API returns success:false', async () => {
    const { apiClient } = await import('@/lib/api')
    vi.mocked(apiClient.getTrades).mockResolvedValueOnce({
      success: false,
      data: null as unknown as { trades: Trade[]; count: number },
      error: 'db error',
      timestamp: new Date().toISOString(),
    })

    const { result } = renderHook(() => useTrades(), { wrapper: makeWrapper() })

    await waitFor(() => expect(result.current.isError).toBe(true))

    expect((result.current.error as Error).message).toBe('db error')
  })

  it('returns empty array when API response has no trades wrapper', async () => {
    const { apiClient } = await import('@/lib/api')
    vi.mocked(apiClient.getTrades).mockResolvedValueOnce({
      success: true,
      data: null as unknown as { trades: Trade[]; count: number },
      timestamp: new Date().toISOString(),
    })

    const { result } = renderHook(() => useTrades(), { wrapper: makeWrapper() })

    await waitFor(() => expect(result.current.isSuccess).toBe(true))

    expect(result.current.data?.data).toEqual([])
  })

  it('accepts custom limit and offset params', async () => {
    const { apiClient } = await import('@/lib/api')
    vi.mocked(apiClient.getTrades).mockResolvedValueOnce({
      success: true,
      data: { trades: [], count: 0 } as unknown as { trades: Trade[]; count: number },
      timestamp: new Date().toISOString(),
    })

    renderHook(() => useTrades(10, 20), { wrapper: makeWrapper() })

    await waitFor(() => expect(vi.mocked(apiClient.getTrades)).toHaveBeenCalled())

    expect(apiClient.getTrades).toHaveBeenCalledWith(10, 20)
  })
})
