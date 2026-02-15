// API Configuration
export const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';
export const API_VERSION = 'v1';
export const API_PREFIX = `/api/${API_VERSION}`;

// WebSocket Configuration
export const WS_URL = API_BASE_URL.replace('http', 'ws') + `${API_PREFIX}/ws`;
export const WS_RECONNECT_ATTEMPTS = 5;
export const WS_RECONNECT_INTERVAL = 3000;

// Refresh Intervals (milliseconds)
export const REFRESH_INTERVALS = {
  dashboard: 5000,
  trades: 2000,
  positions: 3000,
  agents: 10000,
  performance: 30000,
  risk: 15000,
} as const;

// Colors
export const COLORS = {
  profit: '#22c55e',
  loss: '#ef4444',
  warning: '#f59e0b',
  info: '#3b82f6',
  neutral: '#6b7280',
  primary: '#8b5cf6',
  secondary: '#64748b',
  background: '#0f172a',
  card: '#1e293b',
  border: '#334155',
} as const;

// Chart Colors
export const CHART_COLORS = [
  '#8b5cf6', '#22c55e', '#ef4444', '#f59e0b', 
  '#3b82f6', '#ec4899', '#10b981', '#f97316',
  '#06b6d4', '#84cc16', '#a855f7', '#e11d48'
] as const;

// Trading Pairs
export const POPULAR_PAIRS = [
  'BTC/USDT', 'ETH/USDT', 'BNB/USDT', 'XRP/USDT',
  'ADA/USDT', 'SOL/USDT', 'DOT/USDT', 'MATIC/USDT',
  'AVAX/USDT', 'LINK/USDT', 'UNI/USDT', 'LTC/USDT'
] as const;

// Time Ranges
export const TIME_RANGES = {
  '1h': { label: '1 Hour', value: '1h', minutes: 60 },
  '4h': { label: '4 Hours', value: '4h', minutes: 240 },
  '1d': { label: '1 Day', value: '1d', minutes: 1440 },
  '1w': { label: '1 Week', value: '1w', minutes: 10080 },
  '1m': { label: '1 Month', value: '1m', minutes: 43200 },
  '3m': { label: '3 Months', value: '3m', minutes: 129600 },
  '1y': { label: '1 Year', value: '1y', minutes: 525600 },
} as const;

// Chart Intervals
export const CHART_INTERVALS = {
  '1m': 60000,
  '5m': 300000,
  '15m': 900000,
  '1h': 3600000,
  '4h': 14400000,
  '1d': 86400000,
  '1w': 604800000,
} as const;

// Agent Types
export const AGENT_TYPES = [
  'momentum', 'mean_reversion', 'arbitrage', 'scalper',
  'swing', 'trend_following', 'ml_prediction', 'sentiment'
] as const;

// Order Types
export const ORDER_TYPES = ['market', 'limit', 'stop', 'stop_limit'] as const;

// Position Sides
export const POSITION_SIDES = ['long', 'short'] as const;

// Trade Statuses
export const TRADE_STATUSES = ['open', 'closed', 'pending'] as const;

// Agent Statuses
export const AGENT_STATUSES = ['active', 'idle', 'error', 'disabled'] as const;

// System Status Levels
export const SYSTEM_STATUS_LEVELS = ['healthy', 'degraded', 'down'] as const;

// Risk Thresholds
export const RISK_THRESHOLDS = {
  drawdown: {
    warning: 0.05, // 5%
    critical: 0.10, // 10%
  },
  var: {
    warning: 0.02, // 2%
    critical: 0.05, // 5%
  },
  concentration: {
    warning: 0.30, // 30%
    critical: 0.50, // 50%
  },
  leverage: {
    warning: 5,
    critical: 10,
  },
} as const;

// Performance Benchmarks
export const PERFORMANCE_BENCHMARKS = {
  sharpeRatio: {
    excellent: 2.0,
    good: 1.0,
    acceptable: 0.5,
  },
  winRate: {
    excellent: 0.65,
    good: 0.55,
    acceptable: 0.45,
  },
  maxDrawdown: {
    excellent: 0.05,
    good: 0.10,
    acceptable: 0.20,
  },
} as const;

// Animation Durations
export const ANIMATIONS = {
  fast: 150,
  normal: 300,
  slow: 500,
  flash: 1000,
} as const;

// Table Pagination
export const PAGINATION = {
  defaultPageSize: 20,
  pageSizeOptions: [10, 20, 50, 100],
  maxPages: 10,
} as const;

// Number Formatting
export const FORMATTING = {
  currency: {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  },
  percentage: {
    minimumFractionDigits: 1,
    maximumFractionDigits: 2,
  },
  crypto: {
    minimumFractionDigits: 2,
    maximumFractionDigits: 8,
  },
} as const;