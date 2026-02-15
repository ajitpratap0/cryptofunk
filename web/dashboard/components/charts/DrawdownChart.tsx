'use client'

import { 
  AreaChart, 
  Area, 
  XAxis, 
  YAxis, 
  CartesianGrid, 
  Tooltip, 
  ResponsiveContainer,
  ReferenceLine
} from 'recharts'
import { format, parseISO } from 'date-fns'
import { TimeRange } from '@/lib/types'
import { formatPercentage, formatDateTime, cn } from '@/lib/utils'

interface DrawdownData {
  timestamp: string
  drawdown: number
  equity: number
}

interface DrawdownChartProps {
  data: DrawdownData[]
  timeRange?: TimeRange
  height?: number
  showGrid?: boolean
  showTooltip?: boolean
  loading?: boolean
  className?: string
  thresholds?: {
    warning: number
    critical: number
  }
}

export function DrawdownChart({
  data,
  timeRange = '1m',
  height = 300,
  showGrid = true,
  showTooltip = true,
  loading = false,
  className,
  thresholds = { warning: -5, critical: -10 }
}: DrawdownChartProps) {
  if (loading) {
    return (
      <div className={cn("chart-container", className)} style={{ height }}>
        <div className="chart-loading">
          <div className="flex items-center gap-2">
            <div className="w-4 h-4 border-2 border-muted-foreground border-t-transparent rounded-full animate-spin" />
            <span className="text-sm text-muted-foreground">Loading drawdown data...</span>
          </div>
        </div>
      </div>
    )
  }

  if (!data || data.length === 0) {
    return (
      <div className={cn("chart-container flex items-center justify-center", className)} style={{ height }}>
        <div className="text-center">
          <div className="text-muted-foreground text-sm">No drawdown data available</div>
          <div className="text-muted-foreground text-xs mt-1">Data will appear once trading begins</div>
        </div>
      </div>
    )
  }

  // Sanitize data - filter out entries with non-finite values
  const safeData = data.filter(d => 
    Number.isFinite(d.drawdown) && Number.isFinite(d.equity)
  )

  if (safeData.length === 0) {
    return (
      <div className={cn("chart-container flex items-center justify-center", className)} style={{ height }}>
        <div className="text-center">
          <div className="text-muted-foreground text-sm">No valid drawdown data available</div>
        </div>
      </div>
    )
  }

  // Calculate drawdown metrics
  const maxDrawdown = Math.min(...safeData.map(d => d.drawdown))
  const currentDrawdown = safeData[safeData.length - 1]?.drawdown || 0
  const maxDrawdownDate = safeData.find(d => d.drawdown === maxDrawdown)?.timestamp
  
  // Count periods in different risk zones
  const safeCount = safeData.filter(d => d.drawdown > thresholds.warning).length
  const warningCount = safeData.filter(d => d.drawdown <= thresholds.warning && d.drawdown > thresholds.critical).length
  const criticalCount = safeData.filter(d => d.drawdown <= thresholds.critical).length

  // Format data for chart
  const chartData = safeData.map(point => ({
    ...point,
    timestamp: new Date(point.timestamp).getTime(),
    formattedTime: format(parseISO(point.timestamp), getTimeFormat(timeRange)),
  }))

  const getRiskLevel = (drawdown: number) => {
    if (drawdown <= thresholds.critical) return 'critical'
    if (drawdown <= thresholds.warning) return 'warning'
    return 'safe'
  }

  const getRiskColor = (level: string) => {
    switch (level) {
      case 'critical': return '#EF4444'
      case 'warning': return '#F59E0B'
      case 'safe': return '#22C55E'
      default: return '#6B7280'
    }
  }

  const currentRiskLevel = getRiskLevel(currentDrawdown)

  const CustomTooltip = ({ active, payload, label }: any) => {
    if (active && payload && payload.length) {
      const data = payload[0].payload
      const riskLevel = getRiskLevel(data.drawdown)
      
      return (
        <div className="bg-card border border-border rounded-lg p-3 shadow-lg">
          <p className="text-sm font-medium">{formatDateTime(new Date(label))}</p>
          <div className="mt-2 space-y-1">
            <div className="flex justify-between gap-4">
              <span className="text-muted-foreground text-sm">Drawdown:</span>
              <span className={cn(
                "font-mono text-sm",
                riskLevel === 'critical' && "text-loss",
                riskLevel === 'warning' && "text-warning",
                riskLevel === 'safe' && "text-profit"
              )}>
                {data.drawdown.toFixed(2)}%
              </span>
            </div>
            <div className="flex justify-between gap-4">
              <span className="text-muted-foreground text-sm">Risk Level:</span>
              <span className={cn(
                "text-sm font-medium capitalize",
                riskLevel === 'critical' && "text-loss",
                riskLevel === 'warning' && "text-warning",
                riskLevel === 'safe' && "text-profit"
              )}>
                {riskLevel}
              </span>
            </div>
          </div>
        </div>
      )
    }
    return null
  }

  const getGradientColor = () => {
    switch (currentRiskLevel) {
      case 'critical': return '#EF4444'
      case 'warning': return '#F59E0B'
      default: return '#22C55E'
    }
  }

  return (
    <div className={cn("chart-container", className)}>
      {/* Chart Header */}
      <div className="absolute top-4 left-4 z-10 bg-card/80 backdrop-blur-sm rounded-lg p-3 border border-border">
        <h3 className="font-semibold text-sm mb-2">Drawdown Analysis</h3>
        <div className="grid grid-cols-2 gap-4 text-xs">
          <div>
            <div className="text-muted-foreground">Current Drawdown</div>
            <div className={cn(
              "font-mono font-medium",
              currentRiskLevel === 'critical' && "text-loss",
              currentRiskLevel === 'warning' && "text-warning",
              currentRiskLevel === 'safe' && "text-profit"
            )}>
              {currentDrawdown.toFixed(2)}%
            </div>
          </div>
          <div>
            <div className="text-muted-foreground">Max Drawdown</div>
            <div className="font-mono font-medium text-loss">
              {maxDrawdown.toFixed(2)}%
            </div>
          </div>
          <div>
            <div className="text-muted-foreground">Risk Level</div>
            <div className={cn(
              "font-medium capitalize",
              currentRiskLevel === 'critical' && "text-loss",
              currentRiskLevel === 'warning' && "text-warning",
              currentRiskLevel === 'safe' && "text-profit"
            )}>
              {currentRiskLevel}
            </div>
          </div>
          <div>
            <div className="text-muted-foreground">Max DD Date</div>
            <div className="font-mono text-xs">
              {maxDrawdownDate ? format(parseISO(maxDrawdownDate), 'MMM dd') : 'N/A'}
            </div>
          </div>
        </div>
      </div>

      {/* Chart */}
      <div style={{ height }}>
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={chartData} margin={{ top: 80, right: 30, left: 20, bottom: 20 }}>
            {showGrid && (
              <CartesianGrid 
                strokeDasharray="3 3" 
                stroke="hsl(var(--border))" 
                opacity={0.3}
              />
            )}
            
            <XAxis
              dataKey="timestamp"
              type="number"
              scale="time"
              domain={['dataMin', 'dataMax']}
              tickFormatter={(value) => format(new Date(value), getTickFormat(timeRange))}
              stroke="hsl(var(--muted-foreground))"
              fontSize={12}
              tickLine={false}
            />
            
            <YAxis
              domain={[Math.min(-20, maxDrawdown - 2), 2]}
              tickFormatter={(value) => `${value.toFixed(0)}%`}
              stroke="hsl(var(--muted-foreground))"
              fontSize={12}
              tickLine={false}
              axisLine={false}
            />

            {showTooltip && <Tooltip content={<CustomTooltip />} />}

            {/* Risk threshold lines */}
            <ReferenceLine 
              y={thresholds.warning} 
              stroke="#F59E0B" 
              strokeDasharray="5 5"
              strokeOpacity={0.7}
              label={{ value: "Warning", position: "insideTopLeft", fontSize: 10 }}
            />
            
            <ReferenceLine 
              y={thresholds.critical} 
              stroke="#EF4444" 
              strokeDasharray="5 5"
              strokeOpacity={0.7}
              label={{ value: "Critical", position: "insideTopLeft", fontSize: 10 }}
            />

            <defs>
              <linearGradient id="drawdownGradient" x1="0" y1="0" x2="0" y2="1">
                <stop 
                  offset="0%" 
                  stopColor={getGradientColor()} 
                  stopOpacity={0.1} 
                />
                <stop 
                  offset="100%" 
                  stopColor={getGradientColor()} 
                  stopOpacity={0.6} 
                />
              </linearGradient>
            </defs>
            
            <Area
              type="monotone"
              dataKey="drawdown"
              stroke={getGradientColor()}
              strokeWidth={2}
              fill="url(#drawdownGradient)"
              fillOpacity={1}
              dot={false}
              activeDot={{ 
                r: 4, 
                stroke: getGradientColor(),
                strokeWidth: 2,
                fill: "hsl(var(--background))"
              }}
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>

      {/* Risk Distribution */}
      <div className="absolute bottom-4 right-4 bg-card/80 backdrop-blur-sm rounded-lg p-3 border border-border text-xs">
        <div className="font-medium text-muted-foreground mb-2">Time in Risk Zones</div>
        <div className="space-y-1">
          <div className="flex justify-between gap-4">
            <div className="flex items-center gap-2">
              <div className="w-2 h-2 rounded-full bg-profit" />
              <span>Safe</span>
            </div>
            <span>{formatPercentage((safeCount / safeData.length) * 100)}</span>
          </div>
          <div className="flex justify-between gap-4">
            <div className="flex items-center gap-2">
              <div className="w-2 h-2 rounded-full bg-warning" />
              <span>Warning</span>
            </div>
            <span>{formatPercentage((warningCount / safeData.length) * 100)}</span>
          </div>
          <div className="flex justify-between gap-4">
            <div className="flex items-center gap-2">
              <div className="w-2 h-2 rounded-full bg-loss" />
              <span>Critical</span>
            </div>
            <span>{formatPercentage((criticalCount / safeData.length) * 100)}</span>
          </div>
        </div>
      </div>
    </div>
  )
}

function getTimeFormat(timeRange: TimeRange): string {
  switch (timeRange) {
    case '1h':
    case '4h':
      return 'HH:mm'
    case '1d':
      return 'MMM dd HH:mm'
    case '1w':
      return 'MMM dd'
    case '1m':
    case '3m':
      return 'MMM dd'
    case '1y':
      return 'MMM yyyy'
    default:
      return 'MMM dd'
  }
}

function getTickFormat(timeRange: TimeRange): string {
  switch (timeRange) {
    case '1h':
    case '4h':
      return 'HH:mm'
    case '1d':
      return 'HH:mm'
    case '1w':
      return 'MMM dd'
    case '1m':
    case '3m':
      return 'MMM dd'
    case '1y':
      return 'MMM'
    default:
      return 'MMM dd'
  }
}