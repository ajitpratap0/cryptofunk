// Type guards and raw types for API responses

export interface RawAgent {
  agent_name?: string
  name?: string
  agent_type?: string
  type?: string
  status?: string
  win_rate?: number
  winRate?: number
  total_pnl?: number
  totalPnl?: number
  avg_return?: number
  avgReturn?: number
  total_trades?: number
  totalTrades?: number
  total_signals?: number
  active_positions?: number
  activePositions?: number
  last_action?: string
  lastAction?: string
  updated_at?: string
  timestamp?: string
  version?: string
}

interface RawAgentsResponse {
  agents: RawAgent[]
  count?: number
}

export function isRawAgentsResponse(data: unknown): data is RawAgentsResponse {
  return (
    typeof data === 'object' &&
    data !== null &&
    'agents' in data &&
    Array.isArray((data as RawAgentsResponse).agents)
  )
}

interface WrappedPositions {
  positions: unknown[]
  count?: number
  summary?: unknown
}

export function isWrappedResponse(data: unknown): data is WrappedPositions {
  return (
    typeof data === 'object' &&
    data !== null &&
    !Array.isArray(data) &&
    'positions' in data
  )
}

interface WrappedOrders {
  orders: unknown[]
  count?: number
}

export function isWrappedOrders(data: unknown): data is WrappedOrders {
  return (
    typeof data === 'object' &&
    data !== null &&
    !Array.isArray(data) &&
    'orders' in data
  )
}

interface RawDashboardResponse {
  trading_status?: unknown
  pnl_summary?: {
    total_pnl?: number
    return_percent?: number
    win_rate?: number
    total_trades?: number
    current_capital?: number
    [key: string]: unknown
  }
  position_summary?: {
    open_positions?: number
    total_positions?: number
    total_exposure?: number
    [key: string]: unknown
  }
}

export function isRawDashboardResponse(data: unknown): data is RawDashboardResponse {
  if (typeof data !== 'object' || data === null) return false
  const d = data as Record<string, unknown>
  return 'trading_status' in d || 'pnl_summary' in d || 'position_summary' in d
}

interface RawDashboardPnlResponse {
  total_pnl: number
  realized_pnl?: number
  [key: string]: unknown
}

export function isRawDashboardPnlResponse(data: unknown): data is RawDashboardPnlResponse {
  return (
    typeof data === 'object' &&
    data !== null &&
    'total_pnl' in data &&
    !('equity' in data)
  )
}
