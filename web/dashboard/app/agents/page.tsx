'use client'

import { useState } from 'react'
import { AgentCard } from '@/components/agents/AgentCard'
import { AgentPerformanceBar } from '@/components/charts/AgentPerformanceBar'
import { StatCard } from '@/components/ui/StatCard'
import { StatusDot } from '@/components/ui/StatusDot'
import { useAgents, useDecisions, useDecisionStats, useAgentPerformance } from '@/hooks/useAgents'
import { AgentFilters } from '@/lib/types'
import { formatCurrency, formatPercentage, cn } from '@/lib/utils'
import { 
  Bot,
  Activity,
  Brain,
  TrendingUp,
  Award,
  Users,
  Filter,
  Plus,
  MoreVertical
} from 'lucide-react'

export default function AgentsPage() {
  const [filters, setFilters] = useState<AgentFilters>({})
  const [selectedMetric, setSelectedMetric] = useState<'pnl' | 'winRate' | 'trades' | 'avgReturn'>('pnl')
  const [showActions, setShowActions] = useState(false)

  const { data: agentsData, isLoading: agentsLoading, refetch } = useAgents(filters)
  const { data: decisionsData, isLoading: decisionsLoading } = useDecisions({ limit: 10 })
  const { data: decisionStatsData, isLoading: statsLoading } = useDecisionStats()
  const { data: performanceData, isLoading: performanceLoading } = useAgentPerformance()

  const agents = agentsData?.data || []
  const decisions = decisionsData?.data || []
  const decisionStats = decisionStatsData?.data
  const agentPerformance = performanceData || []

  // Calculate agent statistics
  const agentStats = {
    total: agents.length,
    active: agents.filter(a => a.status === 'active').length,
    idle: agents.filter(a => a.status === 'idle').length,
    error: agents.filter(a => a.status === 'error').length,
    totalPnl: agents.reduce((sum, a) => sum + a.totalPnl, 0),
    avgWinRate: agents.length > 0 ? agents.reduce((sum, a) => sum + a.winRate, 0) / agents.length : 0,
    totalTrades: agents.reduce((sum, a) => sum + a.totalTrades, 0),
    bestAgent: agents.length > 0 ? agents.reduce((best, agent) => 
      agent.totalPnl > best.totalPnl ? agent : best
    ) : null,
  }

  const handleAgentAction = async (agentName: string, action: 'start' | 'stop' | 'pause' | 'configure') => {
    console.log(`${action} agent:`, agentName)
    // Here you would implement the actual agent control logic
    // For now, we'll just log the action
    
    try {
      // Simulate API call
      await new Promise(resolve => setTimeout(resolve, 1000))
      
      // Refresh agents data after action
      refetch()
    } catch (error) {
      console.error(`Failed to ${action} agent:`, error)
    }
  }

  const metricOptions = [
    { value: 'pnl', label: 'Total P&L' },
    { value: 'winRate', label: 'Win Rate' },
    { value: 'trades', label: 'Total Trades' },
    { value: 'avgReturn', label: 'Avg Return' },
  ] as const

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">AI Agent Scorecard</h1>
          <p className="text-muted-foreground">
            Monitor and manage your AI trading agents
          </p>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={() => setShowActions(!showActions)}
            className="flex items-center gap-2 px-4 py-2 bg-muted hover:bg-muted/80 rounded-lg transition-colors"
          >
            <Filter className="h-4 w-4" />
            Filters
          </button>

          <button className="flex items-center gap-2 px-4 py-2 bg-primary text-primary-foreground hover:bg-primary/90 rounded-lg transition-colors">
            <Plus className="h-4 w-4" />
            Deploy Agent
          </button>

          <div className="relative">
            <button className="p-2 hover:bg-muted/50 rounded-lg transition-colors">
              <MoreVertical className="h-4 w-4" />
            </button>
          </div>
        </div>
      </div>

      {/* Filter Panel */}
      {showActions && (
        <div className="bg-card border border-border rounded-lg p-4">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div>
              <label className="block text-sm font-medium text-muted-foreground mb-2">Status</label>
              <select
                value={filters.status || ''}
                onChange={(e) => setFilters({ ...filters, status: e.target.value as any || undefined })}
                className="w-full px-3 py-2 border border-border rounded-lg bg-background text-foreground"
              >
                <option value="">All Status</option>
                <option value="active">Active</option>
                <option value="idle">Idle</option>
                <option value="error">Error</option>
                <option value="disabled">Disabled</option>
              </select>
            </div>

            <div>
              <label className="block text-sm font-medium text-muted-foreground mb-2">Type</label>
              <select
                value={filters.type || ''}
                onChange={(e) => setFilters({ ...filters, type: e.target.value || undefined })}
                className="w-full px-3 py-2 border border-border rounded-lg bg-background text-foreground"
              >
                <option value="">All Types</option>
                <option value="momentum">Momentum</option>
                <option value="mean_reversion">Mean Reversion</option>
                <option value="scalper">Scalper</option>
                <option value="ml_prediction">ML Prediction</option>
              </select>
            </div>

            <div className="flex items-end">
              <button
                onClick={() => setFilters({})}
                className="px-4 py-2 border border-border rounded-lg hover:bg-muted/50 transition-colors"
              >
                Clear Filters
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Agent Statistics */}
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-5 gap-6">
        <StatCard
          title="Total Agents"
          value={agentStats.total}
          formatAs="number"
          icon={<Bot className="h-5 w-5" />}
          loading={agentsLoading}
        />

        <StatCard
          title="Active Agents"
          value={agentStats.active}
          formatAs="number"
          icon={<Activity className="h-5 w-5" />}
          trend={agentStats.active > agentStats.total / 2 ? 'up' : 'down'}
          loading={agentsLoading}
        />

        <StatCard
          title="Combined P&L"
          value={agentStats.totalPnl}
          formatAs="currency"
          icon={<TrendingUp className="h-5 w-5" />}
          trend={agentStats.totalPnl > 0 ? 'up' : agentStats.totalPnl < 0 ? 'down' : 'neutral'}
          loading={agentsLoading}
        />

        <StatCard
          title="Average Win Rate"
          value={agentStats.avgWinRate}
          formatAs="percentage"
          icon={<Award className="h-5 w-5" />}
          trend={agentStats.avgWinRate > 60 ? 'up' : agentStats.avgWinRate < 40 ? 'down' : 'neutral'}
          loading={agentsLoading}
        />

        <StatCard
          title="Total Decisions"
          value={decisionStats?.total || 0}
          formatAs="number"
          icon={<Brain className="h-5 w-5" />}
          loading={statsLoading}
        />
      </div>

      {/* Performance Chart */}
      <div className="bg-card border border-border rounded-lg p-6">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-xl font-semibold">Agent Performance Comparison</h2>
          
          <div className="flex items-center gap-2">
            <span className="text-sm text-muted-foreground">Metric:</span>
            <div className="flex bg-muted rounded-lg p-1">
              {metricOptions.map((option) => (
                <button
                  key={option.value}
                  onClick={() => setSelectedMetric(option.value)}
                  className={cn(
                    "px-3 py-1 text-sm font-medium rounded transition-colors",
                    selectedMetric === option.value
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

        <AgentPerformanceBar
          agents={agents}
          metric={selectedMetric}
          height={350}
          showLegend={false}
          loading={agentsLoading}
        />
      </div>

      {/* Agent Cards Grid */}
      <div>
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-xl font-semibold">Agent Overview</h2>
          
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-2 text-sm">
              <StatusDot status="active" size="sm" label={null} />
              <span>{agentStats.active} Active</span>
            </div>
            <div className="flex items-center gap-2 text-sm">
              <StatusDot status="idle" size="sm" label={null} />
              <span>{agentStats.idle} Idle</span>
            </div>
            <div className="flex items-center gap-2 text-sm">
              <StatusDot status="error" size="sm" label={null} />
              <span>{agentStats.error} Error</span>
            </div>
          </div>
        </div>

        {agentsLoading ? (
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
            {[1, 2, 3, 4, 5, 6].map((i) => (
              <div key={i} className="bg-card border border-border rounded-lg p-6">
                <div className="space-y-4">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <div className="skeleton w-10 h-10 rounded-lg" />
                      <div className="space-y-1">
                        <div className="skeleton h-4 w-24" />
                        <div className="skeleton h-3 w-16" />
                      </div>
                    </div>
                    <div className="skeleton w-3 h-3 rounded-full" />
                  </div>
                  <div className="skeleton h-8 w-full" />
                  <div className="grid grid-cols-2 gap-4">
                    <div className="skeleton h-12 rounded" />
                    <div className="skeleton h-12 rounded" />
                  </div>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-6">
            {agents.map((agent) => (
              <AgentCard
                key={agent.name}
                agent={agent}
                onAction={(action) => handleAgentAction(agent.name, action)}
              />
            ))}

            {agents.length === 0 && (
              <div className="col-span-full text-center py-12">
                <Bot className="h-12 w-12 text-muted-foreground mx-auto mb-4" />
                <h3 className="text-lg font-semibold mb-2">No agents found</h3>
                <p className="text-muted-foreground mb-4">
                  {Object.keys(filters).length > 0 
                    ? 'No agents match your current filters'
                    : 'Deploy your first AI trading agent to get started'}
                </p>
                <button className="px-6 py-2 bg-primary text-primary-foreground rounded-lg hover:bg-primary/90 transition-colors">
                  Deploy Agent
                </button>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Recent Decisions */}
      {decisions.length > 0 && (
        <div className="bg-card border border-border rounded-lg p-6">
          <h2 className="text-xl font-semibold mb-4">Recent Agent Decisions</h2>
          
          <div className="space-y-3">
            {decisions.slice(0, 5).map((decision) => (
              <div key={decision.id} className="flex items-start justify-between p-4 bg-muted/30 rounded-lg">
                <div className="flex-1">
                  <div className="flex items-center gap-3 mb-2">
                    <span className="font-medium">{decision.agent}</span>
                    <span className="text-sm px-2 py-1 bg-background rounded text-muted-foreground">
                      {decision.symbol}
                    </span>
                    <span className={cn(
                      "text-sm font-medium capitalize",
                      decision.action === 'buy' && "text-profit",
                      decision.action === 'sell' && "text-loss",
                      decision.action === 'hold' && "text-muted-foreground"
                    )}>
                      {decision.action}
                    </span>
                  </div>
                  
                  <p className="text-sm text-muted-foreground mb-2">
                    {decision.reasoning}
                  </p>
                  
                  <div className="flex items-center gap-4 text-xs text-muted-foreground">
                    <span>Confidence: {decision.confidence}%</span>
                    <span>Executed: {decision.executed ? 'Yes' : 'No'}</span>
                    {decision.pnl && (
                      <span className={cn(
                        decision.pnl > 0 ? "text-profit" : "text-loss"
                      )}>
                        P&L: {decision.pnl > 0 ? '+' : ''}{formatCurrency(decision.pnl)}
                      </span>
                    )}
                  </div>
                </div>
                
                <div className="text-xs text-muted-foreground">
                  {new Date(decision.timestamp).toLocaleString()}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}