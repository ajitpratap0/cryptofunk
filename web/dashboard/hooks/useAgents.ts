'use client'

import { useQuery, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '@/lib/api'
import { REFRESH_INTERVALS } from '@/lib/constants'
import type { Agent, AgentDecision, AgentFilters } from '@/lib/types'

// Query Keys
export const AGENT_QUERY_KEYS = {
  agents: ['agents'],
  agentDetails: (name: string) => ['agents', name],
  agentStatus: (name: string) => ['agents', name, 'status'],
  decisions: ['decisions'],
  decisionStats: ['decisions', 'stats'],
  decisionDetails: (id: string) => ['decisions', id],
  similarDecisions: (id: string) => ['decisions', id, 'similar'],
} as const

// Agents
export function useAgents(filters?: AgentFilters) {
  return useQuery({
    queryKey: [...AGENT_QUERY_KEYS.agents, filters],
    queryFn: async () => {
      let response
      try {
        response = await apiClient.getAgents()
      } catch {
        // API unreachable - fall through to mock
      }
      
      // Fallback to mock data if API fails
      if (!response?.success) {
        let agents = apiClient.getMockAgents()
        
        // Apply filters
        if (filters?.status) {
          agents = agents.filter(agent => agent.status === filters.status)
        }
        if (filters?.type) {
          agents = agents.filter(agent => agent.type === filters.type)
        }
        
        return {
          success: true,
          data: agents,
          timestamp: new Date().toISOString(),
        }
      }
      
      // API returns {agents: [...], count: N} - extract and transform the array
      const rawData = response.data as any
      const agentsArray = Array.isArray(rawData) ? rawData : (rawData?.agents || [])
      
      // Transform API agent shape to frontend Agent type
      const transformedAgents: Agent[] = agentsArray.map((a: any) => ({
        name: a.agent_name || a.name || '',
        type: a.agent_type || a.type || '',
        status: (a.status || 'idle').toLowerCase() as Agent['status'],
        winRate: a.win_rate ?? a.winRate ?? 0,
        totalPnl: a.total_pnl ?? a.totalPnl ?? 0,
        avgReturn: a.avg_return ?? a.avgReturn ?? 0,
        totalTrades: a.total_trades ?? a.totalTrades ?? a.total_signals ?? 0,
        activePositions: a.active_positions ?? a.activePositions ?? 0,
        lastAction: a.last_action ?? a.lastAction ?? '',
        timestamp: a.updated_at ?? a.timestamp ?? new Date().toISOString(),
        version: a.version ?? '',
      }))
      
      // Apply filters
      let filtered = transformedAgents
      if (filters?.status) {
        filtered = filtered.filter(agent => agent.status === filters.status)
      }
      if (filters?.type) {
        filtered = filtered.filter(agent => agent.type === filters.type)
      }
      
      return {
        success: true,
        data: filtered,
        timestamp: response.timestamp,
      }
    },
    staleTime: REFRESH_INTERVALS.agents,
    refetchInterval: REFRESH_INTERVALS.agents,
  })
}

export function useAgent(name: string) {
  return useQuery({
    queryKey: AGENT_QUERY_KEYS.agentDetails(name),
    queryFn: async () => {
      let response
      try {
        response = await apiClient.getAgent(name)
      } catch {
        // API unreachable - fall through to mock
      }
      
      // Fallback to mock data if API fails
      if (!response?.success) {
        const agents = apiClient.getMockAgents()
        const agent = agents.find(a => a.name === name)
        
        return {
          success: true,
          data: agent || null,
          timestamp: new Date().toISOString(),
        }
      }
      
      return response
    },
    enabled: !!name,
    staleTime: REFRESH_INTERVALS.agents,
  })
}

export function useAgentStatus(name: string) {
  return useQuery({
    queryKey: AGENT_QUERY_KEYS.agentStatus(name),
    queryFn: async () => {
      let response
      try {
        response = await apiClient.getAgentStatus(name)
      } catch {
        // API unreachable - fall through to mock
      }
      
      // Fallback to mock data if API fails
      if (!response?.success) {
        const agents = apiClient.getMockAgents()
        const agent = agents.find(a => a.name === name)
        
        return {
          success: true,
          data: agent ? {
            status: agent.status,
            lastAction: agent.lastAction,
          } : null,
          timestamp: new Date().toISOString(),
        }
      }
      
      return response
    },
    enabled: !!name,
    staleTime: 5000, // More frequent updates for status
    refetchInterval: 5000,
  })
}

// Decisions
export function useDecisions(filters?: { limit?: number; agent?: string }) {
  return useQuery({
    queryKey: [...AGENT_QUERY_KEYS.decisions, filters],
    queryFn: async () => {
      let response
      try {
        response = await apiClient.getDecisions(filters)
      } catch {
        // API unreachable - fall through to mock
      }
      
      // Fallback to mock data if API fails
      if (!response?.success) {
        const now = Date.now()
        const mockDecisions: AgentDecision[] = [
          {
            id: '1',
            agent: 'momentum_agent',
            symbol: 'BTC/USDT',
            action: 'buy',
            confidence: 85,
            reasoning: 'Strong bullish momentum detected with volume confirmation and break above resistance at 45000. RSI shows healthy levels with room for upside.',
            timestamp: new Date(now - 75 * 60 * 1000).toISOString(), // 1h 15m ago
            executed: true,
            pnl: 434.88,
          },
          {
            id: '2',
            agent: 'ml_prediction',
            symbol: 'ETH/USDT',
            action: 'buy',
            confidence: 92,
            reasoning: 'ML model predicts 87% probability of upward price movement in next 4 hours based on pattern recognition and volume analysis.',
            timestamp: new Date(now - 135 * 60 * 1000).toISOString(), // 2h 15m ago
            executed: true,
            pnl: 239.52,
          },
          {
            id: '3',
            agent: 'mean_reversion',
            symbol: 'BNB/USDT',
            action: 'sell',
            confidence: 78,
            reasoning: 'Price overextended 2.5 standard deviations from 20-period mean. Historical data suggests high probability of reversion.',
            timestamp: new Date(now - 165 * 60 * 1000).toISOString(), // 2h 45m ago
            executed: true,
            pnl: 23.25,
          },
          {
            id: '4',
            agent: 'scalper',
            symbol: 'XRP/USDT',
            action: 'hold',
            confidence: 45,
            reasoning: 'Low volatility environment with tight spreads. Waiting for clearer directional signal before entering position.',
            timestamp: new Date(now - 180 * 60 * 1000).toISOString(), // 3h ago
            executed: false,
          },
          {
            id: '5',
            agent: 'momentum_agent',
            symbol: 'ADA/USDT',
            action: 'buy',
            confidence: 73,
            reasoning: 'Breakout above descending triangle pattern with increasing volume. Target price 0.52 with stop loss at 0.47.',
            timestamp: new Date(now - 210 * 60 * 1000).toISOString(), // 3h 30m ago
            executed: true,
            pnl: -45.20,
          },
        ]
        
        let filteredDecisions = mockDecisions
        
        if (filters?.agent) {
          filteredDecisions = filteredDecisions.filter(d => d.agent === filters.agent)
        }
        
        if (filters?.limit) {
          filteredDecisions = filteredDecisions.slice(0, filters.limit)
        }
        
        return {
          success: true,
          data: filteredDecisions,
          timestamp: new Date().toISOString(),
        }
      }
      
      return response
    },
    staleTime: REFRESH_INTERVALS.agents,
    refetchInterval: REFRESH_INTERVALS.agents,
  })
}

export function useDecisionStats() {
  return useQuery({
    queryKey: AGENT_QUERY_KEYS.decisionStats,
    queryFn: async () => {
      let response
      try {
        response = await apiClient.getDecisionStats()
      } catch {
        // API unreachable - fall through to mock
      }
      
      // Fallback to mock data if API fails
      if (!response?.success) {
        return {
          success: true,
          data: {
            total: 1247,
            executed: 892,
            avgConfidence: 74.5,
            byAgent: {
              'momentum_agent': 342,
              'ml_prediction': 287,
              'mean_reversion': 198,
              'scalper': 420,
            },
          },
          timestamp: new Date().toISOString(),
        }
      }
      
      return response
    },
    staleTime: REFRESH_INTERVALS.agents,
    refetchInterval: REFRESH_INTERVALS.agents,
  })
}

export function useDecision(id: string) {
  return useQuery({
    queryKey: AGENT_QUERY_KEYS.decisionDetails(id),
    queryFn: async () => {
      let response
      try {
        response = await apiClient.getDecision(id)
      } catch {
        // API unreachable - fall through to mock
      }
      
      // Fallback to mock data if API fails
      if (!response?.success) {
        let decisionsResp
        try { decisionsResp = await apiClient.getDecisions() } catch { /* ignore */ }
        const decision = decisionsResp?.data?.find((d: any) => d.id === id)
        
        return {
          success: true,
          data: decision || null,
          timestamp: new Date().toISOString(),
        }
      }
      
      return response
    },
    enabled: !!id,
    staleTime: 60000, // Decisions don't change often
  })
}

export function useSimilarDecisions(id: string) {
  return useQuery({
    queryKey: AGENT_QUERY_KEYS.similarDecisions(id),
    queryFn: async () => {
      let response
      try {
        response = await apiClient.getSimilarDecisions(id)
      } catch {
        // API unreachable - fall through to mock
      }
      
      // Fallback to mock data if API fails
      if (!response?.success) {
        // Return a subset of decisions as "similar"
        const now = Date.now()
        const mockDecisions: AgentDecision[] = [
          {
            id: '6',
            agent: 'momentum_agent',
            symbol: 'BTC/USDT',
            action: 'buy',
            confidence: 82,
            reasoning: 'Similar momentum pattern detected with volume confirmation.',
            timestamp: new Date(now - 24 * 60 * 60 * 1000).toISOString(), // 1 day ago
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
            timestamp: new Date(now - 48 * 60 * 60 * 1000).toISOString(), // 2 days ago
            executed: true,
            pnl: 289.67,
          },
        ]
        
        return {
          success: true,
          data: mockDecisions,
          timestamp: new Date().toISOString(),
        }
      }
      
      return response
    },
    enabled: !!id,
    staleTime: 300000, // 5 minutes - similar decisions don't change
  })
}

// Search decisions
export function useSearchDecisions() {
  const queryClient = useQueryClient()
  
  return {
    searchDecisions: async (query: {
      agent?: string
      symbol?: string
      action?: string
      dateFrom?: string
      dateTo?: string
    }) => {
      const response = await apiClient.searchDecisions(query)
      
      // Cache the results
      queryClient.setQueryData(
        [...AGENT_QUERY_KEYS.decisions, 'search', query],
        response
      )
      
      return response
    },
  }
}

// Computed queries for analytics
export function useAgentPerformance() {
  const { data: agents } = useAgents()
  const { data: decisions } = useDecisions()
  
  return useQuery({
    queryKey: ['agent-performance', agents?.data, decisions?.data],
    queryFn: () => {
      if (!agents?.data || !decisions?.data) return null
      
      return agents.data.map(agent => {
        const agentDecisions = decisions.data.filter(d => d.agent === agent.name)
        const executedDecisions = agentDecisions.filter(d => d.executed)
        const profitableDecisions = executedDecisions.filter(d => d.pnl && d.pnl > 0)
        
        return {
          agent: agent.name,
          totalDecisions: agentDecisions.length,
          executedDecisions: executedDecisions.length,
          winRate: executedDecisions.length > 0 ? 
            (profitableDecisions.length / executedDecisions.length) * 100 : 0,
          avgConfidence: agentDecisions.length > 0 ?
            agentDecisions.reduce((sum, d) => sum + d.confidence, 0) / agentDecisions.length : 0,
          totalPnl: executedDecisions.reduce((sum, d) => sum + (d.pnl || 0), 0),
        }
      })
    },
    enabled: !!(agents?.data && decisions?.data),
    staleTime: REFRESH_INTERVALS.performance,
  })
}