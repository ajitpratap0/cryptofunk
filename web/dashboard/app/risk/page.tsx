'use client'

import { useState, useMemo } from 'react'
import {
  Shield,
  TrendingDown,
  Activity,
  RefreshCw,
  BarChart3,
  Zap
} from 'lucide-react'
import { ExposurePie } from '@/components/charts/ExposurePie'
import { DrawdownChart } from '@/components/charts/DrawdownChart'
import { CircuitBreakerStatus } from '@/components/risk/CircuitBreakerStatus'
import { StatCard } from '@/components/ui/StatCard'
import { cn, formatCurrency, formatPercentage } from '@/lib/utils'
import { useRiskMetrics, useCircuitBreakers, useRiskExposure } from '@/hooks/usePerformance'
import type { CircuitBreaker } from '@/lib/types'

// ── Static Mock Data (no API equivalent yet) ───────────────────────

// TODO: replace with real data from API
const MOCK_DRAWDOWN_NOISE = [
  0.05, 0.12, 0.08, 0.19, 0.03, 0.14, 0.07, 0.17, 0.02, 0.11,
  0.06, 0.15, 0.09, 0.18, 0.04, 0.13, 0.01, 0.16, 0.10, 0.20,
  0.05, 0.12, 0.08, 0.19, 0.03, 0.14, 0.07, 0.17, 0.02, 0.11,
  0.06, 0.15, 0.09, 0.18, 0.04, 0.13, 0.01, 0.16, 0.10, 0.20,
  0.05, 0.12, 0.08, 0.19, 0.03, 0.14, 0.07, 0.17, 0.02, 0.11,
  0.06, 0.15, 0.09, 0.18, 0.04, 0.13, 0.01, 0.16, 0.10, 0.20,
  0.05, 0.12, 0.08, 0.19, 0.03, 0.14, 0.07, 0.17, 0.02, 0.11,
  0.06, 0.15, 0.09, 0.18, 0.04, 0.13, 0.01, 0.16, 0.10, 0.20,
  0.05, 0.12, 0.08, 0.19, 0.03, 0.14, 0.07, 0.17, 0.02, 0.11,
]
const mockAlerts = [
  { id: '1', severity: 'warning' as const, message: 'Max drawdown approaching threshold (78%)', timestamp: '2 min ago', asset: 'Portfolio' },
  { id: '2', severity: 'info' as const, message: 'ETH/USDT correlation with BTC increasing', timestamp: '15 min ago', asset: 'ETH/USDT' },
  { id: '3', severity: 'info' as const, message: 'Position rebalancing recommended', timestamp: '1 hr ago', asset: 'Portfolio' },
  { id: '4', severity: 'resolved' as const, message: 'Daily loss limit recovered to safe zone', timestamp: '3 hrs ago', asset: 'Portfolio' },
  { id: '5', severity: 'warning' as const, message: 'SOL/USDT volatility spike detected', timestamp: '5 hrs ago', asset: 'SOL/USDT' },
]

// ── Runtime Type Guards ─────────────────────────────────────────────

function isRiskMetricsData(data: unknown): data is { var_95: number | null; var_99: number | null; expected_shortfall: number | null; open_positions: number; total_exposure: number } {
  return typeof data === 'object' && data !== null && 'var_95' in data
}

function isCircuitBreakersData(data: unknown): data is { circuit_breakers: Array<{ name: string; current: number; threshold: number; status: string }> } {
  return typeof data === 'object' && data !== null && 'circuit_breakers' in data
}

function isExposureData(data: unknown): data is { exposure: Array<{ symbol: string; exposure: number }> } {
  return typeof data === 'object' && data !== null && 'exposure' in data
}

// ── Page Component ─────────────────────────────────────────────────

