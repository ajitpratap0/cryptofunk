'use client'

import { useState } from 'react'
import { CandlestickChart } from '@/components/charts/CandlestickChart'
import { EquityCurve } from '@/components/charts/EquityCurve'
import { DrawdownChart } from '@/components/charts/DrawdownChart'
import { StatCard } from '@/components/ui/StatCard'
import { useEquityHistory, usePerformanceMetrics, usePairPerformance, useCandlestickData, useDrawdownAnalysis } from '@/hooks/usePerformance'
import { TimeRange } from '@/lib/types'
import { formatPercentage, formatCurrency, cn } from '@/lib/utils'
import { 
  TrendingUp,
  BarChart3,
  Activity,
  Target,
  AlertTriangle,
  Calendar
} from 'lucide-react'

export default function PerformancePage() {
  const [selectedTimeRange, setSelectedTimeRange] = useState<TimeRange>('1m')
  const [selectedSymbol, setSelectedSymbol] = useState('BTC/USDT')

  const { data: equityData, isLoading: equityLoading } = useEquityHistory(selectedTimeRange)
  const { data: metricsData, isLoading: metricsLoading } = usePerformanceMetrics()
  const { data: pairData, isLoading: pairLoading } = usePairPerformance()
  const { data: candlestickData, isLoading: candlestickLoading } = useCandlestickData(selectedSymbol, '5m')
  const { data: drawdownData, isLoading: drawdownLoading } = useDrawdownAnalysis()

  const metrics = metricsData?.data
  const pairPerformance = pairData?.data || []
  const candlestickChartData = candlestickData?.data || []
  
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
            {pairPerformance.map((pair: any, index: number) => (
              <div key={pair.pair} className="flex items-center justify-between p-3 bg-muted/30 rounded-lg">
                <div>
                  <div className="font-medium">{pair.pair}</div>
                  <div className="text-sm text-muted-foreground">
                    {pair.trades} trades • {formatPercentage(pair.winRate)} win rate
                  </div>
                </div>
                
                <div className="text-right">
                  <div className={cn(
                    "font-bold",
                    pair.pnl > 0 ? "text-profit" : pair.pnl < 0 ? "text-loss" : "text-muted-foreground"
                  )}>
                    {pair.pnl > 0 ? '+' : ''}{formatCurrency(pair.pnl)}
                  </div>
                  <div className={cn(
                    "text-sm",
                    pair.avgReturn > 0 ? "text-profit" : "text-loss"
                  )}>
                    {pair.avgReturn > 0 ? '+' : ''}{formatPercentage(pair.avgReturn)} avg
                  </div>
                </div>
              </div>
            ))}

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
    </div>
  )
}