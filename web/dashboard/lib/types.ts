// API Response Types
export interface ApiResponse<T> {
  success: boolean;
  data: T;
  error?: string;
  timestamp: string;
}

// Trading Types
export interface Trade {
  id: string;
  symbol: string;
  side: 'long' | 'short';
  entryPrice: number;
  currentPrice: number;
  quantity: number;
  pnl: number;
  pnlPercent: number;
  agent: string;
  confidence: number;
  timestamp: string;
  status: 'open' | 'closed' | 'pending';
  reasoning?: string;
  exitPrice?: number;
  exitTimestamp?: string;
}

export interface Position {
  symbol: string;
  side: 'long' | 'short';
  size: number;
  entryPrice: number;
  markPrice: number;
  unrealizedPnl: number;
  unrealizedPnlPercent: number;
  marginUsed: number;
  timestamp: string;
}

export interface Order {
  id: string;
  symbol: string;
  side: 'buy' | 'sell';
  type: 'market' | 'limit' | 'stop';
  quantity: number;
  price?: number;
  stopPrice?: number;
  status: 'pending' | 'filled' | 'cancelled' | 'rejected';
  timestamp: string;
  agent: string;
}

// Agent Types
export interface Agent {
  name: string;
  type: string;
  status: 'active' | 'idle' | 'error' | 'disabled';
  winRate: number;
  totalPnl: number;
  avgReturn: number;
  totalTrades: number;
  activePositions: number;
  lastAction: string;
  timestamp: string;
  version: string;
}

export interface AgentDecision {
  id: string;
  agent: string;
  symbol: string;
  action: 'buy' | 'sell' | 'hold';
  confidence: number;
  reasoning: string;
  timestamp: string;
  executed: boolean;
  pnl?: number;
}

// Dashboard Types
export interface DashboardStats {
  totalPnl: number;
  totalPnlPercent: number;
  winRate: number;
  activePositions: number;
  totalTrades: number;
  equity: number;
  availableBalance: number;
  marginUsed: number;
  marginAvailable: number;
}

export interface EquityPoint {
  timestamp: string;
  equity: number;
  pnl: number;
}

export interface PerformanceMetrics {
  sharpeRatio: number;
  sortinoRatio: number;
  maxDrawdown: number;
  maxDrawdownPercent: number;
  calmarRatio: number;
  winRate: number;
  avgWin: number;
  avgLoss: number;
  profitFactor: number;
  totalReturn: number;
}

// Chart Types
export interface CandlestickData {
  time: string;
  open: number;
  high: number;
  low: number;
  close: number;
  volume?: number;
}

export interface TradeMarker {
  time: string;
  position: 'aboveBar' | 'belowBar';
  color: string;
  shape: 'circle' | 'square' | 'arrowUp' | 'arrowDown';
  text: string;
  size: number;
}

// Risk Types
export interface RiskMetrics {
  var95: number;
  var99: number;
  expectedShortfall: number;
  betaToMarket: number;
  correlation: Record<string, number>;
  concentration: Record<string, number>;
}

export interface CircuitBreaker {
  name: string;
  status: 'normal' | 'warning' | 'triggered';
  threshold: number;
  currentValue: number;
  description: string;
}

// WebSocket Types
export interface WebSocketMessage {
  type: 'trade' | 'position' | 'order' | 'agent' | 'market' | 'system';
  data: any;
  timestamp: string;
}

export interface SystemStatus {
  status: 'healthy' | 'degraded' | 'down';
  uptime: number;
  version: string;
  environment: string;
  services: Record<string, 'up' | 'down' | 'degraded'>;
}

// Strategy Types
export interface Strategy {
  name: string;
  version: string;
  description: string;
  parameters: Record<string, any>;
  performance: PerformanceMetrics;
  lastUpdated: string;
}

// Filter Types
export interface TradeFilters {
  symbol?: string;
  agent?: string;
  side?: 'long' | 'short';
  status?: 'open' | 'closed' | 'pending';
  dateFrom?: string;
  dateTo?: string;
}

export interface AgentFilters {
  status?: 'active' | 'idle' | 'error' | 'disabled';
  type?: string;
}

// Polymarket Types
export interface PolymarketMarket {
  id: string;
  question: string;
  category: string | null;
  yes_price: number;
  no_price: number;
  volume: number;
  end_date: string | null;
  active: boolean;
  updated_at: string;
}

