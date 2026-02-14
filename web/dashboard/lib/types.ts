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

// Utility Types
export type TimeRange = '1h' | '4h' | '1d' | '1w' | '1m' | '3m' | '1y';
export type ChartType = 'candlestick' | 'line' | 'area' | 'bar';