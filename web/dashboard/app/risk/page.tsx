'use client'

import { useState, useMemo } from 'react'
import { 
  Shield, 
  AlertTriangle, 
  TrendingDown, 
  Activity, 
  RefreshCw,
  ChevronDown,
  BarChart3,
  Zap
} from 'lucide-react'
import { ExposurePie } from '@/components/charts/ExposurePie'
import { DrawdownChart } from '@/components/charts/DrawdownChart'
import { CircuitBreakerStatus } from '@/components/risk/CircuitBreakerStatus'
import { StatCard } from '@/components/ui/StatCard'
import { cn, formatCurrency, formatPercentage } from '@/lib/utils'
import { COLORS, RISK_THRESHOLDS } from '@/lib/constants'
import type { CircuitBreaker, RiskMetrics } from '@/lib/types'

// ── Mock Data ──────────────────────────────────────────────────────

const mockRiskMetrics: RiskMetrics = {
  var95: 0.023,
  var99: 0.041,
  expectedShortfall: 0.052,
  betaToMarket: 0.87,
  correlation: {
    'BTC/USDT': 1.0,
    'ETH/USDT': 0.89,
    'SOL/USDT': 0.76,
    'XRP/USDT': 0.64,
    'ADA/USDT': 0.58,
  },
  concentration: {
    'BTC/USDT': 0.35,
    'ETH/USDT': 0.25,
    'SOL/USDT': 0.18,
    'XRP/USDT': 0.12,
    'ADA/USDT': 0.10,
  },
}

const mockCircuitBreakers: CircuitBreaker[] = [
  {
    name: 'Max Daily Loss',
    status: 'normal',
    threshold: 0.05,
    currentValue: 0.018,
    description: 'Maximum allowed daily portfolio loss',
  },
  {
    name: 'Max Drawdown',
    status: 'warning',
    threshold: 0.10,
    currentValue: 0.078,
    description: 'Maximum peak-to-trough drawdown',
  },
  {
    name: 'Position Concentration',
    status: 'normal',
    threshold: 0.50,
    currentValue: 0.35,
    description: 'Maximum single-asset concentration',
  },
  {
    name: 'Leverage Limit',
    status: 'normal',
    threshold: 10,
    currentValue: 3.2,
    description: 'Maximum portfolio leverage ratio',
  },
]

const mockExposure = [
  { symbol: 'BTC/USDT', exposure: 35000, percentage: 35, value: 35000, side: 'long' as const },
  { symbol: 'ETH/USDT', exposure: 25000, percentage: 25, value: 25000, side: 'long' as const },
  { symbol: 'SOL/USDT', exposure: 18000, percentage: 18, value: 18000, side: 'long' as const },
  { symbol: 'XRP/USDT', exposure: 12000, percentage: 12, value: 12000, side: 'short' as const },
  { symbol: 'ADA/USDT', exposure: 10000, percentage: 10, value: 10000, side: 'long' as const },
]

const mockDrawdown = Array.from({ length: 90 }, (_, i) => {
  const now = Date.now()
  const timestamp = new Date(now - (89 - i) * 24 * 60 * 60 * 1000).toISOString()
  const base = Math.sin(i / 15) * 4 + Math.random() * 2 // Generate percentage values (0-6%)
  const drawdownPercent = -Math.abs(base) // Negative percentage value (-6% to 0%)
  return {
    timestamp,
    drawdown: drawdownPercent,
    equity: 100000 + (drawdownPercent / 100) * 100000, // Convert percentage to equity change
  }
})

const mockAlerts = [
  { id: '1', severity: 'warning' as const, message: 'Max drawdown approaching threshold (78%)', timestamp: '2 min ago', asset: 'Portfolio' },
  { id: '2', severity: 'info' as const, message: 'ETH/USDT correlation with BTC increasing', timestamp: '15 min ago', asset: 'ETH/USDT' },
  { id: '3', severity: 'info' as const, message: 'Position rebalancing recommended', timestamp: '1 hr ago', asset: 'Portfolio' },
  { id: '4', severity: 'resolved' as const, message: 'Daily loss limit recovered to safe zone', timestamp: '3 hrs ago', asset: 'Portfolio' },
  { id: '5', severity: 'warning' as const, message: 'SOL/USDT volatility spike detected', timestamp: '5 hrs ago', asset: 'SOL/USDT' },
]

// ── Page Component ─────────────────────────────────────────────────

