/**
 * React Query hooks for LLM decision explainability
 * Provides type-safe hooks with caching and automatic refetching
 */

import { useQuery, useInfiniteQuery, useMutation, useQueryClient, UseQueryResult, UseInfiniteQueryResult } from '@tanstack/react-query';

/**
 * Cache configuration constants
 * Centralized settings for React Query caching behavior
 */
export const CACHE_CONFIG = {
  /** List queries: frequent updates expected */
  DECISIONS_LIST: {
    staleTime: 30_000,      // 30 seconds
    gcTime: 300_000,        // 5 minutes
  },
  /** Detail queries: single item, less frequent updates */
  DECISION_DETAIL: {
    staleTime: 60_000,      // 1 minute
    gcTime: 600_000,        // 10 minutes
  },
  /** Stats queries: aggregated data */
  DECISION_STATS: {
    staleTime: 60_000,      // 1 minute
    gcTime: 300_000,        // 5 minutes
  },
  /** Search queries: cached for repeated searches */
  DECISION_SEARCH: {
    staleTime: 120_000,     // 2 minutes
    gcTime: 300_000,        // 5 minutes
  },
  /** Similar decisions: expensive vector operations */
  DECISION_SIMILAR: {
    staleTime: 300_000,     // 5 minutes
    gcTime: 600_000,        // 10 minutes
  },
  /** Default page size for infinite queries */
  DEFAULT_PAGE_SIZE: 20,
} as const;
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
    staleTime: CACHE_CONFIG.DECISIONS_LIST.staleTime,
    gcTime: CACHE_CONFIG.DECISIONS_LIST.gcTime,
    ...options,
  });
}

/**
 * Hook to list decisions with infinite scrolling/pagination
 * Uses React Query's useInfiniteQuery for efficient data loading
 *
 * @param filter - Filtering options (excluding limit/offset which are managed internally)
 * @param options - React Query options
 * @returns Infinite query result with fetchNextPage and hasNextPage
 *
 * @example
 * const { data, fetchNextPage, hasNextPage, isFetchingNextPage } = useInfiniteDecisions({ symbol: 'BTCUSDT' });
 */
export function useInfiniteDecisions(
  filter: Omit<DecisionFilter, 'limit' | 'offset'> = {},
  options?: {
    enabled?: boolean;
    pageSize?: number;
  }
): UseInfiniteQueryResult<{ pages: ListDecisionsResponse[]; pageParams: number[] }, Error> {
  const pageSize = options?.pageSize ?? CACHE_CONFIG.DEFAULT_PAGE_SIZE;

  return useInfiniteQuery({
    queryKey: [...decisionKeys.lists(), 'infinite', filter],
    queryFn: async ({ pageParam = 0 }) => {
      return listDecisions({
        ...filter,
        limit: pageSize,
        offset: pageParam as number,
      });
    },
    initialPageParam: 0,
    getNextPageParam: (lastPage, allPages) => {
      // If we got fewer results than the page size, we've reached the end
      if (lastPage.decisions.length < pageSize) {
        return undefined;
      }
      // Calculate next offset based on total items fetched
      const totalFetched = allPages.reduce((sum, page) => sum + page.decisions.length, 0);
      return totalFetched;
    },
    staleTime: CACHE_CONFIG.DECISIONS_LIST.staleTime,
    gcTime: CACHE_CONFIG.DECISIONS_LIST.gcTime,
    enabled: options?.enabled ?? true,
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
    staleTime: CACHE_CONFIG.DECISION_DETAIL.staleTime,
    gcTime: CACHE_CONFIG.DECISION_DETAIL.gcTime,
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
    staleTime: CACHE_CONFIG.DECISION_STATS.staleTime,
    gcTime: CACHE_CONFIG.DECISION_STATS.gcTime,
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
    staleTime: CACHE_CONFIG.DECISION_SEARCH.staleTime,
    gcTime: CACHE_CONFIG.DECISION_SEARCH.gcTime,
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
    staleTime: CACHE_CONFIG.DECISION_SIMILAR.staleTime,
    gcTime: CACHE_CONFIG.DECISION_SIMILAR.gcTime,
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
  useInfiniteDecisions,
  useDecision,
  useDecisionStats,
  useSearchDecisions,
  useSearchDecisionsQuery,
  useSimilarDecisions,
  useInvalidateDecisions,
  decisionKeys,
  CACHE_CONFIG,
};
