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
    queryFn: () => fetchDecisionAnalytics(since),
    refetchInterval: 60000,
  })
}

export function useDecisionOutcomes(limit = 50) {
  return useQuery<ApiResponse<DecisionWithOutcome[]>>({
    queryKey: DECISION_QUERY_KEYS.outcomes(limit),
    queryFn: () => fetchDecisionOutcomes(limit),
    refetchInterval: 60000,
  })
}
