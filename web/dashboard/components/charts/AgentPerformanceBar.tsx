'use client'

import { 
  BarChart, 
  Bar, 
  XAxis, 
  YAxis, 
  CartesianGrid, 
  Tooltip, 
  ResponsiveContainer,
  Legend
} from 'recharts'
import { Agent } from '@/lib/types'
import { formatCurrency, formatPercentage, cn } from '@/lib/utils'
import { CHART_COLORS } from '@/lib/constants'

interface AgentPerformanceBarProps {
  agents: Agent[]
  metric?: 'pnl' | 'winRate' | 'trades' | 'avgReturn'
  height?: number
  showLegend?: boolean
  showGrid?: boolean
  loading?: boolean
  className?: string
}

export function AgentPerformanceBar({
  agents,
  metric = 'pnl',
  height = 300,
  showLegend = true,
  showGrid = true,
  loading = false,
  className
}: AgentPerformanceBarProps) {
  if (loading) {
    return (
      <div className={cn("chart-container", className)} style={{ height }}>
        <div className="chart-loading">
          <div className="flex items-center gap-2">
            <div className="w-4 h-4 border-2 border-muted-foreground border-t-transparent rounded-full animate-spin" />
            <span className="text-sm text-muted-foreground">Loading performance data...</span>
          </div>
        </div>
      </div>
    )
  }

  if (!agents || agents.length === 0) {
    return (
      <div className={cn("chart-container flex items-center justify-center", className)} style={{ height }}>
        <div className="text-center">
          <div className="text-muted-foreground text-sm">No agent data available</div>
          <div className="text-muted-foreground text-xs mt-1">Agents will appear once they start trading</div>
        </div>
      </div>
    )
  }

  const getMetricValue = (agent: Agent) => {
    switch (metric) {
      case 'pnl':
        return agent.totalPnl
      case 'winRate':
        return agent.winRate
      case 'trades':
        return agent.totalTrades
      case 'avgReturn':
        return agent.avgReturn
      default:
        return agent.totalPnl
    }
  }

  const getMetricLabel = () => {
    switch (metric) {
      case 'pnl':
        return 'Total P&L'
      case 'winRate':
        return 'Win Rate %'
      case 'trades':
        return 'Total Trades'
      case 'avgReturn':
        return 'Average Return %'
      default:
        return 'Total P&L'
    }
  }

  const formatMetricValue = (value: number) => {
    switch (metric) {
      case 'pnl':
        return formatCurrency(value)
      case 'winRate':
      case 'avgReturn':
        return formatPercentage(value)
      case 'trades':
        return value.toString()
      default:
        return value.toString()
    }
  }

  const getBarColor = (value: number, index: number) => {
    if (metric === 'pnl' || metric === 'avgReturn') {
      return value >= 0 ? '#22C55E' : '#EF4444'
    }
    return CHART_COLORS[index % CHART_COLORS.length]
  }

  // Prepare chart data - ensure all values are finite numbers
  const chartData = agents
    .map((agent, index) => ({
      name: agent.name,
      displayName: agent.name.replace('_', ' ').replace(/\b\w/g, l => l.toUpperCase()),
      value: Number.isFinite(getMetricValue(agent)) ? getMetricValue(agent) : 0,
      status: agent.status,
      color: getBarColor(getMetricValue(agent), index),
      totalTrades: agent.totalTrades,
      winRate: agent.winRate,
      totalPnl: agent.totalPnl,
      avgReturn: agent.avgReturn,
    }))
    .sort((a, b) => b.value - a.value) // Sort by metric value

  const CustomTooltip = ({ active, payload, label }: any) => {
    if (active && payload && payload.length) {
      const data = payload[0].payload
      return (
        <div className="bg-card border border-border rounded-lg p-3 shadow-lg min-w-48">
          <div className="font-medium text-sm mb-2">{data.displayName}</div>
          <div className="space-y-1 text-sm">
            <div className="flex justify-between">
              <span className="text-muted-foreground">Status:</span>
              <span className={cn(
                "capitalize font-medium",
                data.status === 'active' && "text-profit",
                data.status === 'idle' && "text-info",
                data.status === 'error' && "text-loss",
                data.status === 'disabled' && "text-muted-foreground"
              )}>
                {data.status}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Total P&L:</span>
              <span className={cn(
                "font-mono",
                data.totalPnl > 0 ? "text-profit" : data.totalPnl < 0 ? "text-loss" : "text-muted-foreground"
              )}>
                {formatCurrency(data.totalPnl)}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Win Rate:</span>
              <span className="font-mono">{formatPercentage(data.winRate)}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Total Trades:</span>
              <span className="font-mono">{data.totalTrades}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Avg Return:</span>
              <span className={cn(
                "font-mono",
                data.avgReturn > 0 ? "text-profit" : data.avgReturn < 0 ? "text-loss" : "text-muted-foreground"
              )}>
                {formatPercentage(data.avgReturn)}
              </span>
            </div>
          </div>
        </div>
      )
    }
    return null
  }

  const CustomBar = (props: any) => {
    const { fill, ...rest } = props
    return <Bar {...rest} fill={props.payload.color} />
  }

  return (
    <div className={cn("chart-container", className)}>
      {/* Chart Header */}
      <div className="absolute top-4 left-4 z-10 bg-card/80 backdrop-blur-sm rounded-lg p-3 border border-border">
        <h3 className="font-semibold text-sm">{getMetricLabel()} by Agent</h3>
        <div className="mt-2 text-xs text-muted-foreground">
          {chartData.length} agents • Sorted by {getMetricLabel().toLowerCase()}
        </div>
      </div>

      {/* Chart */}
      <div style={{ height }}>
        <ResponsiveContainer width="100%" height="100%">
          <BarChart 
            data={chartData} 
            margin={{ top: 80, right: 30, left: 20, bottom: 60 }}
          >
            {showGrid && (
              <CartesianGrid 
                strokeDasharray="3 3" 
                stroke="hsl(var(--border))" 
                opacity={0.3}
              />
            )}
            
            <XAxis
              dataKey="displayName"
              stroke="hsl(var(--muted-foreground))"
              fontSize={11}
              tickLine={false}
              axisLine={false}
              angle={-45}
              textAnchor="end"
              height={60}
              interval={0}
            />
            
            <YAxis
              tickFormatter={(value) => {
                switch (metric) {
                  case 'pnl':
                    return formatCurrency(value, 'USD', { 
                      notation: 'compact',
                      minimumFractionDigits: 0,
                      maximumFractionDigits: 0 
                    })
                  case 'winRate':
                  case 'avgReturn':
                    return `${value.toFixed(0)}%`
                  default:
                    return value.toString()
                }
              }}
              stroke="hsl(var(--muted-foreground))"
              fontSize={12}
              tickLine={false}
              axisLine={false}
            />

            <Tooltip content={<CustomTooltip />} />

            {showLegend && (
              <Legend 
                verticalAlign="bottom"
                height={36}
                content={() => (
                  <div className="flex justify-center mt-4">
                    <div className="text-xs text-muted-foreground">
                      Hover bars for detailed metrics
                    </div>
                  </div>
                )}
              />
            )}

            <Bar
              dataKey="value"
              shape={<CustomBar />}
              radius={[4, 4, 0, 0]}
              stroke="transparent"
              strokeWidth={0}
            />
          </BarChart>
        </ResponsiveContainer>
      </div>

      {/* Summary Stats */}
      <div className="absolute bottom-4 right-4 bg-card/80 backdrop-blur-sm rounded-lg p-3 border border-border text-xs">
        <div className="space-y-1">
          <div className="flex justify-between gap-4">
            <span className="text-muted-foreground">Best:</span>
            <span className="font-mono">{formatMetricValue(chartData[0]?.value || 0)}</span>
          </div>
          <div className="flex justify-between gap-4">
            <span className="text-muted-foreground">Worst:</span>
            <span className="font-mono">{formatMetricValue(chartData[chartData.length - 1]?.value || 0)}</span>
          </div>
          <div className="flex justify-between gap-4">
            <span className="text-muted-foreground">Avg:</span>
            <span className="font-mono">
              {formatMetricValue(
                chartData.reduce((sum, agent) => sum + agent.value, 0) / chartData.length
              )}
            </span>
          </div>
        </div>
      </div>
    </div>
  )
}