import {
  ApiResponse,
  Trade,
  Position,
  Order,
  Agent,
  AgentDecision,
  DashboardStats,
  EquityPoint,
  Strategy,
  SystemStatus,
  PolymarketMarket,
  PolymarketPosition,
  PolymarketTrade,
  PolymarketPrediction,
  PolymarketPortfolio,
  PolymarketPerformance,
  PaperTradeRequest,
  UnifiedPortfolio,
  CandlestickData,
} from './types'
import { buildApiUrl } from './utils'

// Shared interface for /api/v1/performance/summary response shape.
// Used by ApiClient.getPerformanceSummary() and re-exported for hooks.
export interface RawPerformanceSummary {
  sharpe_ratio?: number | null
  sortino_ratio?: number | null
  max_drawdown?: number | null
  max_drawdown_percent?: number | null
  calmar_ratio?: number | null
  win_rate?: number | null
  avg_win?: number | null
  avg_loss?: number | null
  profit_factor?: number | null
  total_return?: number | null
}

// One-time warning if NEXT_PUBLIC_API_KEY is unset on the client.
// Without it the dashboard will get cascading 401s with no obvious cause.
let warnedMissingApiKey = false
function warnIfMissingApiKey(): void {
  if (warnedMissingApiKey) return
  if (typeof window === 'undefined') return
  if (!process.env.NEXT_PUBLIC_API_KEY) {
    warnedMissingApiKey = true
    console.warn(
      '[api] NEXT_PUBLIC_API_KEY is not set — API requests will be unauthenticated ' +
        'and will fail if the backend has auth.enabled=true. See web/dashboard/.env.example.'
    )
  }
}

function describeHttpStatus(status: number): string {
  if (status === 401 || status === 403) {
    return `HTTP ${status} unauthorized — check NEXT_PUBLIC_API_KEY`
  }
  if (status >= 500) {
    return `HTTP ${status} backend unavailable`
  }
  return `HTTP ${status}`
}

class ApiClient {
  private baseUrl: string

