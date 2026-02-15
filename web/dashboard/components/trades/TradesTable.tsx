'use client'

import { formatCurrency } from '@/lib/utils'
import type { Trade } from '@/lib/types'

interface TradesTableProps {
  trades: Trade[]
  loading?: boolean
  maxHeight?: string
  showFilters?: boolean
}

export function TradesTable({ trades, loading, maxHeight, showFilters }: TradesTableProps) {
  if (loading) {
    return (
      <div className="bg-card border border-border rounded-lg p-6">
        <div className="animate-pulse space-y-3">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="h-8 bg-muted rounded" />
          ))}
        </div>
      </div>
    )
  }

  if (trades.length === 0) {
    return (
      <div className="bg-card border border-border rounded-lg p-6 text-center text-muted-foreground">
        No trades to display
      </div>
    )
  }

  return (
    <div className="bg-card border border-border rounded-lg overflow-hidden" style={{ maxHeight }}>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border text-muted-foreground">
              <th className="text-left p-3">Symbol</th>
              <th className="text-left p-3">Side</th>
              <th className="text-left p-3">Agent</th>
              <th className="text-right p-3">Entry</th>
              <th className="text-right p-3">Qty</th>
              <th className="text-right p-3">P&L</th>
              <th className="text-left p-3">Status</th>
            </tr>
          </thead>
          <tbody>
            {trades.map((trade) => (
              <tr key={trade.id} className="border-b border-border/50 hover:bg-muted/30">
                <td className="p-3 font-medium">{trade.symbol}</td>
                <td className="p-3">
                  <span className={trade.side === 'long' ? 'text-profit' : 'text-loss'}>
                    {trade.side.toUpperCase()}
                  </span>
                </td>
                <td className="p-3">{trade.agent}</td>
                <td className="p-3 text-right font-mono">{formatCurrency(trade.entryPrice)}</td>
                <td className="p-3 text-right font-mono">{trade.quantity}</td>
                <td className={`p-3 text-right font-mono ${trade.pnl >= 0 ? 'text-profit' : 'text-loss'}`}>
                  {trade.pnl >= 0 ? '+' : ''}{formatCurrency(trade.pnl)}
                </td>
                <td className="p-3">
                  <span className={`text-xs px-2 py-1 rounded-full ${
                    trade.status === 'open' ? 'bg-info/20 text-info' :
                    trade.status === 'closed' ? 'bg-muted text-muted-foreground' :
                    'bg-warning/20 text-warning'
                  }`}>
                    {trade.status}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

export default TradesTable
