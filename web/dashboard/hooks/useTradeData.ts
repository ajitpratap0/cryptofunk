'use client'

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '@/lib/api'
import { REFRESH_INTERVALS } from '@/lib/constants'
import {
  USE_MOCK_DATA,
  getMockTrades,
  getMockDashboard,
  getMockEquityPoints,
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
import type { Trade, Position, Order, UnifiedPortfolio, DashboardStats, EquityPoint } from '@/lib/types'

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
          ? (raw as { trades: Trade[] }).trades
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

export function useTrade(id: string) {
  return useQuery({
    queryKey: [...QUERY_KEYS.trades, id],
    queryFn: async () => {
      const trades = getMockTrades()
      const trade = trades.find(t => t.id === id)
      return {
        success: true as const,
        data: trade || null,
        timestamp: new Date().toISOString(),
      }
    },
    enabled: !!id,
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
            equity: getMockEquityPoints(),
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
            equity: (raw as { equity_curve?: EquityPoint[] }).equity_curve ?? [],
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
