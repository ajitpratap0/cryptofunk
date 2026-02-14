'use client'

import { Agent } from '@/lib/types'
import { 
  formatCurrency, 
  formatPercentage, 
  formatRelativeTime,
  cn 
} from '@/lib/utils'
import { StatusDot } from '@/components/ui/StatusDot'
import { 
  Bot,
  TrendingUp,
  TrendingDown,
  Activity,
  Target,
  Clock,
  Zap,
  MoreVertical,
  Play,
  Pause,
  Square,
  Settings
} from 'lucide-react'
import { useState } from 'react'

interface AgentCardProps {
  agent: Agent
  onAction?: (action: 'start' | 'stop' | 'pause' | 'configure') => void
  className?: string
}

export function AgentCard({ agent, onAction, className }: AgentCardProps) {
  const [showActions, setShowActions] = useState(false)

  const getAgentTypeIcon = (type: string) => {
    switch (type) {
      case 'momentum':
        return <TrendingUp className="h-5 w-5" />
      case 'mean_reversion':
        return <TrendingDown className="h-5 w-5" />
      case 'scalper':
        return <Zap className="h-5 w-5" />
      case 'ml_prediction':
        return <Bot className="h-5 w-5" />
      default:
        return <Activity className="h-5 w-5" />
    }
  }

  const getAgentTypeColor = (type: string) => {
    switch (type) {
      case 'momentum':
        return 'text-profit'
      case 'mean_reversion':
        return 'text-info'
      case 'scalper':
        return 'text-warning'
      case 'ml_prediction':
        return 'text-purple-400'
      default:
        return 'text-muted-foreground'
    }
  }

  const getPerformanceGrade = (winRate: number, totalPnl: number) => {
    if (winRate >= 70 && totalPnl > 1000) return { grade: 'A+', color: 'text-profit' }
    if (winRate >= 60 && totalPnl > 500) return { grade: 'A', color: 'text-profit' }
    if (winRate >= 55 && totalPnl > 0) return { grade: 'B+', color: 'text-info' }
    if (winRate >= 50 && totalPnl > -500) return { grade: 'B', color: 'text-warning' }
    return { grade: 'C', color: 'text-loss' }
  }

  const performance = getPerformanceGrade(agent.winRate, agent.totalPnl)

  const getActionControls = () => {
    switch (agent.status) {
      case 'active':
        return (
          <>
            <button
              onClick={() => onAction?.('pause')}
              className="flex items-center gap-2 w-full px-3 py-2 text-sm hover:bg-muted/50 transition-colors"
            >
              <Pause className="h-4 w-4" />
              Pause Agent
            </button>
            <button
              onClick={() => onAction?.('stop')}
              className="flex items-center gap-2 w-full px-3 py-2 text-sm hover:bg-muted/50 transition-colors text-loss"
            >
              <Square className="h-4 w-4" />
              Stop Agent
            </button>
          </>
        )
      case 'idle':
      case 'disabled':
        return (
          <button
            onClick={() => onAction?.('start')}
            className="flex items-center gap-2 w-full px-3 py-2 text-sm hover:bg-muted/50 transition-colors text-profit"
          >
            <Play className="h-4 w-4" />
            Start Agent
          </button>
        )
      default:
        return null
    }
  }

  return (
    <div className={cn(
      "bg-card border border-border rounded-lg p-6 space-y-4 hover:shadow-md transition-all",
      className
    )}>
      {/* Header */}
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-3">
          <div className={cn(
            "p-2 rounded-lg bg-muted/50",
            getAgentTypeColor(agent.type)
          )}>
            {getAgentTypeIcon(agent.type)}
          </div>
          
          <div>
            <h3 className="font-semibold">{agent.name}</h3>
            <div className="flex items-center gap-2">
              <span className="text-sm text-muted-foreground capitalize">
                {agent.type.replace('_', ' ')}
              </span>
              <span className="text-xs text-muted-foreground">
                v{agent.version}
              </span>
            </div>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <StatusDot status={agent.status} label={null} />
          
          <div className="relative">
            <button
              onClick={() => setShowActions(!showActions)}
              className="p-1 hover:bg-muted/50 rounded transition-colors"
            >
              <MoreVertical className="h-4 w-4" />
            </button>

            {showActions && (
              <div className="absolute right-0 top-full mt-1 w-36 bg-card border border-border rounded-lg shadow-lg py-1 z-10">
                {getActionControls()}
                <div className="border-t border-border my-1" />
                <button
                  onClick={() => onAction?.('configure')}
                  className="flex items-center gap-2 w-full px-3 py-2 text-sm hover:bg-muted/50 transition-colors"
                >
                  <Settings className="h-4 w-4" />
                  Configure
                </button>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Performance Grade */}
      <div className="flex items-center justify-between">
        <div>
          <div className="text-2xl font-bold">{performance.grade}</div>
          <div className="text-xs text-muted-foreground">Performance Grade</div>
        </div>
        
        <div className="text-right">
          <div className={cn(
            "text-lg font-bold",
            agent.totalPnl > 0 ? "text-profit" : agent.totalPnl < 0 ? "text-loss" : "text-muted-foreground"
          )}>
            {agent.totalPnl > 0 ? '+' : ''}{formatCurrency(agent.totalPnl)}
          </div>
          <div className="text-xs text-muted-foreground">Total P&L</div>
        </div>
      </div>

      {/* Metrics Grid */}
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-sm text-muted-foreground">Win Rate</span>
            <span className="font-mono font-medium">{formatPercentage(agent.winRate)}</span>
          </div>
          
          <div className="flex items-center justify-between">
            <span className="text-sm text-muted-foreground">Avg Return</span>
            <span className={cn(
              "font-mono font-medium",
              agent.avgReturn > 0 ? "text-profit" : agent.avgReturn < 0 ? "text-loss" : "text-muted-foreground"
            )}>
              {agent.avgReturn > 0 ? '+' : ''}{formatPercentage(agent.avgReturn)}
            </span>
          </div>
        </div>

        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-sm text-muted-foreground">Total Trades</span>
            <span className="font-mono font-medium">{agent.totalTrades}</span>
          </div>
          
          <div className="flex items-center justify-between">
            <span className="text-sm text-muted-foreground">Active Pos.</span>
            <span className="font-mono font-medium">{agent.activePositions}</span>
          </div>
        </div>
      </div>

      {/* Win Rate Progress Bar */}
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <span className="text-sm text-muted-foreground">Win Rate Progress</span>
          <span className="text-xs text-muted-foreground">{agent.winRate.toFixed(1)}%</span>
        </div>
        <div className="w-full h-2 bg-muted rounded-full overflow-hidden">
          <div 
            className={cn(
              "h-full transition-all duration-300",
              agent.winRate >= 70 ? "bg-profit" : 
              agent.winRate >= 50 ? "bg-warning" : "bg-loss"
            )}
            style={{ width: `${Math.min(agent.winRate, 100)}%` }}
          />
        </div>
      </div>

      {/* Last Action */}
      <div className="pt-2 border-t border-border">
        <div className="flex items-center gap-2">
          <Clock className="h-4 w-4 text-muted-foreground" />
          <span className="text-sm text-muted-foreground">Last action:</span>
        </div>
        <div className="mt-1">
          <span className="text-sm font-medium">{agent.lastAction}</span>
          <div className="text-xs text-muted-foreground">
            {formatRelativeTime(agent.timestamp)}
          </div>
        </div>
      </div>

      {/* Status Banner */}
      {agent.status === 'error' && (
        <div className="p-3 bg-loss/10 border border-loss/20 rounded-lg">
          <div className="text-sm font-medium text-loss">Agent Error</div>
          <div className="text-xs text-loss/80 mt-1">
            Check logs for details
          </div>
        </div>
      )}

      {agent.status === 'idle' && agent.totalTrades > 0 && (
        <div className="p-3 bg-info/10 border border-info/20 rounded-lg">
          <div className="text-sm font-medium text-info">Agent Idle</div>
          <div className="text-xs text-info/80 mt-1">
            Waiting for suitable trading opportunities
          </div>
        </div>
      )}
    </div>
  )
}