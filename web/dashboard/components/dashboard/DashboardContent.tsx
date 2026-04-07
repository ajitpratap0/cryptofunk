'use client'

import { useMemo } from 'react'
import { StatCard } from '@/components/ui/StatCard'
import { CHART_HEIGHT } from '@/lib/constants'
import { EquityCurve } from '@/components/charts/EquityCurve'
import { AgentPerformanceBar } from '@/components/charts/AgentPerformanceBar'
import { TradesTable } from '@/components/trades/TradesTable'
import { SystemStatusIndicator } from '@/components/ui/StatusDot'
import { useDashboard, useDashboardPnl, useTrades, useUnifiedPortfolio, useSystemStatus } from '@/hooks/useTradeData'
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

export default function DashboardContent() {
  const dashboardQuery = useDashboard()
  const pnlQuery = useDashboardPnl()
  const tradesQuery = useTrades()
  const agentsQuery = useAgents()
  const { data: unifiedData } = useUnifiedPortfolio()
  const statusQuery = useSystemStatus()

  const { data: dashboardData, isLoading: dashboardLoading } = dashboardQuery
  const { data: pnlData, isLoading: pnlLoading } = pnlQuery
  const { data: tradesData, isLoading: tradesLoading } = tradesQuery
  const { data: agentsData, isLoading: agentsLoading } = agentsQuery
  const { data: statusData } = statusQuery

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

  // Default to 'unknown' (not 'healthy') so a status fetch failure is visible
  // instead of silently masking real outages on a trading dashboard.
  const systemStatus: {
    status: 'healthy' | 'degraded' | 'down' | 'unknown'
    services: Record<string, 'up' | 'down' | 'degraded'>
  } = statusData?.data ?? {
    status: 'unknown',
    services: {},
  }

  // Surface backend failures so '$0 P&L / 0 trades / no agents' is never
  // confused with 'backend is down' or 'auth failed'. The data hooks throw
  // on failure, so react-query exposes the error via `isError`/`error`.
  const failedQueries = useMemo(() => {
    const queries: Array<[string, { isError: boolean; error: unknown }]> = [
      ['dashboard', dashboardQuery],
      ['pnl', pnlQuery],
      ['trades', tradesQuery],
      ['agents', agentsQuery],
      ['status', statusQuery],
    ]
    return queries
      .filter(([, q]) => q.isError)
      .map(([name, q]) => ({
        name,
        message: q.error instanceof Error ? q.error.message : 'unknown error',
      }))
    // Depend on the underlying primitives, not the query objects themselves —
    // react-query returns a fresh result object on every render, which would
    // bust this memo every time and defeat the point.
  }, [
    dashboardQuery.isError,
    dashboardQuery.error,
    pnlQuery.isError,
    pnlQuery.error,
    tradesQuery.isError,
    tradesQuery.error,
    agentsQuery.isError,
    agentsQuery.error,
    statusQuery.isError,
    statusQuery.error,
  ])

  return (
    <>
      <div className="flex justify-end mb-6">
        <SystemStatusIndicator
          status={systemStatus.status}
          services={systemStatus.services}
        />
      </div>

      {failedQueries.length > 0 && (
        <div
          role="alert"
          className="rounded-lg border border-loss/40 bg-loss/10 p-4 text-sm text-loss"
        >
          <div className="font-semibold mb-1">Some dashboard data failed to load</div>
          <ul className="list-disc list-inside space-y-0.5">
            {failedQueries.map((q) => (
              <li key={q.name}>{q.name}: {q.message}</li>
            ))}
          </ul>
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-6">
        <StatCard
          title="Total P&L"
          value={stats?.totalPnl ?? 0}
          change={pnl?.daily}
          changeType="currency"
          formatAs="currency"
          icon={<DollarSign className="h-5 w-5" />}
          sparklineData={pnlSparkline}
          hasSparkline
          loading={dashboardLoading}
        />

        <StatCard
          title="Win Rate"
          value={stats?.winRate ?? 0}
          formatAs="percentage"
          icon={<Target className="h-5 w-5" />}
          trend={stats?.winRate && stats.winRate > 60 ? 'up' : stats?.winRate && stats.winRate < 40 ? 'down' : 'neutral'}
          loading={dashboardLoading}
        />

        <StatCard
          title="Active Positions"
          value={stats?.activePositions ?? 0}
          formatAs="number"
          icon={<Activity className="h-5 w-5" />}
          loading={dashboardLoading}
        />

        <StatCard
          title="Total Trades"
          value={stats?.totalTrades ?? 0}
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
            height={CHART_HEIGHT}
            loading={pnlLoading}
          />
        </div>

        {/* Agent Performance */}
        <div className="bg-card border border-border rounded-lg p-6">
          <h2 className="text-xl font-semibold mb-4">Agent Performance</h2>
          <AgentPerformanceBar
            agents={agents}
            metric="pnl"
            height={CHART_HEIGHT}
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
                pnl?.daily && pnl.daily > 0 ? 'text-profit' : pnl?.daily && pnl.daily < 0 ? 'text-loss' : 'text-muted-foreground'
              }`}>
                {pnl?.daily ? formatCurrency(pnl.daily) : '--'}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-muted-foreground">Return Today</span>
              <span className={`font-mono font-medium ${
                pnl?.daily && pnl.daily > 0 ? 'text-profit' : pnl?.daily && pnl.daily < 0 ? 'text-loss' : 'text-muted-foreground'
              }`}>
                {pnl?.daily && stats?.equity ?
                  formatPercentage((pnl.daily / stats.equity) * 100) : '--'}
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Platform Breakdown */}
      {unifiedData?.data && (
        <div className="bg-card border border-border rounded-lg p-6">
          <h2 className="text-xl font-semibold mb-4">Platform Breakdown</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            {Object.entries(unifiedData.data.by_platform).map(([platform, summary]) => (
              <div key={platform} className="border border-border rounded-lg p-4">
                <h3 className="font-semibold capitalize mb-3">{platform}</h3>
                <div className="space-y-2 text-sm">
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Positions</span>
                    <span className="font-mono">{summary.position_count}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Trades</span>
                    <span className="font-mono">{summary.trade_count}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">Value</span>
                    <span className="font-mono">{formatCurrency(summary.total_value)}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-muted-foreground">P&L</span>
                    <span className={`font-mono ${summary.pnl >= 0 ? 'text-profit' : 'text-loss'}`}>
                      {summary.pnl >= 0 ? '+' : ''}{formatCurrency(summary.pnl)}
                    </span>
                  </div>
                </div>
              </div>
            ))}
          </div>

          {/* Unified positions list */}
          {unifiedData.data.positions.length > 0 && (
            <div className="mt-6">
              <h3 className="font-semibold mb-3">All Open Positions</h3>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border text-muted-foreground">
                      <th className="text-left py-2">Platform</th>
                      <th className="text-left py-2">Symbol</th>
                      <th className="text-left py-2">Side</th>
                      <th className="text-right py-2">Qty</th>
                      <th className="text-right py-2">Entry</th>
                      <th className="text-right py-2">Current</th>
                      <th className="text-right py-2">P&L</th>
                    </tr>
                  </thead>
                  <tbody>
                    {unifiedData.data.positions.map((pos) => (
                      <tr key={pos.id} className="border-b border-border/50">
                        <td className="py-2 capitalize">{pos.platform}</td>
                        <td className="py-2 max-w-[200px] truncate" title={pos.symbol}>{pos.symbol}</td>
                        <td className="py-2">{pos.side}</td>
                        <td className="py-2 text-right font-mono">{pos.quantity.toFixed(4)}</td>
                        <td className="py-2 text-right font-mono">{formatCurrency(pos.entry_price)}</td>
                        <td className="py-2 text-right font-mono">{formatCurrency(pos.current_price)}</td>
                        <td className={`py-2 text-right font-mono ${pos.unrealized_pnl >= 0 ? 'text-profit' : 'text-loss'}`}>
                          {pos.unrealized_pnl >= 0 ? '+' : ''}{formatCurrency(pos.unrealized_pnl)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </div>
      )}

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
    </>
  )
}
