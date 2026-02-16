'use client'

import { Trade } from '@/lib/types'
import { 
  formatCurrency, 
  formatPercentage, 
  formatDateTime,
  cn 
} from '@/lib/utils'
import { 
  TrendingUp,
  TrendingDown,
  Clock,
  Target,
  Shield,
  Brain,
  BarChart3
} from 'lucide-react'

interface TradeDetailProps {
  trade: Trade
  className?: string
}

export function TradeDetail({ trade, className }: TradeDetailProps) {
  const calculateMetrics = () => {
    const leverageUsed = (trade.entryPrice * trade.quantity) / (Math.abs(trade.pnl) || 1)
    const riskReward = trade.exitPrice 
      ? Math.abs((trade.exitPrice - trade.entryPrice) / trade.entryPrice)
      : null

    return {
      leverageUsed: Math.round(leverageUsed * 100) / 100,
      riskReward: riskReward ? Math.round(riskReward * 10000) / 100 : null,
      duration: trade.exitTimestamp 
        ? new Date(trade.exitTimestamp).getTime() - new Date(trade.timestamp).getTime()
        : Date.now() - new Date(trade.timestamp).getTime(),
      notionalValue: trade.entryPrice * trade.quantity,
    }
  }

  const metrics = calculateMetrics()

  const formatDuration = (ms: number) => {
    const hours = Math.floor(ms / (1000 * 60 * 60))
    const minutes = Math.floor((ms % (1000 * 60 * 60)) / (1000 * 60))
    
    if (hours > 24) {
      const days = Math.floor(hours / 24)
      return `${days}d ${hours % 24}h`
    }
    if (hours > 0) {
      return `${hours}h ${minutes}m`
    }
    return `${minutes}m`
  }

  return (
    <div className={cn("bg-muted/30 p-6", className)}>
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Trade Information */}
        <div className="space-y-4">
          <div className="flex items-center gap-2 mb-4">
            <BarChart3 className="h-5 w-5 text-muted-foreground" />
            <h4 className="font-semibold">Trade Details</h4>
          </div>
          
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <label className="text-muted-foreground block">Trade ID</label>
              <span className="font-mono text-xs">{trade.id}</span>
            </div>
            
            <div>
              <label className="text-muted-foreground block">Symbol</label>
              <span className="font-mono font-medium">{trade.symbol}</span>
            </div>
            
            <div>
              <label className="text-muted-foreground block">Side</label>
              <div className="flex items-center gap-2">
                {trade.side === 'long' 
                  ? <TrendingUp className="h-4 w-4 text-profit" />
                  : <TrendingDown className="h-4 w-4 text-loss" />
                }
                <span className={cn(
                  "capitalize font-medium",
                  trade.side === 'long' ? "text-profit" : "text-loss"
                )}>
                  {trade.side}
                </span>
              </div>
            </div>
            
            <div>
              <label className="text-muted-foreground block">Quantity</label>
              <span className="font-mono">{trade.quantity}</span>
            </div>
            
            <div>
              <label className="text-muted-foreground block">Entry Price</label>
              <span className="font-mono">{formatCurrency(trade.entryPrice)}</span>
            </div>
            
            <div>
              <label className="text-muted-foreground block">Current Price</label>
              <span className="font-mono">{formatCurrency(trade.currentPrice)}</span>
            </div>
            
            {trade.exitPrice && (
              <div>
                <label className="text-muted-foreground block">Exit Price</label>
                <span className="font-mono">{formatCurrency(trade.exitPrice)}</span>
              </div>
            )}
            
            <div>
              <label className="text-muted-foreground block">Notional Value</label>
              <span className="font-mono">{formatCurrency(metrics.notionalValue)}</span>
            </div>
          </div>
        </div>

        {/* Performance Metrics */}
        <div className="space-y-4">
          <div className="flex items-center gap-2 mb-4">
            <Target className="h-5 w-5 text-muted-foreground" />
            <h4 className="font-semibold">Performance</h4>
          </div>
          
          <div className="space-y-3">
            <div className="p-3 bg-card rounded-lg border border-border">
              <label className="text-muted-foreground text-sm block">P&L</label>
              <div className="flex items-baseline gap-2">
                <span className={cn(
                  "text-lg font-bold",
                  trade.pnl > 0 ? "text-profit" : trade.pnl < 0 ? "text-loss" : "text-muted-foreground"
                )}>
                  {trade.pnl > 0 ? '+' : ''}{formatCurrency(trade.pnl)}
                </span>
                <span className={cn(
                  "text-sm font-medium",
                  trade.pnlPercent > 0 ? "text-profit" : trade.pnlPercent < 0 ? "text-loss" : "text-muted-foreground"
                )}>
                  ({trade.pnlPercent > 0 ? '+' : ''}{formatPercentage(trade.pnlPercent)})
                </span>
              </div>
            </div>
            
            <div className="grid grid-cols-2 gap-3 text-sm">
              <div>
                <label className="text-muted-foreground block">Duration</label>
                <div className="flex items-center gap-1">
                  <Clock className="h-3 w-3 text-muted-foreground" />
                  <span className="font-mono">{formatDuration(metrics.duration)}</span>
                </div>
              </div>
              
              {metrics.riskReward && (
                <div>
                  <label className="text-muted-foreground block">Risk/Reward</label>
                  <span className="font-mono">{formatPercentage(metrics.riskReward)}</span>
                </div>
              )}
              
              <div>
                <label className="text-muted-foreground block">Entry Time</label>
                <span className="text-xs">{formatDateTime(trade.timestamp)}</span>
              </div>
              
              {trade.exitTimestamp && (
                <div>
                  <label className="text-muted-foreground block">Exit Time</label>
                  <span className="text-xs">{formatDateTime(trade.exitTimestamp)}</span>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Agent & Strategy */}
        <div className="space-y-4">
          <div className="flex items-center gap-2 mb-4">
            <Brain className="h-5 w-5 text-muted-foreground" />
            <h4 className="font-semibold">Agent Analysis</h4>
          </div>
          
          <div className="space-y-3">
            <div className="p-3 bg-card rounded-lg border border-border">
              <label className="text-muted-foreground text-sm block">Agent</label>
              <span className="font-medium">{trade.agent}</span>
            </div>
            
            <div className="p-3 bg-card rounded-lg border border-border">
              <label className="text-muted-foreground text-sm block">Confidence</label>
              <div className="flex items-center gap-3">
                <div className="flex-1 h-2 rounded-full bg-muted overflow-hidden">
                  <div 
                    className={cn(
                      "h-full transition-all",
                      trade.confidence >= 80 ? "bg-profit" : 
                      trade.confidence >= 60 ? "bg-warning" : "bg-loss"
                    )}
                    style={{ width: `${trade.confidence}%` }}
                  />
                </div>
                <span className="font-mono text-sm">{trade.confidence}%</span>
              </div>
            </div>
            
            <div className="p-3 bg-card rounded-lg border border-border">
              <label className="text-muted-foreground text-sm block">Status</label>
              <div className="flex items-center gap-2">
                <div className={cn(
                  "w-2 h-2 rounded-full",
                  trade.status === 'open' && "bg-info",
                  trade.status === 'closed' && "bg-profit",
                  trade.status === 'pending' && "bg-warning"
                )} />
                <span className="capitalize font-medium">{trade.status}</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Reasoning */}
      {trade.reasoning && (
        <div className="mt-6">
          <div className="flex items-center gap-2 mb-3">
            <Shield className="h-5 w-5 text-muted-foreground" />
            <h4 className="font-semibold">Trade Reasoning</h4>
          </div>
          
          <div className="p-4 bg-card rounded-lg border border-border">
            <p className="text-sm leading-relaxed">{trade.reasoning}</p>
          </div>
        </div>
      )}

      {/* Risk Metrics */}
      <div className="mt-6 p-4 bg-card/50 rounded-lg border border-border">
        <h5 className="font-medium mb-3 text-sm text-muted-foreground">Risk Assessment</h5>
        <div className="grid grid-cols-3 gap-4 text-xs">
          <div>
            <span className="text-muted-foreground block">Position Size</span>
            <span className="font-mono">{formatPercentage((metrics.notionalValue / 100000) * 100)}</span>
          </div>
          <div>
            <span className="text-muted-foreground block">Max Risk</span>
            <span className="font-mono text-loss">-2.5%</span>
          </div>
          <div>
            <span className="text-muted-foreground block">Risk Score</span>
            <span className={cn(
              "font-medium",
              trade.confidence >= 80 ? "text-profit" : 
              trade.confidence >= 60 ? "text-warning" : "text-loss"
            )}>
              {trade.confidence >= 80 ? 'Low' : trade.confidence >= 60 ? 'Medium' : 'High'}
            </span>
          </div>
        </div>
      </div>
    </div>
  )
}