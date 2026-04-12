'use client'

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '@/lib/api'
import { REFRESH_INTERVALS } from '@/lib/constants'
import {
  USE_MOCK_DATA,
  getMockTrades,
  getMockDashboard,
  getMockOrders,
  getMockPositionsFromTrades,
  getMockUnifiedPortfolio,
} from '@/lib/mock-data'
import {
  isWrappedResponse,
  isWrappedOrders,
  isRawDashboardResponse,
  isRawDashboardPnlResponse,
} from '@/types/api-responses'
import type { Trade, Position, Order, UnifiedPortfolio, DashboardStats } from '@/lib/types'

// RawApiTrade matches the JSON shape produced by encoding/json on db.Trade
// with snake_case struct tags (added in the backend data-pipeline update).
interface RawApiTrade {
  id: string
  order_id: string
  exchange_trade_id?: string | null
  symbol: string
  exchange: string
  side: string           // "BUY" | "SELL" from db.OrderSide
  price: number
  quantity: number
  quote_quantity: number
  commission: number
  commission_asset?: string | null
  executed_at: string
  is_maker: boolean
  metadata?: Record<string, unknown> | null
  created_at: string
}

// mapApiTrade converts the snake_case backend shape to the camelCase Trade type
// used throughout the dashboard. Fields that have no direct backend equivalent
// (entryPrice, currentPrice, pnl, pnlPercent, confidence, status, agent) are
// derived or defaulted — the trades table stores exchange fills, not strategy
// metadata, so those fields will be populated once the backend exposes them.
function mapApiTrade(raw: RawApiTrade): Trade {
  const side: Trade['side'] = raw.side === 'SELL' ? 'short' : 'long'
  return {
    id: raw.id,
    symbol: raw.symbol,
    side,
    // exchange fills record the execution price; use it for both entry and current
    entryPrice: raw.price,
    currentPrice: raw.price,
    quantity: raw.quantity,
    // PnL is not available in the fills table — default to 0 until the backend
    // enriches the response with position-level PnL.
    pnl: 0,
    pnlPercent: 0,
    agent: raw.exchange,  // proxy until a dedicated agent_id field is on the trade row
    confidence: 0,
    timestamp: raw.executed_at,
    // A BUY fill opens a position ('open'); a SELL fill closes one ('closed').
    status: (raw.side === 'SELL' ? 'closed' : 'open') as Trade['status'],
    reasoning: undefined,
    exitPrice: undefined,
    exitTimestamp: undefined,
  }
}

// Query Keys
export const QUERY_KEYS = {
  trades: ['trades'],
  positions: ['positions'],
  orders: ['orders'],
  dashboard: ['dashboard'],
  dashboardStats: ['dashboard', 'stats'],
  dashboardPositions: ['dashboard', 'positions'],
  dashboardPnl: ['dashboard', 'pnl'],
  equityHistory: ['equity', 'history'],
} as const

// Trades
export function useTrades(limit = 50, offset = 0) {
  return useQuery({
    queryKey: [...QUERY_KEYS.trades, limit, offset],
    queryFn: async () => {
      if (USE_MOCK_DATA) {
        // NOTE: mock mode returns all trades regardless of limit/offset params.
        // Pagination is only enforced against the real API.
        return {
          success: true as const,
          data: getMockTrades(),
          timestamp: new Date().toISOString(),
        }
      }

      const response = await apiClient.getTrades(limit, offset)
      if (!response.success) {
        throw new Error(response.error || 'Failed to fetch trades')
      }

      const raw: unknown = response.data
      const tradeList: Trade[] =
        raw && typeof raw === 'object' && 'trades' in raw
          ? (raw as { trades: RawApiTrade[] }).trades.map(mapApiTrade)
          : []
      return {
        success: true as const,
        data: tradeList,
        timestamp: response.timestamp,
      }
    },
    staleTime: REFRESH_INTERVALS.trades,
    refetchInterval: REFRESH_INTERVALS.trades,
  })
}

