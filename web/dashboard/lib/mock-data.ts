import type {
  Agent,
  AgentDecision,
  Trade,
  Order,
  DashboardStats,
  EquityPoint,
  UnifiedPortfolio,
} from '@/lib/types'

export const USE_MOCK_DATA = process.env.NEXT_PUBLIC_USE_MOCK_DATA === 'true'

export function getMockDashboard(): DashboardStats {
  return {
    totalPnl: 12547.89,
    totalPnlPercent: 15.67,
    winRate: 68.4,
    activePositions: 7,
    totalTrades: 234,
    equity: 92547.89,
    availableBalance: 45230.12,
    marginUsed: 12500.0,
    marginAvailable: 32730.12,
  }
}

export function getMockTrades(): Trade[] {
  const now = Date.now()
  return [
    {
      id: '1',
      symbol: 'BTC/USDT',
      side: 'long',
      entryPrice: 45230.5,
      currentPrice: 46100.25,
      quantity: 0.5,
      pnl: 434.88,
      pnlPercent: 1.92,
      agent: 'momentum_agent',
      confidence: 85,
      timestamp: new Date(now - 75 * 60 * 1000).toISOString(),
      status: 'open',
      reasoning:
        'Strong bullish momentum detected with volume confirmation and break above resistance',
    },
    {
      id: '2',
      symbol: 'ETH/USDT',
      side: 'long',
      entryPrice: 2845.3,
      currentPrice: 2920.15,
      quantity: 3.2,
      pnl: 239.52,
      pnlPercent: 2.63,
      agent: 'ml_prediction',
      confidence: 92,
      timestamp: new Date(now - 135 * 60 * 1000).toISOString(),
      status: 'open',
      reasoning:
        'ML model predicts upward price movement with 92% confidence based on pattern recognition',
    },
    {
      id: '3',
      symbol: 'BNB/USDT',
      side: 'short',
      entryPrice: 320.45,
      currentPrice: 315.8,
      quantity: 5.0,
      pnl: 23.25,
      pnlPercent: 1.45,
      agent: 'mean_reversion',
      confidence: 78,
      timestamp: new Date(now - 165 * 60 * 1000).toISOString(),
      status: 'open',
      reasoning:
        'Price overextended from mean, expecting reversion to support level',
    },
  ]
}

export function getMockAgents(): Agent[] {
  const now = Date.now()
  return [
    {
      name: 'momentum_agent',
      type: 'momentum',
      status: 'active',
      winRate: 72.5,
      totalPnl: 4520.3,
      avgReturn: 2.8,
      totalTrades: 87,
      activePositions: 3,
      lastAction: 'Buy BTC/USDT',
      timestamp: new Date(now - 75 * 60 * 1000).toISOString(),
      version: '1.2.3',
    },
    {
      name: 'ml_prediction',
      type: 'ml_prediction',
      status: 'active',
      winRate: 69.8,
      totalPnl: 6230.15,
      avgReturn: 3.2,
      totalTrades: 124,
      activePositions: 2,
      lastAction: 'Buy ETH/USDT',
      timestamp: new Date(now - 135 * 60 * 1000).toISOString(),
      version: '2.1.0',
    },
    {
      name: 'mean_reversion',
      type: 'mean_reversion',
      status: 'active',
      winRate: 65.2,
      totalPnl: 1797.44,
      avgReturn: 1.9,
      totalTrades: 156,
      activePositions: 1,
      lastAction: 'Sell BNB/USDT',
      timestamp: new Date(now - 165 * 60 * 1000).toISOString(),
      version: '1.5.2',
    },
    {
      name: 'scalper',
      type: 'scalper',
      status: 'idle',
      winRate: 58.3,
      totalPnl: -234.56,
      avgReturn: 0.5,
      totalTrades: 432,
      activePositions: 0,
      lastAction: 'No trades',
      timestamp: new Date(now - 250 * 60 * 1000).toISOString(),
      version: '1.0.8',
    },
  ]
}

export function getMockEquityPoints(): EquityPoint[] {
  const points: EquityPoint[] = []
  const baseEquity = 80000
  let currentEquity = baseEquity
  const now = Date.now()

  for (let i = 30; i >= 0; i--) {
    const timestamp = new Date(now - i * 24 * 60 * 60 * 1000).toISOString()
    const change = (Math.random() - 0.5) * 2000
    currentEquity += change
    points.push({
      timestamp,
      equity: Math.max(currentEquity, baseEquity * 0.7),
      pnl: change,
    })
  }

  return points
}

