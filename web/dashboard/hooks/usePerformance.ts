'use client'

import { useQuery } from '@tanstack/react-query'
import { apiClient } from '@/lib/api'
import type { RawPerformanceSummary } from '@/lib/api'
import { REFRESH_INTERVALS } from '@/lib/constants'
import type {
  PerformanceMetrics,
  EquityPoint,
  CandlestickData,
  TimeRange,
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
      const response = await apiClient.getPerformanceSummary()

      if (!response.success) {
        throw new Error(response.error || 'Failed to fetch performance metrics')
      }

      const raw = response.data as unknown as RawPerformanceSummary

      const metrics: PerformanceMetrics = {
        sharpeRatio: raw?.sharpe_ratio ?? 0,
        sortinoRatio: raw?.sortino_ratio ?? 0,
        maxDrawdown: raw?.max_drawdown ?? 0,
        maxDrawdownPercent: raw?.max_drawdown_percent ?? 0,
        calmarRatio: raw?.calmar_ratio ?? 0,
        winRate: raw?.win_rate ?? 0,
        avgWin: raw?.avg_win ?? 0,
        avgLoss: raw?.avg_loss ?? 0,
        profitFactor: raw?.profit_factor ?? 0,
        totalReturn: raw?.total_return ?? 0,
      }

      return {
        success: true as const,
        data: metrics,
        timestamp: response.timestamp,
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
      const response = await apiClient.getDashboardPnl(timeRange)

      if (!response.success) {
        throw new Error(response.error || 'Failed to load equity history')
      }

      // API may return equity curve; fall back to empty array (no mock data)
      const equity: EquityPoint[] =
        response.data && Array.isArray((response.data as { equity?: EquityPoint[] }).equity)
          ? (response.data as { equity: EquityPoint[] }).equity
          : []

      return {
        success: true as const,
        data: equity,
        timestamp: new Date().toISOString(),
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
// The /market/candlestick endpoint is not yet implemented server-side
// (TODO: track via backend issue). When the backend adds it, this hook
// works automatically via apiClient. Errors are surfaced to the caller —
// the UI is responsible for rendering an empty / error state.
//
// For local development against a stub backend, set
// NEXT_PUBLIC_USE_MOCK_CANDLES=true to fall back to generated data. The
// mock fallback is logged so it cannot silently mask real API failures.
export function useCandlestickData(symbol: string, timeRange: string = '1d') {
  return useQuery({
    queryKey: PERFORMANCE_QUERY_KEYS.candlestick(symbol, timeRange),
    queryFn: async () => {
      const response = await apiClient.getMarketCandlestick(symbol, timeRange)
      if (response.success) {
        return response
      }

      const useMock = process.env.NEXT_PUBLIC_USE_MOCK_CANDLES === 'true'
      if (useMock) {
        console.warn(
          `[useCandlestickData] candlestick fetch failed for ${symbol} (${timeRange}); ` +
            `serving NEXT_PUBLIC_USE_MOCK_CANDLES fallback. error=${response.error ?? 'unknown'}`
        )
        return {
          success: true as const,
          data: generateMockCandlestickData(symbol, timeRange),
          timestamp: new Date().toISOString(),
          isMock: true,
        }
      }

      throw new Error(response.error || 'Failed to fetch candlestick data')
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

// Risk Alert shape returned by the API (derived from circuit breaker + metrics state)
export interface RiskAlert {
  id: string
  severity: 'warning' | 'info' | 'resolved'
  message: string
  timestamp: string
  asset: string
}

interface RawRiskAlert {
  id?: string
  severity?: string
  message?: string
  timestamp?: string
  asset?: string
}

// Risk Alerts — fetched from /api/v1/risk/metrics alerts field.
// The backend may not yet expose a dedicated alerts array; in that case the
// hook returns an empty list so the UI shows nothing rather than stale mocks.
// TODO: replace with a dedicated /api/v1/risk/alerts endpoint when available.
export function useRiskAlerts() {
  const query = useQuery({
    queryKey: ['risk', 'alerts'],
    queryFn: async (): Promise<RiskAlert[]> => {
      const response = await apiClient.getRiskMetrics()
      if (!response.success) {
        throw new Error(response.error || 'Failed to fetch risk alerts')
      }
      const raw = response.data as unknown as { alerts?: RawRiskAlert[] }
      if (!raw?.alerts || !Array.isArray(raw.alerts)) {
        return []
      }
      return raw.alerts.map((a, i) => ({
        id: a.id ?? String(i),
        severity: (a.severity === 'warning' || a.severity === 'resolved' ? a.severity : 'info') as RiskAlert['severity'],
        message: a.message ?? '',
        timestamp: a.timestamp ?? '',
        asset: a.asset ?? 'Portfolio',
      }))
    },
    staleTime: REFRESH_INTERVALS.risk,
    refetchInterval: REFRESH_INTERVALS.risk,
  })

  return {
    ...query,
    // Convenience: data defaults to [] so callers can spread without null checks.
    data: query.data ?? [],
    isError: query.isError,
  }
}

// Helper Functions

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