export interface PolymarketPosition {
  id: string;
  session_id: string;
  market_id: string;
  side: 'YES' | 'NO';
  shares: number;
  avg_price: number;
  cost_basis: number;
  status: 'OPEN' | 'CLOSED';
  opened_at: string;
  closed_at: string | null;
  realized_pnl: number | null;
  current_price?: number;
  unrealized_pnl?: number;
  market_question?: string;
}

export interface PolymarketTrade {
  id: string;
  position_id: string;
  market_id: string;
  action: 'BUY' | 'SELL';
  side: 'YES' | 'NO';
  amount: number;
  price: number;
  shares: number;
  timestamp: string;
}

export interface PolymarketPrediction {
  id: string;
  market_id: string;
  predicted_prob: number;
  market_price: number;
  edge: number;
  confidence: number;
  category: string | null;
  reasoning: string | null;
  outcome: number | null;
  resolved: boolean;
  pnl: number | null;
  created_at: string;
  resolved_at: string | null;
}

export interface PolymarketPortfolio {
  balance: number;
  total_value: number;
  cost_basis: number;
  unrealized_pnl: number;
  position_count: number;
}

export interface PolymarketPerformance {
  total_predictions: number;
  resolved: number;
  correct: number;
  win_rate: number;
  brier_score: number;
  total_pnl: number;
  by_category: Record<string, {
    total: number;
    correct: number;
    win_rate: number;
    total_pnl: number;
  }>;
}

export interface PaperTradeRequest {
  action: 'BUY' | 'SELL';
  market_id: string;
  question?: string;
  side: 'YES' | 'NO';
  amount: number;
  price: number;
  shares?: number;
  session_id?: string;
}

// Unified Cross-Platform Types
export type UnifiedPlatform = 'binance' | 'polymarket'

export interface UnifiedPosition {
  id: string
  platform: UnifiedPlatform
  session_id: string
  symbol: string
  market_id: string
  side: string
  entry_price: number
  current_price: number
  quantity: number
  cost_basis: number
  unrealized_pnl: number
  realized_pnl: number
  status: 'OPEN' | 'CLOSED'
  opened_at: string
  closed_at: string | null
  agent: string
  confidence: number
  metadata?: Record<string, unknown>
}

export interface UnifiedTrade {
  id: string
  platform: UnifiedPlatform
  position_id: string
  symbol: string
  side: string
  action: 'BUY' | 'SELL'
  price: number
  quantity: number
  amount: number
  agent: string
  confidence: number
  reasoning?: string
  timestamp: string
}

export interface PlatformSummary {
  platform: UnifiedPlatform
  total_value: number
  pnl: number
  position_count: number
  trade_count: number
}

export interface UnifiedPortfolio {
  total_value: number
  cash_balance: number
  unrealized_pnl: number
  realized_pnl: number
  total_pnl: number
  win_rate: number
  total_trades: number
  open_positions: number
  positions: UnifiedPosition[]
  by_platform: Record<UnifiedPlatform, PlatformSummary>
}

// Decision Outcome Types
export interface DecisionWithOutcome {
  id: string
  session_id?: string
  decision_type: string
  symbol: string
  prompt: string
  response: string
  model: string
  tokens_used: number
  latency_ms: number
  outcome?: string
  pnl?: number
  agent_name: string
  confidence: number
  created_at: string
  order_id?: string
  order_side?: string
  order_price?: number
  position_id?: string
  position_pnl?: number
}

export interface AgentAnalytics {
  agent_name: string
  total_decisions: number
  wins: number
  losses: number
  pending: number
  win_rate: number
  avg_confidence: number
  avg_pnl: number
  total_pnl: number
}

export interface ConfidenceBucket {
  bucket_low: number
  bucket_high: number
  count: number
  win_rate: number
  avg_pnl: number
}

export interface DecisionAnalytics {
  total_decisions: number
  resolved_decisions: number
  overall_win_rate: number
  overall_avg_pnl: number
  by_agent: AgentAnalytics[]
  confidence_calibration: ConfidenceBucket[]
}

// Utility Types
export type TimeRange = '1h' | '4h' | '1d' | '1w' | '1m' | '3m' | '1y';
export type ChartType = 'candlestick' | 'line' | 'area' | 'bar';