export default function RiskPage() {
  const [timeRange, setTimeRange] = useState<'1w' | '1m' | '3m'>('1m')

  const { data: metricsResponse, isLoading: metricsLoading, isError: metricsError, refetch: refetchMetrics } = useRiskMetrics()
  const { data: breakersResponse, isLoading: breakersLoading, isError: breakersError, refetch: refetchBreakers } = useCircuitBreakers()
  const { data: exposureResponse, isLoading: exposureLoading, isError: exposureError, refetch: refetchExposure } = useRiskExposure()

  const mockDrawdown = useMemo(() => Array.from({ length: 90 }, (_, i) => {
    const now = Date.now()
    const timestamp = new Date(now - (89 - i) * 24 * 60 * 60 * 1000).toISOString()
    const base = Math.sin(i / 15) * 4 + MOCK_DRAWDOWN_NOISE[i % MOCK_DRAWDOWN_NOISE.length]
    const drawdownPercent = -Math.abs(base)
    return {
      timestamp,
      drawdown: drawdownPercent,
      equity: 100000 + (drawdownPercent / 100) * 100000,
    }
  }), [])

  const handleRefresh = () => {
    refetchMetrics()
    refetchBreakers()
    refetchExposure()
  }

  if (metricsLoading || breakersLoading || exposureLoading) {
    return (
      <div className="p-6 text-muted-foreground">
        Loading risk data...
      </div>
    )
  }

  if (metricsError || breakersError || exposureError) {
    return (
      <div className="p-6 text-red-500">
        Failed to load risk data. Please check the API connection and try refreshing.
      </div>
    )
  }

  // Map API risk metrics
  const raw_metrics = metricsResponse?.data
  if (raw_metrics !== undefined && !isRiskMetricsData(raw_metrics)) {
    console.warn('risk/metrics: unexpected API shape', raw_metrics)
  }
  const apiMetrics = isRiskMetricsData(raw_metrics) ? raw_metrics : undefined

  // Map API circuit breakers to component shape
  const circuitBreakers: CircuitBreaker[] = useMemo(() => {
    const raw = breakersResponse?.data
    if (raw !== undefined && !isCircuitBreakersData(raw)) {
      console.warn('risk/circuit-breakers: unexpected API shape', raw)
      return []
    }
    if (!isCircuitBreakersData(raw) || !raw.circuit_breakers?.length) return []
    return raw.circuit_breakers.map(cb => ({
      name: cb.name,
      status: cb.status === 'TRIGGERED' ? 'triggered' : cb.status === 'WARNING' ? 'warning' : cb.status === 'DISABLED' ? 'disabled' : 'normal',
      threshold: cb.threshold,
      currentValue: cb.current,
      description: cb.name,
    } as CircuitBreaker))
  }, [breakersResponse])

  // Map API exposure data
  const exposureData = useMemo(() => {
    const raw = exposureResponse?.data
    if (raw !== undefined && !isExposureData(raw)) {
      console.warn('risk/exposure: unexpected API shape', raw)
      return []
    }
    if (!isExposureData(raw) || !raw.exposure?.length) return []
    const total = raw.exposure.reduce((sum, e) => sum + e.exposure, 0)
    return raw.exposure.map(e => ({
      symbol: e.symbol,
      exposure: e.exposure,
      percentage: total > 0 ? Math.round((e.exposure / total) * 100) : 0,
      value: e.exposure,
    }))
  }, [exposureResponse])

  const var95 = apiMetrics?.var_95 ?? null
  const var99 = apiMetrics?.var_99 ?? null

  const riskScore = useMemo<number | null>(() => {
    if (!circuitBreakers.length) return null
    let score = 100
    circuitBreakers.forEach(cb => {
      if (cb.status === 'disabled') return
      const usage = cb.threshold > 0 ? Math.abs(cb.currentValue) / Math.abs(cb.threshold) : 0
      if (cb.status === 'triggered') score -= 30
      else if (cb.status === 'warning') score -= 15
      else if (usage > 0.5) score -= 5
    })
    return Math.max(0, Math.min(100, score))
  }, [circuitBreakers])

  const riskLevel = riskScore == null ? 'Unknown' : riskScore >= 80 ? 'Low' : riskScore >= 50 ? 'Medium' : 'High'
  const riskColor = riskScore == null ? 'text-muted-foreground' : riskScore >= 80 ? 'text-profit' : riskScore >= 50 ? 'text-warning' : 'text-loss'

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Risk Dashboard</h1>
          <p className="text-muted-foreground text-sm mt-1">
            Portfolio risk monitoring & circuit breaker status
          </p>
        </div>
        <button
          onClick={handleRefresh}
          className="flex items-center gap-2 px-3 py-2 bg-card border border-border rounded-lg hover:bg-muted/50 transition-colors text-sm"
        >
          <RefreshCw className="h-4 w-4" />
          Refresh
        </button>
      </div>

      {/* Top-level Risk Summary Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          title="Risk Score"
          value={riskScore != null ? `${riskScore}/100` : '—'}
          subtitle={riskLevel}
          icon={<Shield className="h-5 w-5" />}
          valueClassName={riskColor}
        />
        <StatCard
          title="Value at Risk (95%)"
          value={var95 != null ? formatCurrency(var95) : '—'}
          subtitle={var99 != null ? `99%: ${formatCurrency(var99)}` : 'Insufficient data'}
          icon={<TrendingDown className="h-5 w-5" />}
          valueClassName={var95 != null && Math.abs(var95) > 500 ? 'text-warning' : 'text-foreground'}
        />
        <StatCard
          title="Open Positions"
          value={apiMetrics?.open_positions?.toString() ?? '—'}
          subtitle={`Exposure: ${apiMetrics?.total_exposure != null ? formatCurrency(apiMetrics.total_exposure) : '—'}`}
          icon={<Activity className="h-5 w-5" />}
        />
        <StatCard
          title="Total Exposure"
          value={apiMetrics?.total_exposure != null ? formatCurrency(apiMetrics.total_exposure) : '—'}
          subtitle="Across all open positions"
          icon={<BarChart3 className="h-5 w-5" />}
        />
      </div>

      {/* Main Content Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left: Circuit Breakers + Alerts */}
        <div className="lg:col-span-1 space-y-6">
          <CircuitBreakerStatus circuitBreakers={circuitBreakers} />

          {/* Risk Alerts */}
          <div className="bg-card border border-border rounded-lg p-6">
            <div className="flex items-center gap-2 mb-4">
              <Zap className="h-5 w-5" />
              <h3 className="font-semibold">Risk Alerts</h3>
            </div>
            <div className="space-y-3 max-h-[400px] overflow-y-auto">
              {mockAlerts.map(alert => (
                <div
                  key={alert.id}
                  className={cn(
                    'p-3 rounded-lg border text-sm',
                    alert.severity === 'warning' && 'bg-warning/10 border-warning/30',
                    alert.severity === 'info' && 'bg-info/10 border-info/30',
                    alert.severity === 'resolved' && 'bg-profit/10 border-profit/30',
                  )}
                >
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <span className={cn(
                        'text-xs font-medium uppercase',
                        alert.severity === 'warning' && 'text-warning',
                        alert.severity === 'info' && 'text-info',
                        alert.severity === 'resolved' && 'text-profit',
                      )}>
                        {alert.severity}
                      </span>
                      <span className="text-xs text-muted-foreground ml-2">{alert.asset}</span>
                    </div>
                    <span className="text-xs text-muted-foreground whitespace-nowrap">{alert.timestamp}</span>
                  </div>
                  <p className="mt-1 text-muted-foreground">{alert.message}</p>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Right: Charts */}
        <div className="lg:col-span-2 space-y-6">
          {/* Exposure Pie */}
          <div className="bg-card border border-border rounded-lg p-6">
            <ExposurePie
              data={exposureData}
              title="Portfolio Exposure by Asset"
              height={350}
              showLegend
              showLabels
            />
          </div>

          {/* Drawdown Chart */}
          <div className="bg-card border border-border rounded-lg p-6">
            <div className="flex items-center justify-between mb-4">
              <h3 className="font-semibold">Drawdown History</h3>
              <div className="flex gap-1 bg-muted/50 rounded-lg p-1">
                {(['1w', '1m', '3m'] as const).map(range => (
                  <button
                    key={range}
                    onClick={() => setTimeRange(range)}
                    className={cn(
                      'px-3 py-1 text-xs rounded-md transition-colors',
                      timeRange === range
                        ? 'bg-primary text-primary-foreground'
                        : 'text-muted-foreground hover:text-foreground'
                    )}
                  >
                    {range.toUpperCase()}
                  </button>
                ))}
              </div>
            </div>
            <div className="relative">
              <div className="absolute top-2 right-2 z-10 text-xs text-yellow-500 bg-yellow-50 dark:bg-yellow-900/20 px-2 py-0.5 rounded">
                Mock data — real API pending
              </div>
              <DrawdownChart
                data={
                  timeRange === '1w' ? mockDrawdown.slice(-7)
                    : timeRange === '1m' ? mockDrawdown.slice(-30)
                    : mockDrawdown
                }
                height={300}
              />
            </div>
          </div>

          {/* Position Sizing */}
          {exposureData.length > 0 && (
            <div className="bg-card border border-border rounded-lg p-6">
              <h3 className="font-semibold mb-4">Position Sizing Breakdown</h3>
              <div className="space-y-3">
                {exposureData.map(pos => (
                  <div key={pos.symbol} className="flex items-center gap-4">
                    <div className="w-24 text-sm font-medium">{pos.symbol.replace('/USDT', '').replace('USDT', '')}</div>
                    <div className="flex-1">
                      <div className="w-full h-3 bg-muted rounded-full overflow-hidden">
                        <div
                          className="h-full rounded-full transition-all bg-profit"
                          style={{ width: `${pos.percentage}%` }}
                        />
                      </div>
                    </div>
                    <div className="w-20 text-right text-sm font-mono">{formatCurrency(pos.value)}</div>
                    <div className="w-12 text-right text-sm text-muted-foreground">{pos.percentage}%</div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