// #104: useTrade previously always called getMockTrades(), ignoring
// USE_MOCK_DATA entirely. Now follows the same mock/real pattern as
// useTrades. When hitting the real API, checks the useTrades query
// cache first to avoid a duplicate network request, then falls back
// to a large-limit fetch (1000) so trades beyond the default page
// size aren't silently truncated.
//
// Returns data: Trade | null. null means the trade wasn't found in
// the fetched data (not an API error). Callers should check
// result.data?.data !== null before rendering detail views.
//
// refetchInterval is intentionally omitted: polling a detail view
// by re-fetching the full trade list on every interval is expensive.
// staleTime is kept so the cache is reused on tab switches.
export function useTrade(id: string) {
  const queryClient = useQueryClient()
  return useQuery({
    queryKey: [...QUERY_KEYS.trades, id],
    queryFn: async () => {
      if (USE_MOCK_DATA) {
        const trades = getMockTrades()
        const trade = trades.find(t => t.id === id) ?? null
        return {
          success: true as const,
          data: trade,
          timestamp: new Date().toISOString(),
        }
      }

      // Check the useTrades list cache first to avoid a duplicate
      // network request when the list and detail views are mounted
      // simultaneously.
      const cachedQueries = queryClient.getQueriesData<{
        success: boolean
        data: Trade[]
        timestamp: string
      }>({ queryKey: QUERY_KEYS.trades })
      for (const [, cached] of cachedQueries) {
        // getQueriesData does a prefix match on ['trades'], so it
        // returns both useTrades entries (data: Trade[]) AND prior
        // useTrade entries (data: Trade | null). Guard with
        // Array.isArray so we don't call .find() on a non-array.
        if (!cached || !Array.isArray(cached.data)) continue
        const found = cached.data.find((t: Trade) => t.id === id)
        if (found) {
          return {
            success: true as const,
            data: found,
            timestamp: cached.timestamp,
          }
        }
      }

      // Cache miss — fetch with a large limit so trades beyond the
      // default page size (50) aren't silently truncated.
      const response = await apiClient.getTrades(1000, 0)
      if (!response.success) {
        throw new Error(response.error || 'Failed to fetch trade')
      }

      const raw: unknown = response.data
      const tradeList: Trade[] =
        raw && typeof raw === 'object' && 'trades' in raw
          ? (raw as { trades: RawApiTrade[] }).trades.map(mapApiTrade)
          : []
      const trade = tradeList.find(t => t.id === id) ?? null
      return {
        success: true as const,
        data: trade,
        timestamp: response.timestamp,
      }
    },
    enabled: !!id,
    // Longer staleTime than the list hook: without refetchInterval
    // the detail view only refreshes on remount/refocus, so a short
    // stale window (2s) would go stale immediately and never auto-
    // refresh, leaving the user with frozen data. 30s gives a
    // reasonable cache-reuse window for tab switches.
    staleTime: 30_000,
  })
}

// Positions
export function usePositions() {
  return useQuery({
    queryKey: QUERY_KEYS.positions,
    queryFn: async () => {
      if (USE_MOCK_DATA) {
        return {
          success: true as const,
          data: getMockPositionsFromTrades(),
          timestamp: new Date().toISOString(),
        }
      }

      const response = await apiClient.getPositions()

      if (!response.success) {
        throw new Error(response.error || 'Failed to fetch positions')
      }

      // API returns {positions: [...], count: N} - extract the array
      const rawData: unknown = response.data
      if (isWrappedResponse(rawData)) {
        return {
          success: true as const,
          data: rawData.positions as Position[],
          timestamp: response.timestamp,
        }
      }

      return response
    },
    staleTime: REFRESH_INTERVALS.positions,
    refetchInterval: REFRESH_INTERVALS.positions,
  })
}

export function usePosition(symbol: string) {
  return useQuery({
    queryKey: [...QUERY_KEYS.positions, symbol],
    queryFn: () => apiClient.getPosition(symbol),
    enabled: !!symbol,
    staleTime: REFRESH_INTERVALS.positions,
  })
}

// Orders
export function useOrders() {
  return useQuery({
    queryKey: QUERY_KEYS.orders,
    queryFn: async () => {
      if (USE_MOCK_DATA) {
        return {
          success: true as const,
          data: getMockOrders(),
          timestamp: new Date().toISOString(),
        }
      }

      const response = await apiClient.getOrders()

      if (!response.success) {
        throw new Error(response.error || 'Failed to fetch orders')
      }

      // API returns {orders: [...], count: N} - extract the array
      const rawData: unknown = response.data
      if (isWrappedOrders(rawData)) {
        return {
          success: true as const,
          data: rawData.orders as Order[],
          timestamp: response.timestamp,
        }
      }

      return response
    },
    staleTime: REFRESH_INTERVALS.trades,
    refetchInterval: REFRESH_INTERVALS.trades,
  })
}

