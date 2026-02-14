import { 
  ApiResponse, 
  Trade, 
  Position, 
  Order, 
  Agent, 
  AgentDecision,
  DashboardStats,
  EquityPoint,
  PerformanceMetrics,
  Strategy,
  SystemStatus,
  RiskMetrics,
  CircuitBreaker,
  TradeFilters,
  AgentFilters,
  CandlestickData,
  TimeRange
} from './types'
import { buildApiUrl } from './utils'

class ApiClient {
  private baseUrl: string

  constructor(baseUrl?: string) {
    this.baseUrl = baseUrl || process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
  }

  private async request<T>(
    endpoint: string, 
    options: RequestInit = {}
  ): Promise<ApiResponse<T>> {
    try {
      const url = buildApiUrl(endpoint)
      const response = await fetch(url, {
        headers: {
          'Content-Type': 'application/json',
          ...options.headers,
        },
        ...options,
      })

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`)
      }

      const data = await response.json()
      
      // Handle cases where the API doesn't wrap responses
      if (data.success !== undefined) {
        return data
      } else {
        return {
          success: true,
          data,
          timestamp: new Date().toISOString(),
        }
      }
    } catch (error) {
      console.error(`API request failed: ${endpoint}`, error)
      return {
        success: false,
        data: null as any,
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

  async getDashboardPnl(): Promise<ApiResponse<{ daily: number; total: number; equity: EquityPoint[] }>> {
    return this.request('/dashboard/pnl')
  }

  async getDashboardStatus(): Promise<ApiResponse<SystemStatus>> {
    return this.request('/dashboard/status')
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

  async exportStrategy(): Promise<ApiResponse<{ config: any }>> {
    return this.request('/strategies/export')
  }

  async getStrategyVersion(): Promise<ApiResponse<{ version: string; hash: string }>> {
    return this.request('/strategies/version')
  }

  async getStrategySchema(): Promise<ApiResponse<{ schema: any }>> {
    return this.request('/strategies/schema')
  }

  // Config
  async getConfig(): Promise<ApiResponse<any>> {
    return this.request('/config')
  }

  // Mock Data Methods (for fallback when API is unreachable)
  getMockDashboard(): DashboardStats {
    return {
      totalPnl: 12547.89,
      totalPnlPercent: 15.67,
      winRate: 68.4,
      activePositions: 7,
      totalTrades: 234,
      equity: 92547.89,
      availableBalance: 45230.12,
      marginUsed: 12500.00,
      marginAvailable: 32730.12,
    }
  }

  getMockTrades(): Trade[] {
    const now = Date.now()
    return [
      {
        id: '1',
        symbol: 'BTC/USDT',
        side: 'long',
        entryPrice: 45230.50,
        currentPrice: 46100.25,
        quantity: 0.5,
        pnl: 434.88,
        pnlPercent: 1.92,
        agent: 'momentum_agent',
        confidence: 85,
        timestamp: new Date(now - 75 * 60 * 1000).toISOString(), // 1h 15m ago
        status: 'open',
        reasoning: 'Strong bullish momentum detected with volume confirmation and break above resistance'
      },
      {
        id: '2',
        symbol: 'ETH/USDT',
        side: 'long',
        entryPrice: 2845.30,
        currentPrice: 2920.15,
        quantity: 3.2,
        pnl: 239.52,
        pnlPercent: 2.63,
        agent: 'ml_prediction',
        confidence: 92,
        timestamp: new Date(now - 135 * 60 * 1000).toISOString(), // 2h 15m ago
        status: 'open',
        reasoning: 'ML model predicts upward price movement with 92% confidence based on pattern recognition'
      },
      {
        id: '3',
        symbol: 'BNB/USDT',
        side: 'short',
        entryPrice: 320.45,
        currentPrice: 315.80,
        quantity: 5.0,
        pnl: 23.25,
        pnlPercent: 1.45,
        agent: 'mean_reversion',
        confidence: 78,
        timestamp: new Date(now - 165 * 60 * 1000).toISOString(), // 2h 45m ago
        status: 'open',
        reasoning: 'Price overextended from mean, expecting reversion to support level'
      }
    ]
  }

  getMockAgents(): Agent[] {
    const now = Date.now()
    return [
      {
        name: 'momentum_agent',
        type: 'momentum',
        status: 'active',
        winRate: 72.5,
        totalPnl: 4520.30,
        avgReturn: 2.8,
        totalTrades: 87,
        activePositions: 3,
        lastAction: 'Buy BTC/USDT',
        timestamp: new Date(now - 75 * 60 * 1000).toISOString(), // 1h 15m ago
        version: '1.2.3'
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
        timestamp: new Date(now - 135 * 60 * 1000).toISOString(), // 2h 15m ago
        version: '2.1.0'
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
        timestamp: new Date(now - 165 * 60 * 1000).toISOString(), // 2h 45m ago
        version: '1.5.2'
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
        timestamp: new Date(now - 250 * 60 * 1000).toISOString(), // 4h 10m ago
        version: '1.0.8'
      }
    ]
  }

  getMockEquityPoints(): EquityPoint[] {
    const points: EquityPoint[] = []
    const baseEquity = 80000
    let currentEquity = baseEquity
    const now = Date.now()
    
    for (let i = 30; i >= 0; i--) {
      const timestamp = new Date(now - i * 24 * 60 * 60 * 1000).toISOString() // i days ago
      
      const change = (Math.random() - 0.5) * 2000
      currentEquity += change
      
      points.push({
        timestamp,
        equity: Math.max(currentEquity, baseEquity * 0.7), // Don't go below 70% of initial
        pnl: change
      })
    }
    
    return points
  }
}

export const apiClient = new ApiClient()
export default ApiClient