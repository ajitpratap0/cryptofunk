'use client'

import { useQuery, useQueryClient } from '@tanstack/react-query'
import { apiClient } from '@/lib/api'
import { REFRESH_INTERVALS } from '@/lib/constants'
import {
  USE_MOCK_DATA,
  getMockAgents,
  getMockDecisions,
  getMockSimilarDecisions,
  getMockDecisionStats,
} from '@/lib/mock-data'
import {
  isRawAgentsResponse,
  type RawAgent,
} from '@/types/api-responses'
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

function transformRawAgent(a: RawAgent): Agent {
  return {
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
  }
}

function applyAgentFilters(agents: Agent[], filters?: AgentFilters): Agent[] {
  let filtered = agents
  if (filters?.status) {
    filtered = filtered.filter(agent => agent.status === filters.status)
  }
  if (filters?.type) {
    filtered = filtered.filter(agent => agent.type === filters.type)
  }
  return filtered
}

// Agents
export function useAgents(filters?: AgentFilters) {
  return useQuery({
    queryKey: [...AGENT_QUERY_KEYS.agents, filters],
    queryFn: async () => {
      if (USE_MOCK_DATA) {
        return {
          success: true as const,
          data: applyAgentFilters(getMockAgents(), filters),
          timestamp: new Date().toISOString(),
        }
      }

      const response = await apiClient.getAgents()
      
      if (!response.success) {
        throw new Error(response.error || 'Failed to fetch agents')
      }
      
      // API returns {agents: [...], count: N} - extract and transform
      const rawData: unknown = response.data
      const agentsArray: RawAgent[] = Array.isArray(rawData)
        ? rawData
        : isRawAgentsResponse(rawData)
          ? rawData.agents
          : []
      
      const transformedAgents = agentsArray.map(transformRawAgent)
      
      return {
        success: true as const,
        data: applyAgentFilters(transformedAgents, filters),
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
      if (USE_MOCK_DATA) {
        const agent = getMockAgents().find(a => a.name === name)
        return {
          success: true as const,
          data: agent || null,
          timestamp: new Date().toISOString(),
        }
      }

      const response = await apiClient.getAgent(name)
      if (!response.success) {
        throw new Error(response.error || 'Failed to fetch agent')
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
      if (USE_MOCK_DATA) {
        const agent = getMockAgents().find(a => a.name === name)
        return {
          success: true as const,
          data: agent ? { status: agent.status, lastAction: agent.lastAction } : null,
          timestamp: new Date().toISOString(),
        }
      }

      const response = await apiClient.getAgentStatus(name)
      if (!response.success) {
        throw new Error(response.error || 'Failed to fetch agent status')
      }
      return response
    },
    enabled: !!name,
    staleTime: 5000,
    refetchInterval: 5000,
  })
}

// Decisions
export function useDecisions(filters?: { limit?: number; agent?: string }) {
  return useQuery({
    queryKey: [...AGENT_QUERY_KEYS.decisions, filters],
    queryFn: async () => {
      if (USE_MOCK_DATA) {
        let decisions = getMockDecisions()
        if (filters?.agent) {
          decisions = decisions.filter(d => d.agent === filters.agent)
        }
        if (filters?.limit) {
          decisions = decisions.slice(0, filters.limit)
        }
        return {
          success: true as const,
          data: decisions,
          timestamp: new Date().toISOString(),
        }
      }

      const response = await apiClient.getDecisions(filters)
      if (!response.success) {
        throw new Error(response.error || 'Failed to fetch decisions')
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
      if (USE_MOCK_DATA) {
        return {
          success: true as const,
          data: getMockDecisionStats(),
          timestamp: new Date().toISOString(),
        }
      }

      const response = await apiClient.getDecisionStats()
      if (!response.success) {
        throw new Error(response.error || 'Failed to fetch decision stats')
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
      if (USE_MOCK_DATA) {
        const decision = getMockDecisions().find(d => d.id === id)
        return {
          success: true as const,
          data: decision || null,
          timestamp: new Date().toISOString(),
        }
      }

      const response = await apiClient.getDecision(id)
      if (!response.success) {
        throw new Error(response.error || 'Failed to fetch decision')
      }
      return response
    },
    enabled: !!id,
    staleTime: 60000,
  })
}

export function useSimilarDecisions(id: string) {
  return useQuery({
    queryKey: AGENT_QUERY_KEYS.similarDecisions(id),
    queryFn: async () => {
      if (USE_MOCK_DATA) {
        return {
          success: true as const,
          data: getMockSimilarDecisions(),
          timestamp: new Date().toISOString(),
        }
      }

      const response = await apiClient.getSimilarDecisions(id)
      if (!response.success) {
        throw new Error(response.error || 'Failed to fetch similar decisions')
      }
      return response
    },
    enabled: !!id,
    staleTime: 300000,
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
