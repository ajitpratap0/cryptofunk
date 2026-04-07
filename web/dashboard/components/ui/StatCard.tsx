'use client'

import { ReactNode } from 'react'
import { TrendingUp, TrendingDown, Minus } from 'lucide-react'
import { cn, formatNumber, formatPercentage, formatCurrency } from '@/lib/utils'

interface StatCardProps {
  title: string
  value: number | string
  change?: number
  changeType?: 'percentage' | 'currency' | 'number'
  icon?: ReactNode
  sparklineData?: number[]
  /**
   * Hint that this card will eventually render a sparkline. Set this on
   * cards whose `sparklineData` is fetched async — the loading skeleton
   * uses it to reserve sparkline space and avoid CLS once data arrives.
   */
  hasSparkline?: boolean
  formatAs?: 'currency' | 'percentage' | 'number' | 'custom'
  prefix?: string
  suffix?: string
  loading?: boolean
  className?: string
  trend?: 'up' | 'down' | 'neutral'
  subtitle?: string
  valueClassName?: string
}

export function StatCard({
  title,
  value,
  change,
  changeType = 'number',
  icon,
  sparklineData,
  hasSparkline = false,
  formatAs = 'number',
  prefix,
  suffix,
  loading = false,
  className,
  trend,
  subtitle,
  valueClassName,
}: StatCardProps) {
  const formatValue = (val: number | string) => {
    if (typeof val === 'string') return val
    
    switch (formatAs) {
      case 'currency':
        return formatCurrency(val)
      case 'percentage':
        return formatPercentage(val)
      case 'custom':
        return `${prefix || ''}${formatNumber(val)}${suffix || ''}`
      default:
        return formatNumber(val)
    }
  }

  const formatChange = (changeVal: number) => {
    switch (changeType) {
      case 'currency':
        return formatCurrency(Math.abs(changeVal))
      case 'percentage':
        return formatPercentage(Math.abs(changeVal))
      default:
        return formatNumber(Math.abs(changeVal))
    }
  }

  const getChangeColor = (changeVal: number) => {
    if (changeVal > 0) return 'text-profit'
    if (changeVal < 0) return 'text-loss'
    return 'text-muted-foreground'
  }

  const getTrendIcon = (changeVal?: number) => {
    if (trend) {
      switch (trend) {
        case 'up': return <TrendingUp className="h-4 w-4 text-profit" />
        case 'down': return <TrendingDown className="h-4 w-4 text-loss" />
        case 'neutral': return <Minus className="h-4 w-4 text-muted-foreground" />
      }
    }
    
    if (changeVal === undefined) return null
    if (changeVal > 0) return <TrendingUp className="h-4 w-4 text-profit" />
    if (changeVal < 0) return <TrendingDown className="h-4 w-4 text-loss" />
    return <Minus className="h-4 w-4 text-muted-foreground" />
  }

  if (loading) {
    // Reserve sparkline space when the parent indicates one is coming, so
    // the skeleton and loaded card have the same height (no CLS on data).
    const reserveSparkline = hasSparkline || sparklineData !== undefined
    return (
      <div className={cn("metric-card min-h-[120px]", className)}>
        <div className="flex items-center justify-between">
          <div className="skeleton h-4 w-20" />
          {icon && <div className="skeleton h-5 w-5" />}
        </div>
        <div className="skeleton h-8 w-32 mt-1" />
        {subtitle && <div className="skeleton h-3 w-24 mt-1" />}
        {change !== undefined && (
          <div className="flex items-center gap-2 mt-1">
            <div className="skeleton h-4 w-4" />
            <div className="skeleton h-4 w-16" />
          </div>
        )}
        {reserveSparkline && <div className="skeleton h-8 w-full mt-4" />}
      </div>
    )
  }

  return (
    <div className={cn("metric-card min-h-[120px] group hover:shadow-md transition-shadow", className)}>
      <div className="flex items-center justify-between">
        <p className="metric-label">{title}</p>
        {icon && <div className="text-muted-foreground">{icon}</div>}
      </div>
      
      <div className={cn("metric-value number-animate", valueClassName)}>
        {formatValue(value)}
      </div>

      {subtitle && (
        <p className="text-xs text-muted-foreground">{subtitle}</p>
      )}
      
      {change !== undefined && (
        <div className="flex items-center gap-2">
          {getTrendIcon(change)}
          <span className={cn("metric-change", getChangeColor(change))}>
            {change > 0 && '+'}
            {formatChange(change)}
          </span>
        </div>
      )}
      
      {sparklineData && sparklineData.length > 0 && (
        <div className="mt-4">
          <MiniSparkline data={sparklineData} />
        </div>
      )}
    </div>
  )
}

interface MiniSparklineProps {
  data: number[]
  height?: number
  className?: string
}

function MiniSparkline({ data, height = 32, className }: MiniSparklineProps) {
  if (data.length < 2) return null

  const min = Math.min(...data)
  const max = Math.max(...data)
  const range = max - min || 1

  const points = data.map((value, index) => {
    const x = (index / (data.length - 1)) * 100
    const y = ((max - value) / range) * 100
    return `${x},${y}`
  }).join(' ')

  const lastValue = data[data.length - 1]
  const prevValue = data[data.length - 2]
  const isPositive = lastValue >= prevValue

  return (
    <div className={cn("relative", className)} style={{ height }}>
      <svg
        width="100%"
        height="100%"
        viewBox="0 0 100 100"
        preserveAspectRatio="none"
        className="overflow-visible"
      >
        <defs>
          <linearGradient id="sparkline-gradient" x1="0" y1="0" x2="0" y2="1">
            <stop 
              offset="0%" 
              stopColor={isPositive ? '#22c55e' : '#ef4444'} 
              stopOpacity="0.3"
            />
            <stop 
              offset="100%" 
              stopColor={isPositive ? '#22c55e' : '#ef4444'} 
              stopOpacity="0"
            />
          </linearGradient>
        </defs>
        
        <polyline
          points={`${points} 100,100 0,100`}
          fill="url(#sparkline-gradient)"
          className="transition-all duration-300"
        />
        
        <polyline
          points={points}
          fill="none"
          stroke={isPositive ? '#22c55e' : '#ef4444'}
          strokeWidth="2"
          className="transition-all duration-300"
        />
        
        {/* Last point indicator */}
        <circle
          cx={((data.length - 1) / (data.length - 1)) * 100}
          cy={((max - lastValue) / range) * 100}
          r="2"
          fill={isPositive ? '#22c55e' : '#ef4444'}
          className="transition-all duration-300"
        />
      </svg>
    </div>
  )
}