export function getMockDecisions(): AgentDecision[] {
  const now = Date.now()
  return [
    {
      id: '1',
      agent: 'momentum_agent',
      symbol: 'BTC/USDT',
      action: 'buy',
      confidence: 85,
      reasoning:
        'Strong bullish momentum detected with volume confirmation and break above resistance at 45000. RSI shows healthy levels with room for upside.',
      timestamp: new Date(now - 75 * 60 * 1000).toISOString(),
      executed: true,
      pnl: 434.88,
    },
    {
      id: '2',
      agent: 'ml_prediction',
      symbol: 'ETH/USDT',
      action: 'buy',
      confidence: 92,
      reasoning:
        'ML model predicts 87% probability of upward price movement in next 4 hours based on pattern recognition and volume analysis.',
      timestamp: new Date(now - 135 * 60 * 1000).toISOString(),
      executed: true,
      pnl: 239.52,
    },
    {
      id: '3',
      agent: 'mean_reversion',
      symbol: 'BNB/USDT',
      action: 'sell',
      confidence: 78,
      reasoning:
        'Price overextended 2.5 standard deviations from 20-period mean. Historical data suggests high probability of reversion.',
      timestamp: new Date(now - 165 * 60 * 1000).toISOString(),
      executed: true,
      pnl: 23.25,
    },
    {
      id: '4',
      agent: 'scalper',
      symbol: 'XRP/USDT',
      action: 'hold',
      confidence: 45,
      reasoning:
        'Low volatility environment with tight spreads. Waiting for clearer directional signal before entering position.',
      timestamp: new Date(now - 180 * 60 * 1000).toISOString(),
      executed: false,
    },
    {
      id: '5',
      agent: 'momentum_agent',
      symbol: 'ADA/USDT',
      action: 'buy',
      confidence: 73,
      reasoning:
        'Breakout above descending triangle pattern with increasing volume. Target price 0.52 with stop loss at 0.47.',
      timestamp: new Date(now - 210 * 60 * 1000).toISOString(),
      executed: true,
      pnl: -45.2,
    },
  ]
}

export function getMockSimilarDecisions(): AgentDecision[] {
  const now = Date.now()
  return [
    {
      id: '6',
      agent: 'momentum_agent',
      symbol: 'BTC/USDT',
      action: 'buy',
      confidence: 82,
      reasoning: 'Similar momentum pattern detected with volume confirmation.',
      timestamp: new Date(now - 24 * 60 * 60 * 1000).toISOString(),
      executed: true,
      pnl: 156.45,
    },
    {
      id: '7',
      agent: 'momentum_agent',
      symbol: 'BTC/USDT',
      action: 'buy',
      confidence: 88,
      reasoning: 'Strong bullish momentum with breakout above resistance.',
      timestamp: new Date(now - 48 * 60 * 60 * 1000).toISOString(),
      executed: true,
      pnl: 289.67,
    },
  ]
}

export function getMockDecisionStats() {
  return {
    total: 1247,
    executed: 892,
    avgConfidence: 74.5,
    byAgent: {
      momentum_agent: 342,
      ml_prediction: 287,
      mean_reversion: 198,
      scalper: 420,
    },
  }
}

export function getMockOrders(): Order[] {
  return [
    {
      id: '1',
      symbol: 'BTC/USDT',
      side: 'buy',
      type: 'limit',
      quantity: 0.1,
      price: 45000,
      status: 'pending',
      timestamp: new Date().toISOString(),
      agent: 'momentum_agent',
    },
    {
      id: '2',
      symbol: 'ETH/USDT',
      side: 'sell',
      type: 'stop',
      quantity: 1.0,
      stopPrice: 2800,
      status: 'pending',
      timestamp: new Date().toISOString(),
      agent: 'risk_manager',
    },
  ]
}

export function getMockPositionsFromTrades() {
  const mockTrades = getMockTrades()
  return mockTrades
    .filter((trade) => trade.status === 'open')
    .map((trade) => ({
      symbol: trade.symbol,
      side: trade.side,
      size: trade.quantity,
      entryPrice: trade.entryPrice,
      markPrice: trade.currentPrice,
      unrealizedPnl: trade.pnl,
      unrealizedPnlPercent: trade.pnlPercent,
      marginUsed: trade.entryPrice * trade.quantity * 0.1,
      timestamp: trade.timestamp,
    }))
}

export function getMockUnifiedPortfolio(): UnifiedPortfolio {
  return {
    total_value: 0,
    cash_balance: 10000,
    unrealized_pnl: 0,
    realized_pnl: 0,
    total_pnl: 0,
    win_rate: 0,
    total_trades: 0,
    open_positions: 0,
    positions: [],
    by_platform: {
      binance: {
        platform: 'binance',
        total_value: 0,
        pnl: 0,
        position_count: 0,
        trade_count: 0,
      },
      polymarket: {
        platform: 'polymarket',
        total_value: 0,
        pnl: 0,
        position_count: 0,
        trade_count: 0,
      },
    },
  }
}
