'use client'

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '@/lib/api'
import { REFRESH_INTERVALS } from '@/lib/constants'
import type { Trade, Position, Order, TradeFilters } from '@/lib/types'

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
export function useTrades(filters?: TradeFilters) {
  return useQuery({
    queryKey: [...QUERY_KEYS.trades, filters],
    queryFn: async () => {
      // For now, use mock data as the API doesn't have a trades endpoint yet
      const mockTrades = apiClient.getMockTrades()
      return {
        success: true,
        data: mockTrades,
        timestamp: new Date().toISOString(),
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
      // Mock single trade data
      const trades = apiClient.getMockTrades()
      const trade = trades.find(t => t.id === id)
      return {
        success: true,
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
      const response = await apiClient.getPositions()
      
      // Fallback to mock data if API fails
      if (!response.success) {
        const mockTrades = apiClient.getMockTrades()
        const mockPositions = mockTrades
          .filter(trade => trade.status === 'open')
          .map(trade => ({
            symbol: trade.symbol,
            side: trade.side,
            size: trade.quantity,
            entryPrice: trade.entryPrice,
            markPrice: trade.currentPrice,
            unrealizedPnl: trade.pnl,
            unrealizedPnlPercent: trade.pnlPercent,
            marginUsed: trade.entryPrice * trade.quantity * 0.1, // Assume 10x leverage
            timestamp: trade.timestamp,
          }))
        
        return {
          success: true,
          data: mockPositions,
          timestamp: new Date().toISOString(),
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
      const response = await apiClient.getOrders()
      
      // Fallback to mock data if API fails
      if (!response.success) {
        const mockOrders: Order[] = [
          {
            id: '1',
            symbol: 'BTC/USDT',
            side: 'buy',
            type: 'limit',
            quantity: 0.1,
            price: 45000,
            status: 'pending',
            timestamp: new Date().toISOString(),
            agent: 'momentum_agent',
          },
          {
            id: '2',
            symbol: 'ETH/USDT',
            side: 'sell',
            type: 'stop',
            quantity: 1.0,
            stopPrice: 2800,
            status: 'pending',
            timestamp: new Date().toISOString(),
            agent: 'risk_manager',
          },
        ]
        
        return {
          success: true,
          data: mockOrders,
          timestamp: new Date().toISOString(),
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
      const response = await apiClient.getDashboard()
      
      // Fallback to mock data if API fails
      if (!response.success) {
        return {
          success: true,
          data: apiClient.getMockDashboard(),
          timestamp: new Date().toISOString(),
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
      const response = await apiClient.getDashboardPositions()
      
      // Fallback to mock data if API fails
      if (!response.success) {
        const mockTrades = apiClient.getMockTrades()
        const mockPositions = mockTrades
          .filter(trade => trade.status === 'open')
          .map(trade => ({
            symbol: trade.symbol,
            side: trade.side,
            size: trade.quantity,
            entryPrice: trade.entryPrice,
            markPrice: trade.currentPrice,
            unrealizedPnl: trade.pnl,
            unrealizedPnlPercent: trade.pnlPercent,
            marginUsed: trade.entryPrice * trade.quantity * 0.1,
            timestamp: trade.timestamp,
          }))
        
        return {
          success: true,
          data: mockPositions,
          timestamp: new Date().toISOString(),
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
      const response = await apiClient.getDashboardPnl()
      
      // Fallback to mock data if API fails
      if (!response.success) {
        return {
          success: true,
          data: {
            daily: 234.56,
            total: 12547.89,
            equity: apiClient.getMockEquityPoints(),
          },
          timestamp: new Date().toISOString(),
        }
      }
      
      return response
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