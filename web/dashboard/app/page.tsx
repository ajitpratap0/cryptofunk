'use client'

import { StatCard } from '@/components/ui/StatCard'
import { EquityCurve } from '@/components/charts/EquityCurve'
import { AgentPerformanceBar } from '@/components/charts/AgentPerformanceBar'
import { TradesTable } from '@/components/trades/TradesTable'
import { SystemStatusIndicator } from '@/components/ui/StatusDot'
import { useDashboard, useDashboardPnl, useTrades } from '@/hooks/useTradeData'
import { useAgents } from '@/hooks/useAgents'
import { formatCurrency, formatPercentage } from '@/lib/utils'
import { 
  DollarSign,
  TrendingUp,
  Activity,
  Target,
  Users,
  Zap
} from 'lucide-react'

export default function DashboardPage() {
  const { data: dashboardData, isLoading: dashboardLoading } = useDashboard()
  const { data: pnlData, isLoading: pnlLoading } = useDashboardPnl()
  const { data: tradesData, isLoading: tradesLoading } = useTrades()
  const { data: agentsData, isLoading: agentsLoading } = useAgents()

  const stats = dashboardData?.data
  const pnl = pnlData?.data
  const trades = tradesData?.data || []
  const agents = agentsData?.data || []

  // Calculate sparkline data for PnL
  const pnlSparkline = pnl?.equity 
    ? pnl.equity.slice(-10).map(point => point.pnl) 
    : undefined

  // Get recent trades (last 10)
  const recentTrades = trades.slice(0, 10)

  // Mock system status
  const systemStatus = {
    status: 'healthy' as const,
    services: {
      'Trading Engine': 'up' as const,
      'Data Feed': 'up' as const,
      'Risk Manager': 'up' as const,
      'ML Models': 'up' as const,
    }
  }

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Portfolio Overview</h1>
          <p className="text-muted-foreground">
            Real-time performance and trading activity
          </p>
        </div>
        
        <SystemStatusIndicator 
          status={systemStatus.status}
          services={systemStatus.services}
        />
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-6">
        <StatCard
          title="Total P&L"
          value={stats?.totalPnl || 0}
          change={pnl?.daily}
          changeType="currency"
          formatAs="currency"
          icon={<DollarSign className="h-5 w-5" />}
          sparklineData={pnlSparkline}
          loading={dashboardLoading}
        />

        <StatCard
          title="Win Rate"
          value={stats?.winRate || 0}
          formatAs="percentage"
          icon={<Target className="h-5 w-5" />}
          trend={stats?.winRate && stats.winRate > 60 ? 'up' : stats?.winRate && stats.winRate < 40 ? 'down' : 'neutral'}
          loading={dashboardLoading}
        />

        <StatCard
          title="Active Positions"
          value={stats?.activePositions || 0}
          formatAs="number"
          icon={<Activity className="h-5 w-5" />}
          loading={dashboardLoading}
        />

        <StatCard
          title="Total Trades"
          value={stats?.totalTrades || 0}
          formatAs="number"
          icon={<TrendingUp className="h-5 w-5" />}
          loading={dashboardLoading}
        />
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
        {/* Equity Curve */}
        <div className="bg-card border border-border rounded-lg p-6">
          <h2 className="text-xl font-semibold mb-4">Equity Curve</h2>
          <EquityCurve
            data={pnl?.equity || []}
            height={300}
            loading={pnlLoading}
          />
        </div>

        {/* Agent Performance */}
        <div className="bg-card border border-border rounded-lg p-6">
          <h2 className="text-xl font-semibold mb-4">Agent Performance</h2>
          <AgentPerformanceBar
            agents={agents}
            metric="pnl"
            height={300}
            showLegend={false}
            loading={agentsLoading}
          />
        </div>
      </div>

      {/* Portfolio Metrics */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div className="bg-card border border-border rounded-lg p-6">
          <div className="flex items-center gap-3 mb-4">
            <div className="p-2 bg-profit/20 rounded-lg">
              <DollarSign className="h-5 w-5 text-profit" />
            </div>
            <div>
              <h3 className="font-semibold">Account Balance</h3>
              <p className="text-sm text-muted-foreground">Available funds</p>
            </div>
          </div>
          
          <div className="space-y-3">
            <div className="flex justify-between">
              <span className="text-muted-foreground">Total Equity</span>
              <span className="font-mono font-medium">
                {stats ? formatCurrency(stats.equity) : '--'}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Available Balance</span>
              <span className="font-mono font-medium">
                {stats ? formatCurrency(stats.availableBalance) : '--'}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Margin Used</span>
              <span className="font-mono font-medium">
                {stats ? formatCurrency(stats.marginUsed) : '--'}
              </span>
            </div>
          </div>
        </div>

        <div className="bg-card border border-border rounded-lg p-6">
          <div className="flex items-center gap-3 mb-4">
            <div className="p-2 bg-info/20 rounded-lg">
              <Users className="h-5 w-5 text-info" />
            </div>
            <div>
              <h3 className="font-semibold">Active Agents</h3>
              <p className="text-sm text-muted-foreground">Currently trading</p>
            </div>
          </div>
          
          <div className="space-y-3">
            {agents
              .filter(agent => agent.status === 'active')
              .slice(0, 3)
              .map(agent => (
                <div key={agent.name} className="flex justify-between">
                  <span className="text-sm">{agent.name}</span>
                  <span className={`text-sm font-medium ${
                    agent.totalPnl > 0 ? 'text-profit' : 'text-loss'
                  }`}>
                    {agent.totalPnl > 0 ? '+' : ''}{formatCurrency(agent.totalPnl)}
                  </span>
                </div>
              ))}
            
            {agents.filter(agent => agent.status === 'active').length === 0 && (
              <div className="text-sm text-muted-foreground text-center py-4">
                No active agents
              </div>
            )}
          </div>
        </div>

        <div className="bg-card border border-border rounded-lg p-6">
          <div className="flex items-center gap-3 mb-4">
            <div className="p-2 bg-warning/20 rounded-lg">
              <Zap className="h-5 w-5 text-warning" />
            </div>
            <div>
              <h3 className="font-semibold">Today's Activity</h3>
              <p className="text-sm text-muted-foreground">Trading summary</p>
            </div>
          </div>
          
          <div className="space-y-3">
            <div className="flex justify-between">
              <span className="text-muted-foreground">Trades Today</span>
              <span className="font-mono font-medium">
                {trades.filter(trade => {
                  const today = new Date().toDateString()
                  const tradeDate = new Date(trade.timestamp).toDateString()
                  return today === tradeDate
                }).length}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Daily P&L</span>
              <span className={`font-mono font-medium ${
                pnl?.daily && pnl.daily > 0 ? 'text-profit' : 'text-loss'
              }`}>
                {pnl?.daily ? formatCurrency(pnl.daily) : '--'}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Return Today</span>
              <span className={`font-mono font-medium ${
                pnl?.daily && pnl.daily > 0 ? 'text-profit' : 'text-loss'
              }`}>
                {pnl?.daily && stats?.equity ? 
                  formatPercentage((pnl.daily / stats.equity) * 100) : '--'}
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Recent Trades */}
      <div>
        <h2 className="text-xl font-semibold mb-4">Recent Trades</h2>
        <TradesTable
          trades={recentTrades}
          loading={tradesLoading}
          maxHeight="400px"
          showFilters={false}
        />
      </div>
    </div>
  )
}