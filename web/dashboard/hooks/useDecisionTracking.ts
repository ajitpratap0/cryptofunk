'use client'

import { useQuery } from '@tanstack/react-query'
import { fetchDecisionAnalytics, fetchDecisionOutcomes } from '@/lib/api'
import type { ApiResponse, DecisionAnalytics, DecisionWithOutcome } from '@/lib/types'

export const DECISION_QUERY_KEYS = {
  analytics: (since: string) => ['decisions', 'analytics', since],
  outcomes: (limit: number) => ['decisions', 'outcomes', limit],
}

export function useDecisionAnalytics(since = '30d') {
  return useQuery<ApiResponse<DecisionAnalytics>>({
    queryKey: DECISION_QUERY_KEYS.analytics(since),
    queryFn: async () => {
      try {
        const result = await fetchDecisionAnalytics(since)
        if (result && result.success !== false) return result
      } catch {
        // fall through to mock
      }
      // Return empty analytics as fallback
      return {
        success: true,
        data: {
          total_decisions: 0,
          resolved_decisions: 0,
          overall_win_rate: 0,
          overall_avg_pnl: 0,
          by_agent: [],
          confidence_calibration: [],
        },
        timestamp: new Date().toISOString(),
      }
    },
    refetchInterval: 60000,
  })
}

export function useDecisionOutcomes(limit = 50) {
  return useQuery<ApiResponse<DecisionWithOutcome[]>>({
    queryKey: DECISION_QUERY_KEYS.outcomes(limit),
    queryFn: async () => {
      try {
        const result = await fetchDecisionOutcomes(limit)
        if (result && result.success !== false) {
          // Ensure data is an array
          const data = result.data
          return {
            ...result,
            data: Array.isArray(data) ? data : [],
          }
        }
      } catch {
        // fall through to mock
      }
      return {
        success: true,
        data: [],
        timestamp: new Date().toISOString(),
      }
    },
    refetchInterval: 60000,
  })
}
