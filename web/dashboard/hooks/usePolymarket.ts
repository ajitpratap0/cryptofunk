'use client'

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '@/lib/api'
import { REFRESH_INTERVALS } from '@/lib/constants'
import type {
  PolymarketPortfolio,
  PolymarketPosition,
  PolymarketTrade,
  PolymarketMarket,
  PolymarketPrediction,
  PolymarketPerformance,
  PaperTradeRequest,
} from '@/lib/types'

export const POLYMARKET_KEYS = {
  portfolio: ['polymarket', 'portfolio'],
  positions: ['polymarket', 'positions'],
  trades: ['polymarket', 'trades'],
  markets: ['polymarket', 'markets'],
  predictions: ['polymarket', 'predictions'],
  performance: ['polymarket', 'performance'],
} as const

// Mock data for fallback
const mockPortfolio: PolymarketPortfolio = {
  balance: 85.50,
  total_value: 14.50,
  cost_basis: 14.50,
  unrealized_pnl: 0,
  position_count: 0,
}

const mockMarkets: PolymarketMarket[] = [
  {
    id: 'mock-1',
    question: 'Will BTC exceed $100k by March 2026?',
    category: 'crypto',
    yes_price: 0.72,
    no_price: 0.28,
    volume: 125000,
    end_date: '2026-03-31T00:00:00Z',
    active: true,
    updated_at: new Date().toISOString(),
  },
  {
    id: 'mock-2',
    question: 'Will the Fed cut rates in Q1 2026?',
    category: 'politics',
    yes_price: 0.45,
    no_price: 0.55,
    volume: 89000,
    end_date: '2026-03-31T00:00:00Z',
    active: true,
    updated_at: new Date().toISOString(),
  },
  {
    id: 'mock-3',
    question: 'Will ETH flip BTC market cap in 2026?',
    category: 'crypto',
    yes_price: 0.12,
    no_price: 0.88,
    volume: 67000,
    end_date: '2026-12-31T00:00:00Z',
    active: true,
    updated_at: new Date().toISOString(),
  },
  {
    id: 'mock-4',
    question: 'Will US approve a spot SOL ETF by June 2026?',
    category: 'regulation',
    yes_price: 0.35,
    no_price: 0.65,
    volume: 45000,
    end_date: '2026-06-30T00:00:00Z',
    active: true,
    updated_at: new Date().toISOString(),
  },
  {
    id: 'mock-5',
    question: 'Will total crypto market cap exceed $5T in 2026?',
    category: 'crypto',
    yes_price: 0.58,
    no_price: 0.42,
    volume: 203000,
    end_date: '2026-12-31T00:00:00Z',
    active: true,
    updated_at: new Date().toISOString(),
  },
]

export function usePolymarketPortfolio() {
  return useQuery({
    queryKey: POLYMARKET_KEYS.portfolio,
    queryFn: async () => {
      try {
        const response = await apiClient.getPolymarketPortfolio()
        if (response.success) return response.data
      } catch { /* fallback */ }
      return mockPortfolio
    },
    staleTime: REFRESH_INTERVALS.dashboard,
    refetchInterval: REFRESH_INTERVALS.dashboard,
  })
}

export function usePolymarketPositions() {
  return useQuery({
    queryKey: POLYMARKET_KEYS.positions,
    queryFn: async () => {
      try {
        const response = await apiClient.getPolymarketPositions(true)
        if (response.success) return response.data.positions
      } catch { /* fallback */ }
      return [] as PolymarketPosition[]
    },
    staleTime: REFRESH_INTERVALS.positions,
    refetchInterval: REFRESH_INTERVALS.positions,
  })
}

export function usePolymarketTrades() {
  return useQuery({
    queryKey: POLYMARKET_KEYS.trades,
    queryFn: async () => {
      try {
        const response = await apiClient.getPolymarketTrades(100)
        if (response.success) return response.data.trades
      } catch { /* fallback */ }
      return [] as PolymarketTrade[]
    },
    staleTime: REFRESH_INTERVALS.trades,
    refetchInterval: REFRESH_INTERVALS.trades,
  })
}

export function usePolymarketMarkets() {
  return useQuery({
    queryKey: POLYMARKET_KEYS.markets,
    queryFn: async () => {
      try {
        const response = await apiClient.getPolymarketMarkets(true)
        if (response.success) return response.data.markets
      } catch { /* fallback */ }
      return mockMarkets
    },
    staleTime: REFRESH_INTERVALS.agents,
    refetchInterval: REFRESH_INTERVALS.agents,
  })
}

export function usePolymarketPredictions() {
  return useQuery({
    queryKey: POLYMARKET_KEYS.predictions,
    queryFn: async () => {
      try {
        const response = await apiClient.getPolymarketPredictions()
        if (response.success) return response.data.predictions
      } catch { /* fallback */ }
      return [] as PolymarketPrediction[]
    },
    staleTime: REFRESH_INTERVALS.performance,
    refetchInterval: REFRESH_INTERVALS.performance,
  })
}

export function usePolymarketPerformance() {
  return useQuery({
    queryKey: POLYMARKET_KEYS.performance,
    queryFn: async () => {
      try {
        const response = await apiClient.getPolymarketPerformance()
        if (response.success) return response.data
      } catch { /* fallback */ }
      return {
        total_predictions: 0,
        resolved: 0,
        correct: 0,
        win_rate: 0,
        brier_score: 0,
        total_pnl: 0,
        by_category: {},
      } as PolymarketPerformance
    },
    staleTime: REFRESH_INTERVALS.performance,
    refetchInterval: REFRESH_INTERVALS.performance,
  })
}

export function usePaperTrade() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (trade: PaperTradeRequest) => apiClient.executePaperTrade(trade),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['polymarket'] })
    },
  })
}
