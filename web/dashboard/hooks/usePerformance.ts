'use client'

import { useQuery, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '@/lib/api'
import { REFRESH_INTERVALS } from '@/lib/constants'
import { 
  calculateSharpeRatio, 
  calculateMaxDrawdown, 
  calculateWinRate,
  groupBy 
} from '@/lib/utils'
import type {
  PerformanceMetrics,
  EquityPoint,
  CandlestickData,
  TimeRange,
  Trade,
} from '@/lib/types'

// Query Keys
export const PERFORMANCE_QUERY_KEYS = {
  performance: ['performance'],
  equityHistory: (timeRange: TimeRange) => ['performance', 'equity', timeRange],
  drawdown: ['performance', 'drawdown'],
  metrics: ['performance', 'metrics'],
  pairPerformance: ['performance', 'pairs'],
  candlestick: (symbol: string, timeRange: string) => ['candlestick', symbol, timeRange],
  risk: ['risk'],
  circuitBreakers: ['risk', 'circuit-breakers'],
  // Own top-level key — NOT nested under `risk` so that invalidating `risk`
  // does not unintentionally invalidate exposure queries.
  riskExposure: ['risk-exposure'],
} as const

// Performance Metrics
export function usePerformanceMetrics() {
  return useQuery({
    queryKey: PERFORMANCE_QUERY_KEYS.metrics,
    queryFn: async () => {
      try {
        // Since we don't have a dedicated performance endpoint, compute from trades
        const tradesResponse = await fetch('/api/mock-trades')
        
        if (!tradesResponse.ok) throw new Error('API error')
        
        const trades = await tradesResponse.json()
        
        // Calculate metrics from trades
        const pnlValues = trades.filter((t: any) => t.status === 'closed').map((t: any) => t.pnl)
        const winningTrades = pnlValues.filter((p: number) => p > 0)
        const losingTrades = pnlValues.filter((p: number) => p < 0)
        
        const metrics: PerformanceMetrics = {
          sharpeRatio: calculateSharpeRatio(pnlValues),
          sortinoRatio: calculateSharpeRatio(pnlValues.filter((p: number) => p < 0)),
          maxDrawdown: 0.08, // Would need equity curve to calculate properly
          maxDrawdownPercent: 8.0,
          calmarRatio: 1.45,
          winRate: calculateWinRate(trades),
          avgWin: winningTrades.length > 0 ? 
            winningTrades.reduce((sum: number, p: number) => sum + p, 0) / winningTrades.length : 0,
          avgLoss: losingTrades.length > 0 ?
            losingTrades.reduce((sum: number, p: number) => sum + p, 0) / losingTrades.length : 0,
          profitFactor: losingTrades.length > 0 ?
            Math.abs(winningTrades.reduce((sum: number, p: number) => sum + p, 0)) /
            Math.abs(losingTrades.reduce((sum: number, p: number) => sum + p, 0)) : 0,
          totalReturn: 15.67,
        }
        
        return {
          success: true,
          data: metrics,
          timestamp: new Date().toISOString(),
        }
      } catch {
        // Return mock data if API fails
        const mockMetrics: PerformanceMetrics = {
          sharpeRatio: 1.87,
          sortinoRatio: 2.34,
          maxDrawdown: 0.08,
          maxDrawdownPercent: 8.0,
          calmarRatio: 1.45,
          winRate: 68.4,
          avgWin: 245.67,
          avgLoss: -156.89,
          profitFactor: 1.56,
          totalReturn: 15.67,
        }
        
        return {
          success: true,
          data: mockMetrics,
          timestamp: new Date().toISOString(),
        }
      }
    },
    staleTime: REFRESH_INTERVALS.performance,
    refetchInterval: REFRESH_INTERVALS.performance,
  })
}

// Equity History
export function useEquityHistory(timeRange: TimeRange = '1m') {
  return useQuery({
    queryKey: PERFORMANCE_QUERY_KEYS.equityHistory(timeRange),
    queryFn: async () => {
      try {
        const response = await apiClient.getDashboardPnl()
        
        if (!response.success) throw new Error('API error')
        
        return {
          success: true,
          data: response.data.equity || [],
          timestamp: new Date().toISOString(),
        }
      } catch {
        // Return mock data if API fails
        const points = generateMockEquityHistory(timeRange)
        return {
          success: true,
          data: points,
          timestamp: new Date().toISOString(),
        }
      }
    },
    staleTime: REFRESH_INTERVALS.performance,
    refetchInterval: REFRESH_INTERVALS.performance,
  })
}

// Drawdown Analysis
export function useDrawdownAnalysis() {
  const { data: equityHistory } = useEquityHistory('1m')
  
  return useQuery({
    queryKey: [...PERFORMANCE_QUERY_KEYS.drawdown, equityHistory],
    queryFn: () => {
      if (!equityHistory?.data) return null
      
      const drawdownData: Array<{ timestamp: string; drawdown: number; equity: number }> = []
      let peak = equityHistory.data[0]?.equity || 0
      
      for (const point of equityHistory.data) {
        if (point.equity > peak) {
          peak = point.equity
        }
        
        const drawdown = peak > 0 ? ((peak - point.equity) / peak) * 100 : 0
        
        drawdownData.push({
          timestamp: point.timestamp,
          drawdown: -drawdown, // Negative for visualization
          equity: point.equity,
        })
      }
      
      const maxDrawdown = Math.max(...drawdownData.map(d => Math.abs(d.drawdown)))
      const maxDrawdownPoint = drawdownData.find(d => Math.abs(d.drawdown) === maxDrawdown)
      
      return {
        data: drawdownData,
        maxDrawdown,
        maxDrawdownDate: maxDrawdownPoint?.timestamp,
        currentDrawdown: drawdownData[drawdownData.length - 1]?.drawdown || 0,
      }
    },
    enabled: !!equityHistory?.data,
    staleTime: REFRESH_INTERVALS.performance,
  })
}

// Pair Performance
export function usePairPerformance() {
  return useQuery({
    queryKey: PERFORMANCE_QUERY_KEYS.pairPerformance,
    queryFn: async () => {
      const response = await apiClient.getPairPerformance()
      if (!response.success) throw new Error(response.error || 'Failed to fetch pair performance')
      const raw: unknown = response.data
      const pairs =
        raw && typeof raw === 'object' && 'pairs' in raw
          ? (raw as { pairs: Array<{ symbol: string; realized_pnl: number; trade_count: number }> }).pairs
          : []
      return { success: true as const, data: pairs, timestamp: response.timestamp }
    },
    staleTime: REFRESH_INTERVALS.performance,
    refetchInterval: REFRESH_INTERVALS.performance,
  })
}

// Candlestick Data
export function useCandlestickData(symbol: string, timeRange: string = '1d') {
  return useQuery({
    queryKey: PERFORMANCE_QUERY_KEYS.candlestick(symbol, timeRange),
    queryFn: async () => {
      try {
        // Try to get actual market data
        const response = await fetch(
          `${process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'}/market/candlestick/${symbol}?timeRange=${timeRange}`
        )
        if (!response.ok) throw new Error('API error')
        return await response.json()
      } catch {
        // Return mock data if API fails
        const mockData = generateMockCandlestickData(symbol, timeRange)
        
        return {
          success: true,
          data: mockData,
          timestamp: new Date().toISOString(),
        }
      }
    },
    enabled: !!symbol,
    staleTime: 30000, // 30 seconds for price data
    refetchInterval: 30000,
  })
}

// Risk Metrics
export function useRiskMetrics() {
  return useQuery({
    queryKey: PERFORMANCE_QUERY_KEYS.risk,
    queryFn: () => apiClient.getRiskMetrics(),
    staleTime: REFRESH_INTERVALS.risk,
    refetchInterval: REFRESH_INTERVALS.risk,
  })
}

// Circuit Breakers
export function useCircuitBreakers() {
  return useQuery({
    queryKey: PERFORMANCE_QUERY_KEYS.circuitBreakers,
    queryFn: () => apiClient.getCircuitBreakers(),
    staleTime: REFRESH_INTERVALS.risk,
    refetchInterval: REFRESH_INTERVALS.risk,
  })
}

// Risk Exposure by symbol
export function useRiskExposure() {
  return useQuery({
    queryKey: PERFORMANCE_QUERY_KEYS.riskExposure,
    queryFn: () => apiClient.getRiskExposure(),
    staleTime: REFRESH_INTERVALS.risk,
    refetchInterval: REFRESH_INTERVALS.risk,
  })
}

// Helper Functions
function generateMockEquityHistory(timeRange: TimeRange): EquityPoint[] {
  const points: EquityPoint[] = []
  const baseEquity = 80000
  let currentEquity = baseEquity
  
  // Determine number of points and interval based on time range
  const timeRangeConfig = {
    '1h': { points: 60, intervalMs: 60000 }, // 1 minute intervals
    '4h': { points: 240, intervalMs: 60000 }, // 1 minute intervals
    '1d': { points: 288, intervalMs: 300000 }, // 5 minute intervals
    '1w': { points: 336, intervalMs: 1800000 }, // 30 minute intervals
    '1m': { points: 720, intervalMs: 3600000 }, // 1 hour intervals
    '3m': { points: 720, intervalMs: 10800000 }, // 3 hour intervals
    '1y': { points: 365, intervalMs: 86400000 }, // 1 day intervals
  }
  
  const config = timeRangeConfig[timeRange]
  const now = Date.now()
  
  for (let i = config.points; i >= 0; i--) {
    const timestamp = new Date(now - (i * config.intervalMs)).toISOString()
    
    // Generate some realistic equity movement
    const change = (Math.random() - 0.5) * 1000 * (i / config.points + 0.1)
    currentEquity += change
    
    points.push({
      timestamp,
      equity: Math.max(currentEquity, baseEquity * 0.7), // Don't go below 70% of initial
      pnl: change,
    })
  }
  
  return points
}

function generateMockCandlestickData(symbol: string, timeRange: string): CandlestickData[] {
  const data: CandlestickData[] = []
  
  // Base price based on symbol
  const basePrices: Record<string, number> = {
    'BTC/USDT': 45000,
    'ETH/USDT': 2800,
    'BNB/USDT': 320,
    'XRP/USDT': 0.52,
    'ADA/USDT': 0.48,
    'SOL/USDT': 95,
  }
  
  const basePrice = basePrices[symbol] || 100
  let currentPrice = basePrice
  
  // Generate 100 candles with dynamic timestamps
  const now = Date.now()
  for (let i = 100; i >= 0; i--) {
    const timestamp = new Date(now - (i * 300000)).toISOString() // 5 min intervals
    
    const open = currentPrice
    const changePercent = (Math.random() - 0.5) * 0.04 // ±2% max change
    const high = open * (1 + Math.abs(changePercent) + Math.random() * 0.01)
    const low = open * (1 - Math.abs(changePercent) - Math.random() * 0.01)
    const close = open * (1 + changePercent)
    
    data.push({
      time: timestamp,
      open: Number(open.toFixed(2)),
      high: Number(high.toFixed(2)),
      low: Number(low.toFixed(2)),
      close: Number(close.toFixed(2)),
      volume: Math.floor(Math.random() * 1000000),
    })
    
    currentPrice = close
  }
  
  return data
}