// Dashboard
export function useDashboard() {
  return useQuery({
    queryKey: QUERY_KEYS.dashboardStats,
    queryFn: async () => {
      if (USE_MOCK_DATA) {
        return {
          success: true as const,
          data: getMockDashboard(),
          timestamp: new Date().toISOString(),
        }
      }

      const response = await apiClient.getDashboard()

      if (!response.success) {
        throw new Error(response.error || 'Failed to fetch dashboard')
      }

      // Transform API response shape to DashboardStats
      const raw: unknown = response.data
      if (isRawDashboardResponse(raw)) {
        const pnl = raw.pnl_summary || {}
        const pos = raw.position_summary || {}

        const transformed: DashboardStats = {
          totalPnl: pnl.total_pnl ?? 0,
          totalPnlPercent: pnl.return_percent ?? 0,
          winRate: pnl.win_rate ?? 0,
          activePositions: pos.open_positions ?? 0,
          totalTrades: pnl.total_trades ?? 0,
          equity: pnl.current_capital ?? 0,
          availableBalance: (pnl.current_capital ?? 0) - (pos.total_exposure ?? 0),
          marginUsed: pos.total_exposure ?? 0,
          marginAvailable: (pnl.current_capital ?? 0) - (pos.total_exposure ?? 0),
        }

        return {
          success: true as const,
          data: transformed,
          timestamp: response.timestamp,
        }
      }

      return response
    },
    staleTime: REFRESH_INTERVALS.dashboard,
    refetchInterval: REFRESH_INTERVALS.dashboard,
  })
}

export function useDashboardPositions() {
  return useQuery({
    queryKey: QUERY_KEYS.dashboardPositions,
    queryFn: async () => {
      if (USE_MOCK_DATA) {
        return {
          success: true as const,
          data: getMockPositionsFromTrades(),
          timestamp: new Date().toISOString(),
        }
      }

      const response = await apiClient.getDashboardPositions()

      if (!response.success) {
        throw new Error(response.error || 'Failed to fetch dashboard positions')
      }

      // API returns {positions: [...], count: N, summary: {...}} - extract the array
      const rawData: unknown = response.data
      if (isWrappedResponse(rawData)) {
        return {
          success: true as const,
          data: rawData.positions as Position[],
          timestamp: response.timestamp,
        }
      }

      return response
    },
    staleTime: REFRESH_INTERVALS.dashboard,
    refetchInterval: REFRESH_INTERVALS.dashboard,
  })
}

export function useDashboardPnl() {
  return useQuery({
    queryKey: QUERY_KEYS.dashboardPnl,
    queryFn: async () => {
      if (USE_MOCK_DATA) {
        return {
          success: true as const,
          data: {
            daily: 234.56,
            total: 12547.89,
            equity: [],
          },
          timestamp: new Date().toISOString(),
        }
      }

      const response = await apiClient.getDashboardPnl()

      if (!response.success) {
        throw new Error(response.error || 'Failed to fetch dashboard PnL')
      }

      // Transform API response: API returns flat {total_pnl, realized_pnl, ...}
      const raw: unknown = response.data
      if (isRawDashboardPnlResponse(raw)) {
        return {
          success: true as const,
          data: {
            daily: raw.realized_pnl ?? 0,
            total: raw.total_pnl ?? 0,
            equity: [],
          },
          timestamp: response.timestamp,
        }
      }

      return response
    },
    staleTime: REFRESH_INTERVALS.dashboard,
    refetchInterval: REFRESH_INTERVALS.dashboard,
  })
}

// Unified Portfolio
export function useUnifiedPortfolio() {
  return useQuery({
    queryKey: ['unified-portfolio'],
    queryFn: async () => {
      if (USE_MOCK_DATA) {
        return {
          success: true as const,
          data: getMockUnifiedPortfolio(),
          timestamp: new Date().toISOString(),
        }
      }

      const response = await apiClient.getUnifiedPortfolio()
      if (response.success) {
        return response
      }
      throw new Error(response.error || 'Failed to fetch unified portfolio')
    },
    staleTime: REFRESH_INTERVALS.dashboard,
    refetchInterval: REFRESH_INTERVALS.dashboard,
  })
}

// Mutations
export function useCreateOrder() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (order: Partial<Order>) => apiClient.createOrder(order),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.orders })
    },
  })
}

export function useDeleteOrder() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: string) => apiClient.deleteOrder(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.orders })
    },
  })
}

// Trading Controls
export function useStartTrading() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: () => apiClient.startTrading(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.dashboard })
    },
  })
}

export function useStopTrading() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: () => apiClient.stopTrading(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.dashboard })
    },
  })
}

export function usePauseTrading() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: () => apiClient.pauseTrading(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.dashboard })
    },
  })
}

export function useResumeTrading() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: () => apiClient.resumeTrading(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.dashboard })
    },
  })
}

export function useSystemStatus() {
  return useQuery({
    queryKey: ['systemStatus'],
    queryFn: () => apiClient.getStatus(),
    refetchInterval: REFRESH_INTERVALS.dashboard,
  })
}
