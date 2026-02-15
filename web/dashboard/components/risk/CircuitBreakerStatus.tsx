'use client'

import { CircuitBreaker } from '@/lib/types'
import { cn, formatPercentage } from '@/lib/utils'
import { StatusDot } from '@/components/ui/StatusDot'
import { 
  Shield, 
  AlertTriangle, 
  CheckCircle, 
  XCircle,
  TrendingDown,
  Activity,
  DollarSign,
  BarChart3
} from 'lucide-react'

interface CircuitBreakerStatusProps {
  circuitBreakers: CircuitBreaker[]
  loading?: boolean
  className?: string
}

export function CircuitBreakerStatus({ 
  circuitBreakers, 
  loading = false, 
  className 
}: CircuitBreakerStatusProps) {
  if (loading) {
    return (
      <div className={cn("bg-card border border-border rounded-lg p-6", className)}>
        <div className="flex items-center gap-2 mb-4">
          <Shield className="h-5 w-5" />
          <h3 className="font-semibold">Circuit Breakers</h3>
        </div>
        <div className="space-y-3">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="flex items-center justify-between p-3 bg-muted/30 rounded-lg">
              <div className="skeleton h-4 w-32" />
              <div className="skeleton h-6 w-16" />
            </div>
          ))}
        </div>
      </div>
    )
  }

  const getIcon = (name: string) => {
    const iconMap: Record<string, React.ReactNode> = {
      'Max Daily Loss': <TrendingDown className="h-4 w-4" />,
      'Max Drawdown': <Activity className="h-4 w-4" />,
      'Position Concentration': <BarChart3 className="h-4 w-4" />,
      'Leverage Limit': <DollarSign className="h-4 w-4" />,
    }
    return iconMap[name] || <Shield className="h-4 w-4" />
  }

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'normal':
        return <CheckCircle className="h-4 w-4 text-profit" />
      case 'warning':
        return <AlertTriangle className="h-4 w-4 text-warning" />
      case 'triggered':
        return <XCircle className="h-4 w-4 text-loss" />
      default:
        return <Shield className="h-4 w-4 text-muted-foreground" />
    }
  }

  const getProgressColor = (status: string) => {
    switch (status) {
      case 'normal':
        return 'bg-profit'
      case 'warning':
        return 'bg-warning'
      case 'triggered':
        return 'bg-loss'
      default:
        return 'bg-muted'
    }
  }

  const getProgressPercentage = (current: number, threshold: number) => {
    return Math.min((Math.abs(current) / Math.abs(threshold)) * 100, 100)
  }

  const overallStatus = circuitBreakers.some(cb => cb.status === 'triggered') 
    ? 'triggered'
    : circuitBreakers.some(cb => cb.status === 'warning')
    ? 'warning'
    : 'normal'

  return (
    <div className={cn("bg-card border border-border rounded-lg p-6", className)}>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <Shield className="h-5 w-5" />
          <div>
            <h3 className="font-semibold">Circuit Breakers</h3>
            <p className="text-sm text-muted-foreground">Risk protection systems</p>
          </div>
        </div>

        <div className="text-right">
          <StatusDot 
            status={overallStatus === 'triggered' ? 'error' : overallStatus === 'warning' ? 'warning' : 'healthy'} 
            label={null} 
          />
          <div className="text-xs text-muted-foreground mt-1 capitalize">
            {overallStatus}
          </div>
        </div>
      </div>

      {/* Circuit Breaker List */}
      <div className="space-y-4">
        {circuitBreakers.map((breaker, index) => {
          const progressPercentage = getProgressPercentage(breaker.currentValue, breaker.threshold)
          
          return (
            <div 
              key={index}
              className={cn(
                "p-4 rounded-lg border transition-all",
                breaker.status === 'normal' && "bg-background border-border",
                breaker.status === 'warning' && "bg-warning/10 border-warning/30",
                breaker.status === 'triggered' && "bg-loss/10 border-loss/30"
              )}
            >
              <div className="flex items-start justify-between mb-3">
                <div className="flex items-center gap-3">
                  <div className={cn(
                    "p-2 rounded-lg",
                    breaker.status === 'normal' && "bg-muted text-muted-foreground",
                    breaker.status === 'warning' && "bg-warning/20 text-warning",
                    breaker.status === 'triggered' && "bg-loss/20 text-loss"
                  )}>
                    {getIcon(breaker.name)}
                  </div>
                  
                  <div>
                    <div className="font-medium">{breaker.name}</div>
                    <div className="text-xs text-muted-foreground">
                      {breaker.description}
                    </div>
                  </div>
                </div>

                {getStatusIcon(breaker.status)}
              </div>

              {/* Progress Bar */}
              <div className="mb-3">
                <div className="flex items-center justify-between text-sm mb-1">
                  <span className="text-muted-foreground">Current</span>
                  <span className="font-mono">
                    {breaker.name.includes('Percentage') || breaker.name.includes('Loss') || breaker.name.includes('Drawdown') || breaker.name.includes('Concentration')
                      ? formatPercentage(breaker.currentValue)
                      : breaker.currentValue.toFixed(1)
                    }
                  </span>
                </div>
                
                <div className="w-full h-2 bg-muted rounded-full overflow-hidden">
                  <div 
                    className={cn("h-full transition-all duration-300", getProgressColor(breaker.status))}
                    style={{ width: `${progressPercentage}%` }}
                  />
                </div>
                
                <div className="flex items-center justify-between text-xs text-muted-foreground mt-1">
                  <span>0</span>
                  <span>
                    Threshold: {breaker.name.includes('Percentage') || breaker.name.includes('Loss') || breaker.name.includes('Drawdown') || breaker.name.includes('Concentration')
                      ? formatPercentage(breaker.threshold)
                      : breaker.threshold.toFixed(1)
                    }
                  </span>
                </div>
              </div>

              {/* Status Message */}
              {breaker.status === 'triggered' && (
                <div className="p-2 bg-loss/20 border border-loss/30 rounded text-sm">
                  <div className="font-medium text-loss">Circuit Breaker Triggered</div>
                  <div className="text-loss/80 text-xs mt-1">
                    Trading has been automatically halted
                  </div>
                </div>
              )}

              {breaker.status === 'warning' && (
                <div className="p-2 bg-warning/20 border border-warning/30 rounded text-sm">
                  <div className="font-medium text-warning">Approaching Threshold</div>
                  <div className="text-warning/80 text-xs mt-1">
                    Monitor closely - {((breaker.threshold - breaker.currentValue) / breaker.threshold * 100).toFixed(1)}% margin remaining
                  </div>
                </div>
              )}

              {breaker.status === 'normal' && progressPercentage > 50 && (
                <div className="p-2 bg-info/10 border border-info/20 rounded text-sm">
                  <div className="font-medium text-info">Within Safe Range</div>
                  <div className="text-info/80 text-xs mt-1">
                    {(100 - progressPercentage).toFixed(1)}% buffer remaining
                  </div>
                </div>
              )}
            </div>
          )
        })}
      </div>

      {/* Summary */}
      <div className="mt-6 pt-4 border-t border-border">
        <div className="grid grid-cols-3 gap-4 text-center text-sm">
          <div>
            <div className="text-muted-foreground">Normal</div>
            <div className="font-bold text-profit">
              {circuitBreakers.filter(cb => cb.status === 'normal').length}
            </div>
          </div>
          <div>
            <div className="text-muted-foreground">Warning</div>
            <div className="font-bold text-warning">
              {circuitBreakers.filter(cb => cb.status === 'warning').length}
            </div>
          </div>
          <div>
            <div className="text-muted-foreground">Triggered</div>
            <div className="font-bold text-loss">
              {circuitBreakers.filter(cb => cb.status === 'triggered').length}
            </div>
          </div>
        </div>
      </div>

      {/* Last Update */}
      <div className="mt-4 text-xs text-muted-foreground text-center">
        Last updated: {new Date().toLocaleTimeString()}
      </div>
    </div>
  )
}