export default function RiskPage() {
  const [timeRange, setTimeRange] = useState<'1w' | '1m' | '3m'>('1m')

  const riskScore = useMemo(() => {
    let score = 100
    mockCircuitBreakers.forEach(cb => {
      const usage = Math.abs(cb.currentValue) / Math.abs(cb.threshold)
      if (cb.status === 'triggered') score -= 30
      else if (cb.status === 'warning') score -= 15
      else if (usage > 0.5) score -= 5
    })
    return Math.max(0, Math.min(100, score))
  }, [])

  const riskLevel = riskScore >= 80 ? 'Low' : riskScore >= 50 ? 'Medium' : 'High'
  const riskColor = riskScore >= 80 ? 'text-profit' : riskScore >= 50 ? 'text-warning' : 'text-loss'

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
        <button className="flex items-center gap-2 px-3 py-2 bg-card border border-border rounded-lg hover:bg-muted/50 transition-colors text-sm">
          <RefreshCw className="h-4 w-4" />
          Refresh
        </button>
      </div>

      {/* Top-level Risk Summary Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          title="Risk Score"
          value={`${riskScore}/100`}
          subtitle={riskLevel}
          icon={<Shield className="h-5 w-5" />}
          valueClassName={riskColor}
        />
        <StatCard
          title="Value at Risk (95%)"
          value={formatPercentage(mockRiskMetrics.var95)}
          subtitle={`99%: ${formatPercentage(mockRiskMetrics.var99)}`}
          icon={<TrendingDown className="h-5 w-5" />}
          valueClassName={mockRiskMetrics.var95 > RISK_THRESHOLDS.var.warning ? 'text-warning' : 'text-foreground'}
        />
        <StatCard
          title="Max Drawdown"
          value={formatPercentage(0.078)}
          subtitle={`Threshold: ${formatPercentage(RISK_THRESHOLDS.drawdown.critical)}`}
          icon={<Activity className="h-5 w-5" />}
          valueClassName="text-warning"
        />
        <StatCard
          title="Beta to Market"
          value={mockRiskMetrics.betaToMarket.toFixed(2)}
          subtitle="vs BTC benchmark"
          icon={<BarChart3 className="h-5 w-5" />}
        />
      </div>

      {/* Main Content Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left: Circuit Breakers + Alerts */}
        <div className="lg:col-span-1 space-y-6">
          <CircuitBreakerStatus circuitBreakers={mockCircuitBreakers} />

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
              data={mockExposure}
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
            <DrawdownChart
              data={
                timeRange === '1w' ? mockDrawdown.slice(-7)
                  : timeRange === '1m' ? mockDrawdown.slice(-30)
                  : mockDrawdown
              }
              height={300}
            />
          </div>

          {/* Correlation Matrix */}
          <div className="bg-card border border-border rounded-lg p-6">
            <h3 className="font-semibold mb-4">Asset Correlation Matrix</h3>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr>
                    <th className="text-left p-2 text-muted-foreground">Asset</th>
                    {Object.keys(mockRiskMetrics.correlation).map(asset => (
                      <th key={asset} className="p-2 text-muted-foreground text-center">
                        {asset.replace('/USDT', '')}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {Object.entries(mockRiskMetrics.correlation).map(([asset, _]) => (
                    <tr key={asset} className="border-t border-border">
                      <td className="p-2 font-medium">{asset.replace('/USDT', '')}</td>
                      {Object.keys(mockRiskMetrics.correlation).map(otherAsset => {
                        const corr = asset === otherAsset
                          ? 1.0
                          : Math.min(
                              (mockRiskMetrics.correlation[asset] || 0.5) *
                              (mockRiskMetrics.correlation[otherAsset] || 0.5),
                              0.99
                            )
                        const intensity = Math.abs(corr)
                        return (
                          <td
                            key={otherAsset}
                            className="p-2 text-center font-mono text-xs"
                            style={{
                              backgroundColor: `rgba(${corr > 0 ? '34,197,94' : '239,68,68'}, ${intensity * 0.3})`,
                            }}
                          >
                            {corr.toFixed(2)}
                          </td>
                        )
                      })}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          {/* Position Sizing */}
          <div className="bg-card border border-border rounded-lg p-6">
            <h3 className="font-semibold mb-4">Position Sizing Breakdown</h3>
            <div className="space-y-3">
              {mockExposure.map(pos => (
                <div key={pos.symbol} className="flex items-center gap-4">
                  <div className="w-24 text-sm font-medium">{pos.symbol.replace('/USDT', '')}</div>
                  <div className="flex-1">
                    <div className="w-full h-3 bg-muted rounded-full overflow-hidden">
                      <div
                        className={cn(
                          'h-full rounded-full transition-all',
                          pos.side === 'long' ? 'bg-profit' : 'bg-loss'
                        )}
                        style={{ width: `${pos.percentage}%` }}
                      />
                    </div>
                  </div>
                  <div className="w-20 text-right text-sm font-mono">{formatCurrency(pos.value)}</div>
                  <div className="w-12 text-right text-sm text-muted-foreground">{pos.percentage}%</div>
                  <span className={cn(
                    'text-xs px-2 py-0.5 rounded-full',
                    pos.side === 'long' ? 'bg-profit/20 text-profit' : 'bg-loss/20 text-loss'
                  )}>
                    {pos.side}
                  </span>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
