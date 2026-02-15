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
import { EquityPoint, TimeRange } from '@/lib/types'
import { formatCurrency, formatDateTime, cn } from '@/lib/utils'

interface EquityCurveProps {
  data: EquityPoint[]
  timeRange?: TimeRange
  height?: number
  showGrid?: boolean
  showTooltip?: boolean
  loading?: boolean
  className?: string
  highlightPeriod?: {
    start: string
    end: string
    label: string
  }
}

export function EquityCurve({
  data,
  timeRange = '1m',
  height = 300,
  showGrid = true,
  showTooltip = true,
  loading = false,
  className,
  highlightPeriod
}: EquityCurveProps) {
  if (loading) {
    return (
      <div className={cn("chart-container", className)} style={{ height }}>
        <div className="chart-loading">
          <div className="flex items-center gap-2">
            <div className="w-4 h-4 border-2 border-muted-foreground border-t-transparent rounded-full animate-spin" />
            <span className="text-sm text-muted-foreground">Loading equity curve...</span>
          </div>
        </div>
      </div>
    )
  }

  if (!data || data.length === 0) {
    return (
      <div className={cn("chart-container flex items-center justify-center", className)} style={{ height }}>
        <div className="text-center">
          <div className="text-muted-foreground text-sm">No equity data available</div>
          <div className="text-muted-foreground text-xs mt-1">Data will appear once trading begins</div>
        </div>
      </div>
    )
  }

  // Sanitize data - filter out entries with non-finite values
  const safeData = data.filter(d => 
    Number.isFinite(d.equity) && Number.isFinite(d.pnl ?? 0)
  )

  if (safeData.length === 0) {
    return (
      <div className={cn("chart-container flex items-center justify-center", className)} style={{ height }}>
        <div className="text-center">
          <div className="text-muted-foreground text-sm">No valid equity data available</div>
        </div>
      </div>
    )
  }

  // Calculate some metrics
  const firstEquity = safeData[0]?.equity || 0
  const lastEquity = safeData[safeData.length - 1]?.equity || 0
  const totalReturn = lastEquity - firstEquity
  const totalReturnPercent = firstEquity > 0 ? (totalReturn / firstEquity) * 100 : 0
  const highWaterMark = Math.max(...safeData.map(d => d.equity))
  const currentDrawdown = highWaterMark > 0 ? ((highWaterMark - lastEquity) / highWaterMark) * 100 : 0

  // Format data for chart
  const chartData = safeData.map(point => ({
    ...point,
    timestamp: new Date(point.timestamp).getTime(),
    formattedTime: format(parseISO(point.timestamp), getTimeFormat(timeRange)),
  }))

  const CustomTooltip = ({ active, payload, label }: any) => {
    if (active && payload && payload.length) {
      const data = payload[0].payload
      return (
        <div className="bg-card border border-border rounded-lg p-3 shadow-lg">
          <p className="text-sm font-medium">{formatDateTime(new Date(label))}</p>
          <div className="mt-2 space-y-1">
            <div className="flex justify-between gap-4">
              <span className="text-muted-foreground text-sm">Equity:</span>
              <span className="font-mono text-sm">{formatCurrency(data.equity)}</span>
            </div>
            {data.pnl && (
              <div className="flex justify-between gap-4">
                <span className="text-muted-foreground text-sm">P&L:</span>
                <span className={cn(
                  "font-mono text-sm",
                  data.pnl > 0 ? "text-profit" : data.pnl < 0 ? "text-loss" : "text-muted-foreground"
                )}>
                  {data.pnl > 0 ? '+' : ''}{formatCurrency(data.pnl)}
                </span>
              </div>
            )}
          </div>
        </div>
      )
    }
    return null
  }

  return (
    <div className={cn("chart-container", className)}>
      {/* Chart Header */}
      <div className="absolute top-4 left-4 z-10 bg-card/80 backdrop-blur-sm rounded-lg p-3 border border-border">
        <h3 className="font-semibold text-sm mb-2">Equity Curve</h3>
        <div className="grid grid-cols-2 gap-4 text-xs">
          <div>
            <div className="text-muted-foreground">Current Equity</div>
            <div className="font-mono font-medium">{formatCurrency(lastEquity)}</div>
          </div>
          <div>
            <div className="text-muted-foreground">Total Return</div>
            <div className={cn(
              "font-mono font-medium",
              totalReturn > 0 ? "text-profit" : totalReturn < 0 ? "text-loss" : "text-muted-foreground"
            )}>
              {totalReturn > 0 ? '+' : ''}{formatCurrency(totalReturn)} ({totalReturnPercent > 0 ? '+' : ''}{totalReturnPercent.toFixed(2)}%)
            </div>
          </div>
          <div>
            <div className="text-muted-foreground">High Water Mark</div>
            <div className="font-mono font-medium">{formatCurrency(highWaterMark)}</div>
          </div>
          <div>
            <div className="text-muted-foreground">Current Drawdown</div>
            <div className={cn(
              "font-mono font-medium",
              currentDrawdown > 5 ? "text-loss" : currentDrawdown > 2 ? "text-warning" : "text-profit"
            )}>
              -{currentDrawdown.toFixed(2)}%
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
              domain={['dataMin - 1000', 'dataMax + 1000']}
              tickFormatter={(value) => formatCurrency(value, 'USD', { 
                notation: 'compact',
                minimumFractionDigits: 0,
                maximumFractionDigits: 0 
              })}
              stroke="hsl(var(--muted-foreground))"
              fontSize={12}
              tickLine={false}
              axisLine={false}
            />

            {showTooltip && <Tooltip content={<CustomTooltip />} />}

            {/* High water mark line */}
            <ReferenceLine 
              y={highWaterMark} 
              stroke="#8B5CF6" 
              strokeDasharray="5 5"
              strokeOpacity={0.7}
              label={{ value: "High Water Mark", position: "insideTopRight", fontSize: 10 }}
            />

            {/* Highlight period */}
            {highlightPeriod && (
              <>
                <ReferenceLine 
                  x={new Date(highlightPeriod.start).getTime()} 
                  stroke="#F59E0B" 
                  strokeDasharray="3 3"
                />
                <ReferenceLine 
                  x={new Date(highlightPeriod.end).getTime()} 
                  stroke="#F59E0B" 
                  strokeDasharray="3 3"
                />
              </>
            )}

            <defs>
              <linearGradient id="equityGradient" x1="0" y1="0" x2="0" y2="1">
                <stop 
                  offset="0%" 
                  stopColor={totalReturn >= 0 ? "#22C55E" : "#EF4444"} 
                  stopOpacity={0.3} 
                />
                <stop 
                  offset="100%" 
                  stopColor={totalReturn >= 0 ? "#22C55E" : "#EF4444"} 
                  stopOpacity={0} 
                />
              </linearGradient>
            </defs>
            
            <Area
              type="monotone"
              dataKey="equity"
              stroke={totalReturn >= 0 ? "#22C55E" : "#EF4444"}
              strokeWidth={2}
              fill="url(#equityGradient)"
              fillOpacity={1}
              dot={false}
              activeDot={{ 
                r: 4, 
                stroke: totalReturn >= 0 ? "#22C55E" : "#EF4444",
                strokeWidth: 2,
                fill: "hsl(var(--background))"
              }}
            />
          </AreaChart>
        </ResponsiveContainer>
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