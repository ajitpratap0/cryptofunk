'use client'

import { useState } from 'react'
import { StatCard } from '@/components/ui/StatCard'
import {
  usePolymarketPortfolio,
  usePolymarketPositions,
  usePolymarketTrades,
  usePolymarketMarkets,
  usePolymarketPredictions,
  usePolymarketPerformance,
  usePaperTrade,
} from '@/hooks/usePolymarket'
import { formatCurrency, formatPercentage } from '@/lib/utils'
import {
  DollarSign,
  TrendingUp,
  TrendingDown,
  BarChart3,
  Target,
  RefreshCw,
  ShoppingCart,
} from 'lucide-react'
import type { PaperTradeRequest } from '@/lib/types'

export default function PolymarketPage() {
  const { data: portfolio, isLoading: portfolioLoading } = usePolymarketPortfolio()
  const { data: positions, isLoading: positionsLoading } = usePolymarketPositions()
  const { data: trades, isLoading: tradesLoading } = usePolymarketTrades()
  const { data: markets, isLoading: marketsLoading } = usePolymarketMarkets()
  const { data: predictions } = usePolymarketPredictions()
  const { data: performance } = usePolymarketPerformance()
  const paperTrade = usePaperTrade()

  // Trade form state
  const [tradeForm, setTradeForm] = useState({
    market_id: '',
    side: 'YES' as 'YES' | 'NO',
    amount: '',
    price: '',
  })
  const [tradeMessage, setTradeMessage] = useState('')

  const handleTrade = async (action: 'BUY' | 'SELL') => {
    if (!tradeForm.market_id || !tradeForm.amount || !tradeForm.price) {
      setTradeMessage('Please fill all fields')
      return
    }

    const market = markets?.find(m => m.id === tradeForm.market_id)
    const req: PaperTradeRequest = {
      action,
      market_id: tradeForm.market_id,
      question: market?.question,
      side: tradeForm.side,
      amount: parseFloat(tradeForm.amount),
      price: parseFloat(tradeForm.price),
    }

    try {
      const result = await paperTrade.mutateAsync(req)
      if (result.success) {
        setTradeMessage(`${action} executed! Balance: $${result.data.balance.toFixed(2)}`)
        setTradeForm(f => ({ ...f, amount: '', price: '' }))
      } else {
        setTradeMessage(result.error || 'Trade failed')
      }
    } catch (err: any) {
      setTradeMessage(err?.message || 'Trade failed')
    }
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold">Polymarket Paper Trading</h1>
        <p className="text-muted-foreground">
          Prediction market positions and performance tracking
        </p>
      </div>

      {/* Portfolio Summary */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          title="Balance"
          value={portfolio ? formatCurrency(portfolio.balance) : '—'}
          icon={<DollarSign className="h-5 w-5" />}
          loading={portfolioLoading}
        />
        <StatCard
          title="Total Value"
          value={portfolio ? formatCurrency(portfolio.total_value) : '—'}
          icon={<TrendingUp className="h-5 w-5" />}
          loading={portfolioLoading}
        />
        <StatCard
          title="Unrealized P&L"
          value={portfolio ? formatCurrency(portfolio.unrealized_pnl) : '—'}
          icon={portfolio && portfolio.unrealized_pnl >= 0 ? <TrendingUp className="h-5 w-5" /> : <TrendingDown className="h-5 w-5" />}
          trend={portfolio ? (portfolio.unrealized_pnl >= 0 ? 'up' : 'down') : undefined}
          loading={portfolioLoading}
        />
        <StatCard
          title="Open Positions"
          value={portfolio ? String(portfolio.position_count) : '—'}
          icon={<BarChart3 className="h-5 w-5" />}
          loading={portfolioLoading}
        />
      </div>

      {/* Performance Stats */}
      {performance && performance.total_predictions > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <StatCard
            title="Win Rate"
            value={formatPercentage(performance.win_rate)}
            icon={<Target className="h-5 w-5" />}
          />
          <StatCard
            title="Brier Score"
            value={performance.brier_score.toFixed(4)}
            subtitle="Lower is better (0 = perfect)"
            icon={<BarChart3 className="h-5 w-5" />}
          />
          <StatCard
            title="Total P&L"
            value={formatCurrency(performance.total_pnl)}
            trend={performance.total_pnl >= 0 ? 'up' : 'down'}
            icon={<DollarSign className="h-5 w-5" />}
          />
        </div>
      )}

      {/* Trade Controls */}
      <div className="bg-card border border-border rounded-lg p-6">
        <h2 className="text-lg font-semibold mb-4 flex items-center gap-2">
          <ShoppingCart className="h-5 w-5" />
          Paper Trade
        </h2>
        <div className="grid grid-cols-1 md:grid-cols-5 gap-4 items-end">
          <div>
            <label className="block text-sm text-muted-foreground mb-1">Market</label>
            <select
              className="w-full bg-background border border-border rounded-lg px-3 py-2 text-sm"
              value={tradeForm.market_id}
              onChange={e => {
                const m = markets?.find(mk => mk.id === e.target.value)
                setTradeForm(f => ({
                  ...f,
                  market_id: e.target.value,
                  price: m ? (f.side === 'YES' ? m.yes_price.toFixed(2) : m.no_price.toFixed(2)) : f.price,
                }))
              }}
            >
              <option value="">Select market...</option>
              {markets?.map(m => (
                <option key={m.id} value={m.id}>
                  {m.question?.substring(0, 60)}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-sm text-muted-foreground mb-1">Side</label>
            <div className="flex gap-2">
              {(['YES', 'NO'] as const).map(s => (
                <button
                  key={s}
                  className={`flex-1 px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
                    tradeForm.side === s
                      ? s === 'YES' ? 'bg-green-600 text-white' : 'bg-red-600 text-white'
                      : 'bg-muted text-muted-foreground hover:bg-muted/80'
                  }`}
                  onClick={() => {
                    const m = markets?.find(mk => mk.id === tradeForm.market_id)
                    setTradeForm(f => ({
                      ...f,
                      side: s,
                      price: m ? (s === 'YES' ? m.yes_price.toFixed(2) : m.no_price.toFixed(2)) : f.price,
                    }))
                  }}
                >
                  {s}
                </button>
              ))}
            </div>
          </div>
          <div>
            <label className="block text-sm text-muted-foreground mb-1">Amount ($)</label>
            <input
              type="number"
              step="0.01"
              className="w-full bg-background border border-border rounded-lg px-3 py-2 text-sm"
              placeholder="10.00"
              value={tradeForm.amount}
              onChange={e => setTradeForm(f => ({ ...f, amount: e.target.value }))}
            />
          </div>
          <div>
            <label className="block text-sm text-muted-foreground mb-1">Price</label>
            <input
              type="number"
              step="0.01"
              min="0.01"
              max="0.99"
              className="w-full bg-background border border-border rounded-lg px-3 py-2 text-sm"
              placeholder="0.50"
              value={tradeForm.price}
              onChange={e => setTradeForm(f => ({ ...f, price: e.target.value }))}
            />
          </div>
          <div className="flex gap-2">
            <button
              onClick={() => handleTrade('BUY')}
              disabled={paperTrade.isPending}
              className="flex-1 bg-green-600 hover:bg-green-700 text-white px-4 py-2 rounded-lg text-sm font-medium disabled:opacity-50 transition-colors"
            >
              Buy
            </button>
            <button
              onClick={() => handleTrade('SELL')}
              disabled={paperTrade.isPending}
              className="flex-1 bg-red-600 hover:bg-red-700 text-white px-4 py-2 rounded-lg text-sm font-medium disabled:opacity-50 transition-colors"
            >
              Sell
            </button>
          </div>
        </div>
        {tradeMessage && (
          <p className="mt-3 text-sm text-muted-foreground">{tradeMessage}</p>
        )}
      </div>

      {/* Open Positions */}
      <div className="bg-card border border-border rounded-lg p-6">
        <h2 className="text-lg font-semibold mb-4">Open Positions</h2>
        {positionsLoading ? (
          <div className="animate-pulse space-y-3">
            {[1, 2, 3].map(i => (
              <div key={i} className="h-12 bg-muted rounded" />
            ))}
          </div>
        ) : positions && positions.length > 0 ? (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-muted-foreground">
                  <th className="text-left py-2 px-3">Market</th>
                  <th className="text-left py-2 px-3">Side</th>
                  <th className="text-right py-2 px-3">Shares</th>
                  <th className="text-right py-2 px-3">Avg Price</th>
                  <th className="text-right py-2 px-3">Current</th>
                  <th className="text-right py-2 px-3">P&L</th>
                </tr>
              </thead>
              <tbody>
                {positions.map(p => (
                  <tr key={p.id} className="border-b border-border/50 hover:bg-muted/30">
                    <td className="py-2 px-3 max-w-xs truncate">{p.market_question || p.market_id}</td>
                    <td className="py-2 px-3">
                      <span className={`px-2 py-0.5 rounded text-xs font-medium ${
                        p.side === 'YES' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
                      }`}>
                        {p.side}
                      </span>
                    </td>
                    <td className="text-right py-2 px-3">{p.shares.toFixed(2)}</td>
                    <td className="text-right py-2 px-3">${p.avg_price.toFixed(2)}</td>
                    <td className="text-right py-2 px-3">
                      {p.current_price != null ? `$${p.current_price.toFixed(2)}` : '—'}
                    </td>
                    <td className={`text-right py-2 px-3 font-medium ${
                      (p.unrealized_pnl || 0) >= 0 ? 'text-green-400' : 'text-red-400'
                    }`}>
                      {p.unrealized_pnl != null ? formatCurrency(p.unrealized_pnl) : '—'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <p className="text-muted-foreground text-sm">No open positions</p>
        )}
      </div>

      {/* Trade History */}
      <div className="bg-card border border-border rounded-lg p-6">
        <h2 className="text-lg font-semibold mb-4">Trade History</h2>
        {tradesLoading ? (
          <div className="animate-pulse space-y-3">
            {[1, 2, 3].map(i => (
              <div key={i} className="h-10 bg-muted rounded" />
            ))}
          </div>
        ) : trades && trades.length > 0 ? (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-muted-foreground">
                  <th className="text-left py-2 px-3">Time</th>
                  <th className="text-left py-2 px-3">Action</th>
                  <th className="text-left py-2 px-3">Side</th>
                  <th className="text-right py-2 px-3">Amount</th>
                  <th className="text-right py-2 px-3">Price</th>
                  <th className="text-right py-2 px-3">Shares</th>
                </tr>
              </thead>
              <tbody>
                {trades.slice(0, 20).map(t => (
                  <tr key={t.id} className="border-b border-border/50 hover:bg-muted/30">
                    <td className="py-2 px-3 text-muted-foreground">
                      {new Date(t.timestamp).toLocaleString()}
                    </td>
                    <td className="py-2 px-3">
                      <span className={`px-2 py-0.5 rounded text-xs font-medium ${
                        t.action === 'BUY' ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
                      }`}>
                        {t.action}
                      </span>
                    </td>
                    <td className="py-2 px-3">{t.side}</td>
                    <td className="text-right py-2 px-3">{formatCurrency(t.amount)}</td>
                    <td className="text-right py-2 px-3">${t.price.toFixed(2)}</td>
                    <td className="text-right py-2 px-3">{t.shares.toFixed(2)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <p className="text-muted-foreground text-sm">No trades yet</p>
        )}
      </div>

      {/* Predictions */}
      {predictions && predictions.length > 0 && (
        <div className="bg-card border border-border rounded-lg p-6">
          <h2 className="text-lg font-semibold mb-4">Prediction History</h2>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-muted-foreground">
                  <th className="text-left py-2 px-3">Date</th>
                  <th className="text-left py-2 px-3">Market</th>
                  <th className="text-right py-2 px-3">Predicted</th>
                  <th className="text-right py-2 px-3">Market Price</th>
                  <th className="text-right py-2 px-3">Edge</th>
                  <th className="text-right py-2 px-3">Outcome</th>
                </tr>
              </thead>
              <tbody>
                {predictions.slice(0, 20).map(p => (
                  <tr key={p.id} className="border-b border-border/50 hover:bg-muted/30">
                    <td className="py-2 px-3 text-muted-foreground">
                      {new Date(p.created_at).toLocaleDateString()}
                    </td>
                    <td className="py-2 px-3 max-w-xs truncate">{p.market_id}</td>
                    <td className="text-right py-2 px-3">{formatPercentage(p.predicted_prob * 100)}</td>
                    <td className="text-right py-2 px-3">{formatPercentage(p.market_price * 100)}</td>
                    <td className={`text-right py-2 px-3 ${p.edge >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                      {formatPercentage(p.edge * 100)}
                    </td>
                    <td className="text-right py-2 px-3">
                      {p.resolved ? (
                        <span className={p.pnl != null && p.pnl >= 0 ? 'text-green-400' : 'text-red-400'}>
                          {p.outcome != null ? formatPercentage(p.outcome * 100) : '—'}
                        </span>
                      ) : (
                        <span className="text-muted-foreground">Pending</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}
