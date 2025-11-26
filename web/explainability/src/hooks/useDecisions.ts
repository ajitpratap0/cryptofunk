/**
 * React Query hooks for LLM decision explainability
 * Provides type-safe hooks with caching and automatic refetching
 */

import { useQuery, useMutation, useQueryClient, UseQueryResult } from '@tanstack/react-query';
import type {
  Decision,
  DecisionFilter,
  DecisionStats,
  ListDecisionsResponse,
  SearchDecisionsResponse,
  SimilarDecisionsResponse,
} from '../types/decision';
import {
  listDecisions,
  getDecision,
  getDecisionStats,
  searchDecisions,
  getSimilarDecisions,
} from '../api/decisions';

/**
 * Query key factory for decision-related queries
 */
export const decisionKeys = {
  all: ['decisions'] as const,
  lists: () => [...decisionKeys.all, 'list'] as const,
  list: (filter: DecisionFilter) => [...decisionKeys.lists(), filter] as const,
  details: () => [...decisionKeys.all, 'detail'] as const,
  detail: (id: string) => [...decisionKeys.details(), id] as const,
  stats: () => [...decisionKeys.all, 'stats'] as const,
  stat: (filter?: DecisionFilter) => [...decisionKeys.stats(), filter ?? {}] as const,
  searches: () => [...decisionKeys.all, 'search'] as const,
  search: (query: string, limit?: number) => [...decisionKeys.searches(), query, limit] as const,
  similar: (id: string, limit?: number) => [...decisionKeys.all, 'similar', id, limit] as const,
};

/**
 * Hook to list decisions with optional filtering
 *
 * @param filter - Filtering and pagination options
 * @param options - React Query options
 * @returns Query result with decisions data
 *
 * @example
 * const { data, isLoading, error } = useDecisions({ symbol: 'BTCUSDT', limit: 10 });
 */
export function useDecisions(
  filter: DecisionFilter = {},
  options?: {
    enabled?: boolean;
    refetchInterval?: number;
  }
): UseQueryResult<ListDecisionsResponse, Error> {
  return useQuery({
    queryKey: decisionKeys.list(filter),
    queryFn: () => listDecisions(filter),
    staleTime: 1000 * 30, // 30 seconds
    gcTime: 1000 * 60 * 5, // 5 minutes (formerly cacheTime)
    ...options,
  });
}

/**
 * Hook to get a single decision by ID
 *
 * @param id - Decision UUID
 * @param options - React Query options
 * @returns Query result with decision details
 *
 * @example
 * const { data, isLoading, error } = useDecision('123e4567-e89b-12d3-a456-426614174000');
 */
export function useDecision(
  id: string,
  options?: {
    enabled?: boolean;
  }
): UseQueryResult<Decision, Error> {
  return useQuery({
    queryKey: decisionKeys.detail(id),
    queryFn: () => getDecision(id),
    enabled: !!id && (options?.enabled ?? true),
    staleTime: 1000 * 60, // 1 minute
    gcTime: 1000 * 60 * 10, // 10 minutes
  });
}

/**
 * Hook to get aggregated decision statistics
 *
 * @param filter - Optional filtering options
 * @param options - React Query options
 * @returns Query result with decision statistics
 *
 * @example
 * const { data, isLoading, error } = useDecisionStats({ symbol: 'BTCUSDT' });
 */
export function useDecisionStats(
  filter?: DecisionFilter,
  options?: {
    enabled?: boolean;
    refetchInterval?: number;
  }
): UseQueryResult<DecisionStats, Error> {
  return useQuery({
    queryKey: decisionKeys.stat(filter),
    queryFn: () => getDecisionStats(filter),
    staleTime: 1000 * 60, // 1 minute
    gcTime: 1000 * 60 * 5, // 5 minutes
    ...options,
  });
}

/**
 * Hook to search decisions
 * Returns a mutation to allow manual triggering
 *
 * @returns Mutation result with search functionality
 *
 * @example
 * const searchMutation = useSearchDecisions();
 * searchMutation.mutate({ query: 'bullish signal', limit: 20 });
 */
export function useSearchDecisions() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ query, limit }: { query: string; limit?: number }) =>
      searchDecisions(query, limit),
    onSuccess: (data, variables) => {
      // Cache the search results
      queryClient.setQueryData(decisionKeys.search(variables.query, variables.limit), data);
    },
  });
}

/**
 * Hook to search decisions with automatic query execution
 * Useful when you want the search to execute immediately
 *
 * @param query - Search query string
 * @param limit - Maximum number of results
 * @param options - React Query options
 * @returns Query result with search results
 *
 * @example
 * const { data, isLoading, error } = useSearchDecisionsQuery('bullish signal', 20);
 */
export function useSearchDecisionsQuery(
  query: string,
  limit?: number,
  options?: {
    enabled?: boolean;
  }
): UseQueryResult<SearchDecisionsResponse, Error> {
  return useQuery({
    queryKey: decisionKeys.search(query, limit),
    queryFn: () => searchDecisions(query, limit),
    enabled: !!query && (options?.enabled ?? true),
    staleTime: 1000 * 60 * 2, // 2 minutes
    gcTime: 1000 * 60 * 5, // 5 minutes
  });
}

/**
 * Hook to get similar decisions using vector similarity
 *
 * @param id - Decision UUID to find similar decisions for
 * @param limit - Maximum number of results
 * @param options - React Query options
 * @returns Query result with similar decisions
 *
 * @example
 * const { data, isLoading, error } = useSimilarDecisions('123e4567-e89b-12d3-a456-426614174000', 10);
 */
export function useSimilarDecisions(
  id: string,
  limit?: number,
  options?: {
    enabled?: boolean;
  }
): UseQueryResult<SimilarDecisionsResponse, Error> {
  return useQuery({
    queryKey: decisionKeys.similar(id, limit),
    queryFn: () => getSimilarDecisions(id, limit),
    enabled: !!id && (options?.enabled ?? true),
    staleTime: 1000 * 60 * 5, // 5 minutes
    gcTime: 1000 * 60 * 10, // 10 minutes
  });
}

/**
 * Hook to manually invalidate decision queries
 * Useful for forcing a refetch after mutations
 *
 * @returns Function to invalidate queries
 *
 * @example
 * const invalidateDecisions = useInvalidateDecisions();
 * invalidateDecisions(); // Invalidate all decision queries
 * invalidateDecisions('list'); // Invalidate only list queries
 */
export function useInvalidateDecisions() {
  const queryClient = useQueryClient();

  return (type?: 'all' | 'list' | 'detail' | 'stats' | 'search' | 'similar') => {
    switch (type) {
      case 'list':
        return queryClient.invalidateQueries({ queryKey: decisionKeys.lists() });
      case 'detail':
        return queryClient.invalidateQueries({ queryKey: decisionKeys.details() });
      case 'stats':
        return queryClient.invalidateQueries({ queryKey: decisionKeys.stats() });
      case 'search':
        return queryClient.invalidateQueries({ queryKey: decisionKeys.searches() });
      case 'similar':
        return queryClient.invalidateQueries({
          queryKey: [...decisionKeys.all, 'similar'],
        });
      default:
        return queryClient.invalidateQueries({ queryKey: decisionKeys.all });
    }
  };
}

/**
 * Default export with all hooks
 */
export default {
  useDecisions,
  useDecision,
  useDecisionStats,
  useSearchDecisions,
  useSearchDecisionsQuery,
  useSimilarDecisions,
  useInvalidateDecisions,
  decisionKeys,
};