  constructor(baseUrl?: string) {
    this.baseUrl = baseUrl || process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<ApiResponse<T>> {
    warnIfMissingApiKey()
    try {
      const url = buildApiUrl(endpoint)
      const response = await fetch(url, {
        ...options,
        // Spread caller headers first so our auth/content-type headers win
        // and a caller can never accidentally drop X-API-Key.
        headers: {
          ...(options.headers as Record<string, string> | undefined),
          ...buildHeaders({ 'Content-Type': 'application/json' }),
        },
      })

      if (!response.ok) {
        throw new Error(describeHttpStatus(response.status))
      }

      const data: unknown = await response.json()

      // Handle cases where the API doesn't wrap responses
      if (typeof data === 'object' && data !== null && 'success' in data) {
        return data as ApiResponse<T>
      } else {
        return {
          success: true,
          data: data as T,
          timestamp: new Date().toISOString(),
        }
      }
    } catch (error) {
      console.error(`API request failed: ${endpoint}`, error)
      return {
        success: false,
        data: null as unknown as T,
        error: error instanceof Error ? error.message : 'Unknown error',
        timestamp: new Date().toISOString(),
      }
    }
  }

  // Health & Status
  async getHealth(): Promise<ApiResponse<{ status: string }>> {
    return this.request('/health')
  }

  async getStatus(): Promise<ApiResponse<SystemStatus>> {
    return this.request('/status')
  }

  // Dashboard
  async getDashboard(): Promise<ApiResponse<DashboardStats>> {
    return this.request('/dashboard')
  }

  async getDashboardPositions(): Promise<ApiResponse<Position[]>> {
    return this.request('/dashboard/positions')
  }

  async getDashboardPnl(timeRange?: string): Promise<ApiResponse<{ daily: number; total: number; equity: EquityPoint[] }>> {
    return this.request(timeRange ? `/dashboard/pnl?timeRange=${encodeURIComponent(timeRange)}` : '/dashboard/pnl')
  }

  async getDashboardStatus(): Promise<ApiResponse<SystemStatus>> {
    return this.request('/dashboard/status')
  }

  // Market Data
  // See useCandlestickData() in hooks/usePerformance.ts for the contract:
  // the /market/candlestick endpoint is pending backend implementation.
  async getMarketCandlestick(symbol: string, timeRange: string = '1d'): Promise<ApiResponse<CandlestickData[]>> {
    return this.request(`/market/candlestick/${encodeURIComponent(symbol)}?timeRange=${encodeURIComponent(timeRange)}`)
  }

  // Trading Controls
  async startTrading(): Promise<ApiResponse<{ message: string }>> {
    return this.request('/dashboard/trading/start', { method: 'POST' })
  }

  async stopTrading(): Promise<ApiResponse<{ message: string }>> {
    return this.request('/dashboard/trading/stop', { method: 'POST' })
  }

  async pauseTrading(): Promise<ApiResponse<{ message: string }>> {
    return this.request('/dashboard/trading/pause', { method: 'POST' })
  }

  async resumeTrading(): Promise<ApiResponse<{ message: string }>> {
    return this.request('/dashboard/trading/resume', { method: 'POST' })
  }

  // Unified Portfolio
  async getUnifiedPortfolio(): Promise<ApiResponse<UnifiedPortfolio>> {
    return this.request('/dashboard/unified')
  }

  // Positions
  async getPositions(): Promise<ApiResponse<Position[]>> {
    return this.request('/positions')
  }

  async getPosition(symbol: string): Promise<ApiResponse<Position>> {
    return this.request(`/positions/${encodeURIComponent(symbol)}`)
  }

  // Orders
  async getOrders(): Promise<ApiResponse<Order[]>> {
    return this.request('/orders')
  }

  async getOrder(id: string): Promise<ApiResponse<Order>> {
    return this.request(`/orders/${id}`)
  }

  async createOrder(order: Partial<Order>): Promise<ApiResponse<Order>> {
    return this.request('/orders', {
      method: 'POST',
      body: JSON.stringify(order),
    })
  }

  async deleteOrder(id: string): Promise<ApiResponse<{ message: string }>> {
    return this.request(`/orders/${id}`, { method: 'DELETE' })
  }

  // Trade Control
  async startTrade(): Promise<ApiResponse<{ message: string }>> {
    return this.request('/trade/start', { method: 'POST' })
  }

  async stopTrade(): Promise<ApiResponse<{ message: string }>> {
    return this.request('/trade/stop', { method: 'POST' })
  }

  async pauseTrade(): Promise<ApiResponse<{ message: string }>> {
    return this.request('/trade/pause', { method: 'POST' })
  }

  async resumeTrade(): Promise<ApiResponse<{ message: string }>> {
    return this.request('/trade/resume', { method: 'POST' })
  }

  // Agents
  async getAgents(): Promise<ApiResponse<Agent[]>> {
    return this.request('/agents')
  }

  async getAgent(name: string): Promise<ApiResponse<Agent>> {
    return this.request(`/agents/${encodeURIComponent(name)}`)
  }

  async getAgentStatus(name: string): Promise<ApiResponse<{ status: string; lastAction: string }>> {
    return this.request(`/agents/${encodeURIComponent(name)}/status`)
  }

  // Decisions
  async getDecisions(filters?: { limit?: number; agent?: string }): Promise<ApiResponse<AgentDecision[]>> {
    const params = new URLSearchParams()
    if (filters?.limit) params.set('limit', filters.limit.toString())
    if (filters?.agent) params.set('agent', filters.agent)
    
    const query = params.toString()
    return this.request(`/decisions${query ? `?${query}` : ''}`)
  }

  async getDecisionStats(): Promise<ApiResponse<{ 
    total: number; 
    executed: number; 
    avgConfidence: number; 
    byAgent: Record<string, number> 
  }>> {
    return this.request('/decisions/stats')
  }

  async getDecision(id: string): Promise<ApiResponse<AgentDecision>> {
    return this.request(`/decisions/${id}`)
  }

  async getSimilarDecisions(id: string): Promise<ApiResponse<AgentDecision[]>> {
    return this.request(`/decisions/${id}/similar`)
  }

  async searchDecisions(query: {
    agent?: string;
    symbol?: string;
    action?: string;
    dateFrom?: string;
    dateTo?: string;
  }): Promise<ApiResponse<AgentDecision[]>> {
    return this.request('/decisions/search', {
      method: 'POST',
      body: JSON.stringify(query),
    })
  }

  // Strategies
  async getCurrentStrategy(): Promise<ApiResponse<Strategy>> {
    return this.request('/strategies/current')
  }

  async exportStrategy(): Promise<ApiResponse<{ config: Record<string, unknown> }>> {
    return this.request('/strategies/export')
  }

  async getStrategyVersion(): Promise<ApiResponse<{ version: string; hash: string }>> {
    return this.request('/strategies/version')
  }

  async getStrategySchema(): Promise<ApiResponse<{ schema: Record<string, unknown> }>> {
    return this.request('/strategies/schema')
  }

  // Config
  async getConfig(): Promise<ApiResponse<Record<string, unknown>>> {
    return this.request('/config')
  }

  // Polymarket
  async getPolymarketMarkets(active?: boolean): Promise<ApiResponse<{ markets: PolymarketMarket[]; count: number }>> {
    return this.request(`/polymarket/markets?active=${active !== false}`)
  }

  async getPolymarketMarket(id: string): Promise<ApiResponse<{ market: PolymarketMarket }>> {
    return this.request(`/polymarket/markets/${encodeURIComponent(id)}`)
  }

  async getPolymarketPositions(live?: boolean): Promise<ApiResponse<{ positions: PolymarketPosition[]; count: number }>> {
    return this.request(`/polymarket/positions?live=${live || false}`)
  }

  async getPolymarketPosition(id: string): Promise<ApiResponse<{ position: PolymarketPosition }>> {
    return this.request(`/polymarket/positions/${id}`)
  }

  async getPolymarketPortfolio(): Promise<ApiResponse<PolymarketPortfolio>> {
    return this.request('/polymarket/portfolio')
  }

  async getPolymarketTrades(limit?: number): Promise<ApiResponse<{ trades: PolymarketTrade[]; count: number }>> {
    return this.request(`/polymarket/trades?limit=${limit || 100}`)
  }

  async getPolymarketPredictions(resolved?: boolean): Promise<ApiResponse<{ predictions: PolymarketPrediction[]; count: number }>> {
    return this.request(`/polymarket/predictions?resolved=${resolved || false}`)
  }

  async getPolymarketPerformance(): Promise<ApiResponse<PolymarketPerformance>> {
    return this.request('/polymarket/performance')
  }

  async executePaperTrade(trade: PaperTradeRequest): Promise<ApiResponse<{ trade: Record<string, unknown>; balance: number; message: string }>> {
    return this.request('/polymarket/trade', {
      method: 'POST',
      body: JSON.stringify(trade),
    })
  }

  // Trades (fill records)
  async getTrades(limit = 50, offset = 0): Promise<ApiResponse<{ trades: Trade[]; count: number }>> {
    return this.request(`/trades?limit=${limit}&offset=${offset}`)
  }

  // Risk metrics
  async getRiskMetrics(): Promise<ApiResponse<{
    open_positions: number
    total_exposure: number
    data_points: number
    var_95: number | null
    var_99: number | null
    expected_shortfall: number | null
  }>> {
    return this.request('/risk/metrics')
  }

  async getCircuitBreakers(): Promise<ApiResponse<{
    circuit_breakers: Array<{ name: string; current: number; threshold: number; status: string }>
    count: number
  }>> {
    return this.request('/risk/circuit-breakers')
  }

  async getRiskExposure(): Promise<ApiResponse<{
    exposure: Array<{ symbol: string; exposure: number }>
    count: number
  }>> {
    return this.request('/risk/exposure')
  }

  // Performance by trading pair
  async getPairPerformance(): Promise<ApiResponse<{
    pairs: Array<{ symbol: string; realized_pnl: number; trade_count: number }>
    count: number
  }>> {
    return this.request('/performance/pairs')
  }

  // Performance summary metrics (sharpe, sortino, win rate, etc.)
  async getPerformanceSummary(): Promise<ApiResponse<RawPerformanceSummary>> {
    return this.request('/performance/summary')
  }
}

export const apiClient = new ApiClient()

// Build common headers, including the API key when set.
// NOTE: NEXT_PUBLIC_* is shipped to the client bundle — never put a privileged
// production key in NEXT_PUBLIC_API_KEY. See web/dashboard/.env.example.
function buildHeaders(extra?: Record<string, string>): Record<string, string> {
  const headers: Record<string, string> = { ...extra }
  const apiKey = process.env.NEXT_PUBLIC_API_KEY
  if (apiKey) {
    headers['X-API-Key'] = apiKey
  }
  return headers
}

// Shared helper for the standalone decision-analytics fetchers below.
// Uses the same auth + status semantics as ApiClient.request so a 401/500
// is never silently coerced into `{success:true, data:<error body>}`.
async function jsonRequest<T>(
  endpoint: string,
  init: RequestInit = {}
): Promise<ApiResponse<T>> {
  warnIfMissingApiKey()
  try {
    const res = await fetch(buildApiUrl(endpoint), {
      ...init,
      headers: {
        ...(init.headers as Record<string, string> | undefined),
        ...buildHeaders({ 'Content-Type': 'application/json' }),
      },
    })
    if (!res.ok) {
      throw new Error(describeHttpStatus(res.status))
    }
    const data: unknown = await res.json()
    if (typeof data === 'object' && data !== null && 'success' in data) {
      return data as ApiResponse<T>
    }
    return {
      success: true,
      data: data as T,
      timestamp: new Date().toISOString(),
    }
  } catch (error) {
    console.error(`API request failed: ${endpoint}`, error)
    return {
      success: false,
      data: null as unknown as T,
      error: error instanceof Error ? error.message : 'Unknown error',
      timestamp: new Date().toISOString(),
    }
  }
}

// Decision Analytics API
export async function fetchDecisionAnalytics(
  since = '30d'
): Promise<ApiResponse<import('./types').DecisionAnalytics>> {
  return jsonRequest<import('./types').DecisionAnalytics>(
    `/decisions/analytics?since=${encodeURIComponent(since)}`
  )
}

export async function fetchDecisionOutcomes(
  limit = 50
): Promise<ApiResponse<import('./types').DecisionWithOutcome[]>> {
  return jsonRequest<import('./types').DecisionWithOutcome[]>(
    `/decisions/outcomes?limit=${limit}`
  )
}

export async function triggerOutcomeResolution(): Promise<
  ApiResponse<{ polymarket_resolved: number; binance_resolved: number }>
> {
  return jsonRequest<{ polymarket_resolved: number; binance_resolved: number }>(
    '/admin/resolve-outcomes',
    { method: 'POST' }
  )
}

export default ApiClient
