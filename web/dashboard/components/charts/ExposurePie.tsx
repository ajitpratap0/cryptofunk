'use client'

import { 
  PieChart, 
  Pie, 
  Cell, 
  ResponsiveContainer, 
  Tooltip, 
  Legend 
} from 'recharts'
import { cn, formatCurrency, formatPercentage } from '@/lib/utils'
import { CHART_COLORS } from '@/lib/constants'

interface ExposureData {
  symbol: string
  exposure: number
  percentage: number
  value: number
  side?: 'long' | 'short'
}

interface ExposurePieProps {
  data: ExposureData[]
  title?: string
  height?: number
  showLegend?: boolean
  showLabels?: boolean
  loading?: boolean
  className?: string
  type?: 'asset' | 'side' | 'size'
}

export function ExposurePie({
  data,
  title = 'Portfolio Exposure',
  height = 300,
  showLegend = true,
  showLabels = false,
  loading = false,
  className,
  type = 'asset'
}: ExposurePieProps) {
  if (loading) {
    return (
      <div className={cn("chart-container", className)} style={{ height }}>
        <div className="chart-loading">
          <div className="flex items-center gap-2">
            <div className="w-4 h-4 border-2 border-muted-foreground border-t-transparent rounded-full animate-spin" />
            <span className="text-sm text-muted-foreground">Loading exposure data...</span>
          </div>
        </div>
      </div>
    )
  }

  if (!data || data.length === 0) {
    return (
      <div className={cn("chart-container flex items-center justify-center", className)} style={{ height }}>
        <div className="text-center">
          <div className="text-muted-foreground text-sm">No exposure data available</div>
          <div className="text-muted-foreground text-xs mt-1">Data will appear when positions are open</div>
        </div>
      </div>
    )
  }

  // Process data for chart
  const chartData = data
    .filter(item => item.percentage > 0)
    .map((item, index) => ({
      ...item,
      color: getColorForItem(item, index),
      displayName: formatDisplayName(item.symbol),
    }))
    .sort((a, b) => b.percentage - a.percentage)

  const totalExposure = chartData.reduce((sum, item) => sum + item.value, 0)
  const largestExposure = chartData[0]
  const concentrationRisk = largestExposure ? largestExposure.percentage : 0

  function getColorForItem(item: ExposureData, index: number): string {
    if (type === 'side') {
      return item.side === 'long' ? '#22C55E' : '#EF4444'
    }
    return CHART_COLORS[index % CHART_COLORS.length]
  }

  function formatDisplayName(symbol: string): string {
    if (type === 'asset') {
      return symbol.replace('/USDT', '').replace('/USD', '')
    }
    return symbol
  }

  const CustomTooltip = ({ active, payload }: any) => {
    if (active && payload && payload.length) {
      const data = payload[0].payload
      return (
        <div className="bg-card border border-border rounded-lg p-3 shadow-lg">
          <div className="font-medium text-sm mb-2">{data.displayName}</div>
          <div className="space-y-1 text-sm">
            <div className="flex justify-between gap-4">
              <span className="text-muted-foreground">Exposure:</span>
              <span className="font-mono">{formatCurrency(data.value)}</span>
            </div>
            <div className="flex justify-between gap-4">
              <span className="text-muted-foreground">Percentage:</span>
              <span className="font-mono">{formatPercentage(data.percentage)}</span>
            </div>
            {data.side && (
              <div className="flex justify-between gap-4">
                <span className="text-muted-foreground">Side:</span>
                <span className={cn(
                  "capitalize font-medium",
                  data.side === 'long' ? "text-profit" : "text-loss"
                )}>
                  {data.side}
                </span>
              </div>
            )}
          </div>
        </div>
      )
    }
    return null
  }

  const CustomLabel = ({ cx, cy, midAngle, innerRadius, outerRadius, percent }: any) => {
    if (percent < 0.05) return null // Don't show labels for slices less than 5%

    const RADIAN = Math.PI / 180
    const radius = innerRadius + (outerRadius - innerRadius) * 0.5
    const x = cx + radius * Math.cos(-midAngle * RADIAN)
    const y = cy + radius * Math.sin(-midAngle * RADIAN)

    return (
      <text 
        x={x} 
        y={y} 
        fill="white" 
        textAnchor={x > cx ? 'start' : 'end'} 
        dominantBaseline="central"
        fontSize={12}
        fontWeight="medium"
      >
        {`${(percent * 100).toFixed(0)}%`}
      </text>
    )
  }

  const CustomLegend = ({ payload }: any) => {
    return (
      <div className="flex flex-wrap gap-3 justify-center mt-4">
        {payload.map((entry: any, index: number) => (
          <div key={index} className="flex items-center gap-2">
            <div 
              className="w-3 h-3 rounded-sm" 
              style={{ backgroundColor: entry.color }}
            />
            <span className="text-xs text-foreground">{entry.value}</span>
            <span className="text-xs text-muted-foreground">
              ({formatPercentage(entry.payload.percentage)})
            </span>
          </div>
        ))}
      </div>
    )
  }

  return (
    <div className={cn("chart-container", className)}>
      {/* Chart Header */}
      <div className="absolute top-4 left-4 z-10 bg-card/80 backdrop-blur-sm rounded-lg p-3 border border-border">
        <h3 className="font-semibold text-sm">{title}</h3>
        <div className="mt-2 text-xs space-y-1">
          <div className="flex justify-between gap-4">
            <span className="text-muted-foreground">Total Exposure:</span>
            <span className="font-mono">{formatCurrency(totalExposure)}</span>
          </div>
          <div className="flex justify-between gap-4">
            <span className="text-muted-foreground">Largest Position:</span>
            <span className="font-mono">{formatPercentage(concentrationRisk)}</span>
          </div>
          <div className="flex justify-between gap-4">
            <span className="text-muted-foreground">Positions:</span>
            <span className="font-mono">{chartData.length}</span>
          </div>
        </div>
      </div>

      {/* Risk Warning */}
      {concentrationRisk > 50 && (
        <div className="absolute top-4 right-4 bg-warning/20 border border-warning/50 rounded-lg p-2 text-xs">
          <div className="text-warning font-medium">High Concentration Risk</div>
          <div className="text-warning/80">
            {largestExposure?.displayName} = {formatPercentage(concentrationRisk)}
          </div>
        </div>
      )}

      {/* Chart */}
      <div style={{ height }}>
        <ResponsiveContainer width="100%" height="100%">
          <PieChart margin={{ top: 80, right: 30, bottom: showLegend ? 60 : 20, left: 30 }}>
            <Pie
              data={chartData}
              cx="50%"
              cy="50%"
              labelLine={false}
              label={showLabels ? CustomLabel : false}
              outerRadius={Math.min(height * 0.3, 80)}
              innerRadius={Math.min(height * 0.15, 40)}
              fill="#8884d8"
              dataKey="percentage"
              nameKey="displayName"
              stroke="hsl(var(--background))"
              strokeWidth={2}
            >
              {chartData.map((entry, index) => (
                <Cell key={`cell-${index}`} fill={entry.color} />
              ))}
            </Pie>
            <Tooltip content={<CustomTooltip />} />
            {showLegend && <Legend content={<CustomLegend />} />}
          </PieChart>
        </ResponsiveContainer>
      </div>

      {/* Risk Levels */}
      <div className="absolute bottom-4 right-4 bg-card/80 backdrop-blur-sm rounded-lg p-3 border border-border text-xs">
        <div className="font-medium text-muted-foreground mb-2">Risk Level</div>
        <div className="space-y-1">
          <div className={cn(
            "flex items-center gap-2",
            concentrationRisk < 30 ? "text-profit" : "text-muted-foreground"
          )}>
            <div className="w-2 h-2 rounded-full bg-profit" />
            <span>Low ({"<"}30%)</span>
          </div>
          <div className={cn(
            "flex items-center gap-2",
            concentrationRisk >= 30 && concentrationRisk < 50 ? "text-warning" : "text-muted-foreground"
          )}>
            <div className="w-2 h-2 rounded-full bg-warning" />
            <span>Medium (30-50%)</span>
          </div>
          <div className={cn(
            "flex items-center gap-2",
            concentrationRisk >= 50 ? "text-loss" : "text-muted-foreground"
          )}>
            <div className="w-2 h-2 rounded-full bg-loss" />
            <span>High ({"≥"}50%)</span>
          </div>
        </div>
      </div>
    </div>
  )
}