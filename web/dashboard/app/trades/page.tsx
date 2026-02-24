'use client'

import { useState } from 'react'
import { TradesTable } from '@/components/trades/TradesTable'
import { StatCard } from '@/components/ui/StatCard'
import { ConnectionStatus } from '@/components/ui/StatusDot'
import { useTrades } from '@/hooks/useTradeData'
import { useTradingWebSocket } from '@/hooks/useWebSocket'
import { TradeFilters } from '@/lib/types'
import { formatCurrency, formatPercentage } from '@/lib/utils'
import { 
  Activity,
  TrendingUp,
  TrendingDown,
  Clock,
  RefreshCw
} from 'lucide-react'

export default function TradesPage() {
  const [filters, setFilters] = useState<TradeFilters>({})
  const { data: tradesData, isLoading, refetch } = useTrades()
  const { isConnected, lastMessage, trades: liveTradesState } = useTradingWebSocket()

  const trades = tradesData?.data || []

  // Calculate trade statistics
  const tradeStats = {
    total: trades.length,
    open: trades.filter(t => t.status === 'open').length,
    closed: trades.filter(t => t.status === 'closed').length,
    pending: trades.filter(t => t.status === 'pending').length,
    totalPnl: trades.reduce((sum, t) => sum + t.pnl, 0),
    winningTrades: trades.filter(t => t.pnl > 0).length,
    losingTrades: trades.filter(t => t.pnl < 0).length,
    winRate: trades.length > 0 ? (trades.filter(t => t.pnl > 0).length / trades.length) * 100 : 0,
    avgPnl: trades.length > 0 ? trades.reduce((sum, t) => sum + t.pnl, 0) / trades.length : 0,
  }

  const handleRefresh = () => {
    refetch()
  }

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Live Trades</h1>
          <p className="text-muted-foreground">
            Real-time trading activity and position monitoring
          </p>
        </div>

        <div className="flex items-center gap-4">
          <ConnectionStatus
            isConnected={isConnected}
            lastUpdate={lastMessage?.timestamp}
          />
          
          <button
            onClick={handleRefresh}
            className="flex items-center gap-2 px-4 py-2 bg-muted hover:bg-muted/80 rounded-lg transition-colors"
          >
            <RefreshCw className="h-4 w-4" />
            Refresh
          </button>
        </div>
      </div>

      {/* Live Updates Banner */}
      {isConnected && liveTradesState.length > 0 && (
        <div className="bg-gradient-to-r from-profit/20 to-info/20 border border-profit/30 rounded-lg p-4">
          <div className="flex items-center gap-3">
            <div className="w-2 h-2 bg-profit rounded-full animate-pulse" />
            <div>
              <div className="font-medium">Live Trading Active</div>
              <div className="text-sm text-muted-foreground">
                Receiving real-time trade updates • Last update: {
                  lastMessage?.timestamp ? new Date(lastMessage.timestamp).toLocaleTimeString() : 'N/A'
                }
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Summary Statistics */}
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-5 gap-6">
        <StatCard
          title="Total Trades"
          value={tradeStats.total}
          formatAs="number"
          icon={<Activity className="h-5 w-5" />}
          loading={isLoading}
        />

        <StatCard
          title="Open Positions"
          value={tradeStats.open}
          formatAs="number"
          icon={<Clock className="h-5 w-5" />}
          trend={tradeStats.open > 0 ? 'up' : 'neutral'}
          loading={isLoading}
        />

        <StatCard
          title="Total P&L"
          value={tradeStats.totalPnl}
          formatAs="currency"
          icon={<TrendingUp className="h-5 w-5" />}
          trend={tradeStats.totalPnl > 0 ? 'up' : tradeStats.totalPnl < 0 ? 'down' : 'neutral'}
          loading={isLoading}
        />

        <StatCard
          title="Win Rate"
          value={tradeStats.winRate}
          formatAs="percentage"
          icon={<TrendingUp className="h-5 w-5" />}
          trend={tradeStats.winRate > 60 ? 'up' : tradeStats.winRate < 40 ? 'down' : 'neutral'}
          loading={isLoading}
        />

        <StatCard
          title="Average P&L"
          value={tradeStats.avgPnl}
          formatAs="currency"
          icon={<TrendingUp className="h-5 w-5" />}
          trend={tradeStats.avgPnl > 0 ? 'up' : tradeStats.avgPnl < 0 ? 'down' : 'neutral'}
          loading={isLoading}
        />
      </div>

      {/* Performance Breakdown */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <div className="bg-card border border-border rounded-lg p-6">
          <div className="flex items-center gap-3 mb-4">
            <div className="p-2 bg-profit/20 rounded-lg">
              <TrendingUp className="h-5 w-5 text-profit" />
            </div>
            <div>
              <h3 className="font-semibold">Winning Trades</h3>
              <p className="text-sm text-muted-foreground">Profitable positions</p>
            </div>
          </div>
          
          <div className="space-y-2">
            <div className="text-2xl font-bold text-profit">
              {tradeStats.winningTrades}
            </div>
            <div className="text-sm text-muted-foreground">
              {formatPercentage((tradeStats.winningTrades / Math.max(tradeStats.total, 1)) * 100)} of total
            </div>
            <div className="text-sm font-medium">
              Avg Win: {formatCurrency(
                trades.filter(t => t.pnl > 0).length > 0 
                  ? trades.filter(t => t.pnl > 0).reduce((sum, t) => sum + t.pnl, 0) / trades.filter(t => t.pnl > 0).length
                  : 0
              )}
            </div>
          </div>
        </div>

        <div className="bg-card border border-border rounded-lg p-6">
          <div className="flex items-center gap-3 mb-4">
            <div className="p-2 bg-loss/20 rounded-lg">
              <TrendingDown className="h-5 w-5 text-loss" />
            </div>
            <div>
              <h3 className="font-semibold">Losing Trades</h3>
              <p className="text-sm text-muted-foreground">Loss-making positions</p>
            </div>
          </div>
          
          <div className="space-y-2">
            <div className="text-2xl font-bold text-loss">
              {tradeStats.losingTrades}
            </div>
            <div className="text-sm text-muted-foreground">
              {formatPercentage((tradeStats.losingTrades / Math.max(tradeStats.total, 1)) * 100)} of total
            </div>
            <div className="text-sm font-medium">
              Avg Loss: {formatCurrency(
                trades.filter(t => t.pnl < 0).length > 0 
                  ? trades.filter(t => t.pnl < 0).reduce((sum, t) => sum + t.pnl, 0) / trades.filter(t => t.pnl < 0).length
                  : 0
              )}
            </div>
          </div>
        </div>

        <div className="bg-card border border-border rounded-lg p-6">
          <div className="flex items-center gap-3 mb-4">
            <div className="p-2 bg-info/20 rounded-lg">
              <Activity className="h-5 w-5 text-info" />
            </div>
            <div>
              <h3 className="font-semibold">Trading Activity</h3>
              <p className="text-sm text-muted-foreground">Recent performance</p>
            </div>
          </div>
          
          <div className="space-y-2">
            <div className="flex justify-between">
              <span className="text-sm text-muted-foreground">Last Hour</span>
              <span className="text-sm font-medium">
                {trades.filter(t => {
                  const hourAgo = Date.now() - (60 * 60 * 1000)
                  return new Date(t.timestamp).getTime() > hourAgo
                }).length} trades
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-sm text-muted-foreground">Last 24h</span>
              <span className="text-sm font-medium">
                {trades.filter(t => {
                  const dayAgo = Date.now() - (24 * 60 * 60 * 1000)
                  return new Date(t.timestamp).getTime() > dayAgo
                }).length} trades
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-sm text-muted-foreground">Most Active Pair</span>
              <span className="text-sm font-medium">
                {trades.length > 0 
                  ? Object.entries(
                      trades.reduce((acc, trade) => {
                        acc[trade.symbol] = (acc[trade.symbol] || 0) + 1
                        return acc
                      }, {} as Record<string, number>)
                    ).sort(([,a], [,b]) => b - a)[0]?.[0] || 'N/A'
                  : 'N/A'
                }
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Trades Table */}
      <div>
        <TradesTable
          trades={trades}
          filters={filters}
          onFiltersChange={setFilters}
          loading={isLoading}
          showFilters={true}
        />
      </div>
    </div>
  )
}