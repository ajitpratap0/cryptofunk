'use client'

import { useState } from 'react'
import { CandlestickChart } from '@/components/charts/CandlestickChart'
import { EquityCurve } from '@/components/charts/EquityCurve'
import { DrawdownChart } from '@/components/charts/DrawdownChart'
import { StatCard } from '@/components/ui/StatCard'
import { useEquityHistory, usePerformanceMetrics, usePairPerformance, useCandlestickData, useDrawdownAnalysis, type PairPerformance } from '@/hooks/usePerformance'
import { useDecisionAnalytics, useDecisionOutcomes } from '@/hooks/useDecisionTracking'
import { TimeRange, CandlestickData } from '@/lib/types'
import { formatPercentage, formatCurrency, cn } from '@/lib/utils'
import { 
  TrendingUp,
  BarChart3,
  Activity,
  Target,
  AlertTriangle,
  Calendar,
  Brain,
  CheckCircle2,
  XCircle
} from 'lucide-react'

export default function PerformancePage() {
  const [selectedTimeRange, setSelectedTimeRange] = useState<TimeRange>('1m')
  const [selectedSymbol, setSelectedSymbol] = useState('BTC/USDT')

  const { data: equityData, isLoading: equityLoading, isError: equityError, error: equityErrorObj, refetch: refetchEquity } = useEquityHistory(selectedTimeRange)
  const { data: metricsData, isLoading: metricsLoading, isError: metricsError, error: metricsErrorObj, refetch: refetchMetrics } = usePerformanceMetrics()
  const { data: pairData, isLoading: pairLoading, isError: pairError, error: pairErrorObj, refetch: refetchPair } = usePairPerformance()
  const { data: candlestickData, isLoading: candlestickLoading } = useCandlestickData(selectedSymbol, '5m')
  const { data: drawdownData, isLoading: drawdownLoading } = useDrawdownAnalysis()

  const metrics = metricsData?.data
  const pairPerformance = pairData?.data || []

  // Surface backend failures to the user. We aggregate the three primary
  // queries (summary metrics, equity history, pair performance) and show a
  // single banner with a retry that refetches all three. We deliberately
  // exclude the candlestick and drawdown queries: candlestick has its own
  // NEXT_PUBLIC_USE_MOCK_CANDLES fallback and drawdown is derived from
  // equity, so surfacing those separately would be noisy.
  const hasError = metricsError || equityError || pairError
  const firstErrorMessage =
    (metricsError && (metricsErrorObj instanceof Error ? metricsErrorObj.message : 'Failed to load performance metrics')) ||
    (equityError && (equityErrorObj instanceof Error ? equityErrorObj.message : 'Failed to load equity history')) ||
    (pairError && (pairErrorObj instanceof Error ? pairErrorObj.message : 'Failed to load pair performance')) ||
    ''
  const retryAll = () => {
    if (metricsError) refetchMetrics()
    if (equityError) refetchEquity()
    if (pairError) refetchPair()
  }
  const candlestickChartData: CandlestickData[] = candlestickData?.data ?? []
  
  const timeRangeOptions: { value: TimeRange; label: string }[] = [
    { value: '1h', label: '1H' },
    { value: '4h', label: '4H' },
    { value: '1d', label: '1D' },
    { value: '1w', label: '1W' },
    { value: '1m', label: '1M' },
    { value: '3m', label: '3M' },
    { value: '1y', label: '1Y' },
  ]

  const popularPairs = ['BTC/USDT', 'ETH/USDT', 'BNB/USDT', 'XRP/USDT', 'ADA/USDT', 'SOL/USDT']

  // Generate mock trade markers for the candlestick chart
  const tradeMarkers = [
    {
      time: candlestickChartData[Math.floor(candlestickChartData.length * 0.3)]?.time || new Date().toISOString(),
      position: 'belowBar' as const,
      color: '#22C55E',
      shape: 'arrowUp' as const,
      text: 'Long Entry',
      size: 1,
    },
    {
      time: candlestickChartData[Math.floor(candlestickChartData.length * 0.7)]?.time || new Date().toISOString(),
      position: 'aboveBar' as const,
      color: '#EF4444',
      shape: 'arrowDown' as const,
      text: 'Exit',
      size: 1,
    },
  ]

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Performance Analytics</h1>
          <p className="text-muted-foreground">
            Detailed charts, metrics, and performance analysis
          </p>
        </div>

        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2">
            <Calendar className="h-4 w-4 text-muted-foreground" />
            <span className="text-sm text-muted-foreground">Time Range:</span>
          </div>
          <div className="flex bg-muted rounded-lg p-1">
            {timeRangeOptions.map((option) => (
              <button
                key={option.value}
                onClick={() => setSelectedTimeRange(option.value)}
                className={cn(
                  "px-3 py-1 text-sm font-medium rounded transition-colors",
                  selectedTimeRange === option.value
                    ? "bg-background text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground"
                )}
              >
                {option.label}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Error banner — shown when any of the primary performance queries fail */}
      {hasError && (
        <div
          role="alert"
          className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 p-4 rounded-lg border border-loss/30 bg-loss/10 text-sm"
        >
          <div className="flex items-start gap-3">
            <AlertTriangle className="h-5 w-5 text-loss mt-0.5 flex-shrink-0" />
            <div>
              <div className="font-medium text-loss">Failed to load performance data</div>
              <div className="text-muted-foreground mt-0.5">
                {firstErrorMessage || 'One or more performance API calls failed.'}
              </div>
            </div>
          </div>
          <button
            onClick={retryAll}
            className="px-3 py-1.5 text-sm font-medium rounded-md bg-background border border-border hover:bg-muted transition-colors self-start sm:self-auto"
          >
            Retry
          </button>
        </div>
      )}

      {/* Performance Metrics Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-6">
        <StatCard
          title="Sharpe Ratio"
          value={metrics?.sharpeRatio || 0}
          formatAs="number"
          suffix=""
          icon={<Target className="h-5 w-5" />}
          trend={metrics?.sharpeRatio && metrics.sharpeRatio > 1 ? 'up' : 'neutral'}
          loading={metricsLoading}
        />

        <StatCard
          title="Max Drawdown"
          value={metrics?.maxDrawdownPercent || 0}
          formatAs="percentage"
          icon={<AlertTriangle className="h-5 w-5" />}
          trend={metrics?.maxDrawdownPercent && metrics.maxDrawdownPercent < 10 ? 'up' : 'down'}
          loading={metricsLoading}
        />

        <StatCard
          title="Total Return"
          value={metrics?.totalReturn || 0}
          formatAs="percentage"
          icon={<TrendingUp className="h-5 w-5" />}
          trend={metrics?.totalReturn && metrics.totalReturn > 0 ? 'up' : 'down'}
          loading={metricsLoading}
        />

        <StatCard
          title="Profit Factor"
          value={metrics?.profitFactor || 0}
          formatAs="number"
          suffix="x"
          icon={<BarChart3 className="h-5 w-5" />}
          trend={metrics?.profitFactor && metrics.profitFactor > 1 ? 'up' : 'down'}
          loading={metricsLoading}
        />
      </div>

      {/* Candlestick Chart */}
      <div className="bg-card border border-border rounded-lg p-6">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-xl font-semibold">Price Chart with Trade Markers</h2>
          
          <div className="flex items-center gap-2">
            <span className="text-sm text-muted-foreground">Symbol:</span>
            <select
              value={selectedSymbol}
              onChange={(e) => setSelectedSymbol(e.target.value)}
              className="px-3 py-2 border border-border rounded-lg bg-background text-foreground text-sm focus:outline-none focus:ring-2 focus:ring-primary/50"
            >
              {popularPairs.map((pair) => (
                <option key={pair} value={pair}>{pair}</option>
              ))}
            </select>
          </div>
        </div>
        
        <CandlestickChart
          data={candlestickChartData}
          tradeMarkers={tradeMarkers}
          symbol={selectedSymbol}
          timeframe="5m"
          height={500}
          loading={candlestickLoading}
        />
      </div>

      {/* Equity & Drawdown Charts */}
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
        {/* Equity Curve */}
        <div className="bg-card border border-border rounded-lg p-6">
          <h2 className="text-xl font-semibold mb-6">Equity Curve</h2>
          <EquityCurve
            data={equityData?.data || []}
            timeRange={selectedTimeRange}
            height={350}
            loading={equityLoading}
          />
        </div>

        {/* Drawdown Chart */}
        <div className="bg-card border border-border rounded-lg p-6">
          <h2 className="text-xl font-semibold mb-6">Drawdown Analysis</h2>
          <DrawdownChart
            data={drawdownData?.data || []}
            timeRange={selectedTimeRange}
            height={350}
            loading={drawdownLoading}
          />
        </div>
      </div>

      {/* Detailed Metrics */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* Performance Metrics */}
        <div className="bg-card border border-border rounded-lg p-6">
          <h3 className="text-lg font-semibold mb-4">Performance Metrics</h3>
          
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="text-sm text-muted-foreground block">Win Rate</label>
                <div className="text-lg font-bold">
                  {metrics ? formatPercentage(metrics.winRate) : '--'}
                </div>
              </div>
              
              <div>
                <label className="text-sm text-muted-foreground block">Sharpe Ratio</label>
                <div className="text-lg font-bold">
                  {metrics?.sharpeRatio ? metrics.sharpeRatio.toFixed(2) : '--'}
                </div>
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="text-sm text-muted-foreground block">Sortino Ratio</label>
                <div className="text-lg font-bold">
                  {metrics?.sortinoRatio ? metrics.sortinoRatio.toFixed(2) : '--'}
                </div>
              </div>
              
              <div>
                <label className="text-sm text-muted-foreground block">Calmar Ratio</label>
                <div className="text-lg font-bold">
                  {metrics?.calmarRatio ? metrics.calmarRatio.toFixed(2) : '--'}
                </div>
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="text-sm text-muted-foreground block">Average Win</label>
                <div className="text-lg font-bold text-profit">
                  {metrics ? formatCurrency(metrics.avgWin) : '--'}
                </div>
              </div>
              
              <div>
                <label className="text-sm text-muted-foreground block">Average Loss</label>
                <div className="text-lg font-bold text-loss">
                  {metrics ? formatCurrency(metrics.avgLoss) : '--'}
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Pair Performance */}
        <div className="bg-card border border-border rounded-lg p-6">
          <h3 className="text-lg font-semibold mb-4">Performance by Pair</h3>
          
          <div className="space-y-3">
            {pairPerformance.map((pair: PairPerformance) => {
              // Backend may omit realized_pnl/trade_count for pairs with no
              // closed positions — coerce nullish values to 0 so the UI
              // doesn't render "NaN" or "$null".
              const pnl = pair.realized_pnl ?? 0
              const trades = pair.trade_count ?? 0
              return (
                <div key={pair.symbol} className="flex items-center justify-between p-3 bg-muted/30 rounded-lg">
                  <div>
                    <div className="font-medium">{pair.symbol}</div>
                    <div className="text-sm text-muted-foreground">
                      {trades} {trades === 1 ? 'trade' : 'trades'}
                    </div>
                  </div>

                  <div className="text-right">
                    <div className={cn(
                      "font-bold",
                      pnl > 0 ? "text-profit" : pnl < 0 ? "text-loss" : "text-muted-foreground"
                    )}>
                      {pnl > 0 ? '+' : ''}{formatCurrency(pnl)}
                    </div>
                    <div className="text-xs text-muted-foreground">
                      realized P&L
                    </div>
                  </div>
                </div>
              )
            })}

            {pairLoading && (
              <div className="space-y-3">
                {[1, 2, 3, 4, 5].map((i) => (
                  <div key={i} className="flex justify-between p-3 bg-muted/30 rounded-lg">
                    <div className="space-y-1">
                      <div className="skeleton h-4 w-20" />
                      <div className="skeleton h-3 w-32" />
                    </div>
                    <div className="skeleton h-6 w-16" />
                  </div>
                ))}
              </div>
            )}

            {!pairLoading && pairPerformance.length === 0 && (
              <div className="text-center py-8 text-muted-foreground">
                No pair performance data available
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Decision Tracking Section */}
      <DecisionTrackingSection />
    </div>
  )
}

function DecisionTrackingSection() {
  const { data: analyticsData, isLoading: analyticsLoading } = useDecisionAnalytics('30d')
  const { data: outcomesData, isLoading: outcomesLoading } = useDecisionOutcomes(20)

  const analytics = analyticsData?.data
  const outcomes = outcomesData?.data || []

  return (
    <div className="space-y-6">
      <h2 className="text-xl font-semibold flex items-center gap-2">
        <Brain className="h-5 w-5" />
        Decision Tracking
      </h2>

      {/* Summary Stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <StatCard
          title="Total Decisions"
          value={analytics?.total_decisions ?? 0}
          icon={<Brain className="h-4 w-4" />}
          loading={analyticsLoading}
        />
        <StatCard
          title="Resolved"
          value={analytics?.resolved_decisions ?? 0}
          icon={<CheckCircle2 className="h-4 w-4" />}
          loading={analyticsLoading}
        />
        <StatCard
          title="Win Rate"
          value={formatPercentage((analytics?.overall_win_rate ?? 0) / 100)}
          icon={<Target className="h-4 w-4" />}
          loading={analyticsLoading}
        />
        <StatCard
          title="Avg P&L"
          value={formatCurrency(analytics?.overall_avg_pnl ?? 0)}
          icon={<TrendingUp className="h-4 w-4" />}
          loading={analyticsLoading}
        />
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Agent Performance */}
        <div className="rounded-xl border bg-card p-6">
          <h3 className="text-lg font-semibold mb-4">Agent Accuracy</h3>
          {analyticsLoading ? (
            <div className="space-y-3">
              {[1, 2, 3].map((i) => (
                <div key={i} className="skeleton h-10 w-full rounded" />
              ))}
            </div>
          ) : (analytics?.by_agent ?? []).length === 0 ? (
            <div className="text-center py-8 text-muted-foreground">No agent data</div>
          ) : (
            <div className="space-y-3">
              {(analytics?.by_agent ?? []).map((agent) => (
                <div key={agent.agent_name} className="flex items-center justify-between p-3 bg-muted/30 rounded-lg">
                  <div>
                    <div className="font-medium text-sm">{agent.agent_name}</div>
                    <div className="text-xs text-muted-foreground">
                      {agent.total_decisions} decisions · {agent.wins}W / {agent.losses}L
                    </div>
                  </div>
                  <div className="text-right">
                    <div className={cn('text-sm font-semibold', agent.win_rate >= 50 ? 'text-green-500' : 'text-red-500')}>
                      {agent.win_rate.toFixed(1)}%
                    </div>
                    <div className="text-xs text-muted-foreground">
                      avg conf: {(agent.avg_confidence * 100).toFixed(0)}%
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Confidence Calibration */}
        <div className="rounded-xl border bg-card p-6">
          <h3 className="text-lg font-semibold mb-4">Confidence Calibration</h3>
          {analyticsLoading ? (
            <div className="space-y-3">
              {[1, 2, 3].map((i) => (
                <div key={i} className="skeleton h-8 w-full rounded" />
              ))}
            </div>
          ) : (analytics?.confidence_calibration ?? []).length === 0 ? (
            <div className="text-center py-8 text-muted-foreground">No calibration data</div>
          ) : (
            <div className="space-y-2">
              <div className="grid grid-cols-4 text-xs font-medium text-muted-foreground px-2">
                <span>Confidence</span>
                <span className="text-center">Count</span>
                <span className="text-center">Win Rate</span>
                <span className="text-right">Avg P&L</span>
              </div>
              {(analytics?.confidence_calibration ?? []).map((bucket) => (
                <div key={bucket.bucket_low} className="grid grid-cols-4 text-sm p-2 bg-muted/20 rounded">
                  <span>{(bucket.bucket_low * 100).toFixed(0)}-{(bucket.bucket_high * 100).toFixed(0)}%</span>
                  <span className="text-center">{bucket.count}</span>
                  <span className={cn('text-center', bucket.win_rate >= 50 ? 'text-green-500' : 'text-red-500')}>
                    {bucket.win_rate.toFixed(1)}%
                  </span>
                  <span className="text-right">{formatCurrency(bucket.avg_pnl)}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Recent Decisions with Outcomes */}
      <div className="rounded-xl border bg-card p-6">
        <h3 className="text-lg font-semibold mb-4">Recent Decisions</h3>
        {outcomesLoading ? (
          <div className="space-y-3">
            {[1, 2, 3, 4, 5].map((i) => (
              <div key={i} className="skeleton h-12 w-full rounded" />
            ))}
          </div>
        ) : outcomes.length === 0 ? (
          <div className="text-center py-8 text-muted-foreground">No decisions yet</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-xs text-muted-foreground border-b">
                  <th className="text-left p-2">Agent</th>
                  <th className="text-left p-2">Symbol</th>
                  <th className="text-left p-2">Type</th>
                  <th className="text-center p-2">Confidence</th>
                  <th className="text-center p-2">Outcome</th>
                  <th className="text-right p-2">P&L</th>
                  <th className="text-right p-2">Time</th>
                </tr>
              </thead>
              <tbody>
                {outcomes.map((d) => (
                  <tr key={d.id} className="border-b border-muted/30 hover:bg-muted/10">
                    <td className="p-2 font-medium">{d.agent_name}</td>
                    <td className="p-2">{d.symbol}</td>
                    <td className="p-2">{d.decision_type}</td>
                    <td className="p-2 text-center">{(d.confidence * 100).toFixed(0)}%</td>
                    <td className="p-2 text-center">
                      {d.outcome === 'SUCCESS' ? (
                        <CheckCircle2 className="h-4 w-4 text-green-500 inline" />
                      ) : d.outcome === 'FAILURE' ? (
                        <XCircle className="h-4 w-4 text-red-500 inline" />
                      ) : (
                        <span className="text-muted-foreground">—</span>
                      )}
                    </td>
                    <td className={cn('p-2 text-right', (d.pnl ?? 0) >= 0 ? 'text-green-500' : 'text-red-500')}>
                      {d.pnl != null ? formatCurrency(d.pnl) : '—'}
                    </td>
                    <td className="p-2 text-right text-muted-foreground">
                      {new Date(d.created_at).toLocaleDateString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}