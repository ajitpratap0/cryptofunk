import { useState, useCallback } from 'react';
import { Loader2, ChevronDown, RefreshCw } from 'lucide-react';
import { useInfiniteDecisions } from '../hooks/useDecisions';
import DecisionCard from './DecisionCard';
import { DecisionCardSkeletonList } from './DecisionCardSkeleton';
import type { Decision } from '../types';

interface DecisionTimelineProps {
  selectedDecision: Decision | null;
  onSelectDecision: (decision: Decision | null) => void;
}

function DecisionTimeline({ selectedDecision, onSelectDecision }: DecisionTimelineProps) {
  const [agentFilter, setAgentFilter] = useState<string>('');
  const [symbolFilter, setSymbolFilter] = useState<string>('');
  const [outcomeFilter, setOutcomeFilter] = useState<string>('');

  const {
    data,
    isLoading,
    error,
    refetch,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useInfiniteDecisions({
    agent_name: agentFilter || undefined,
    symbol: symbolFilter || undefined,
    outcome: outcomeFilter || undefined,
  });

  const handleLoadMore = useCallback(() => {
    if (hasNextPage && !isFetchingNextPage) {
      fetchNextPage();
    }
  }, [fetchNextPage, hasNextPage, isFetchingNextPage]);

  const handleClearFilters = useCallback(() => {
    setAgentFilter('');
    setSymbolFilter('');
    setOutcomeFilter('');
  }, []);

  // Flatten all pages into a single array of decisions
  const decisions = data?.pages.flatMap((page) => page.decisions) ?? [];
  const totalCount = decisions.length;

  if (error) {
    return (
      <div
        className="bg-red-900/20 border border-red-500 rounded-lg p-6 text-center"
        role="alert"
        aria-live="assertive"
      >
        <p className="text-red-400 font-medium">Error loading decisions</p>
        <p className="text-slate-400 text-sm mt-2">{error.message}</p>
        <button
          onClick={() => refetch()}
          className="mt-4 px-4 py-2 bg-red-600 hover:bg-red-700 rounded-lg transition-colors focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2 focus:ring-offset-slate-900"
          aria-label="Retry loading decisions"
        >
          Retry
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-6" role="region" aria-label="Decision Timeline">
      {/* Filters */}
      <div className="bg-slate-800 rounded-lg p-4 border border-slate-700">
        <fieldset>
          <legend className="sr-only">Filter decisions</legend>
          <div className="flex flex-wrap gap-4 items-end">
            <div className="flex-1 min-w-[200px]">
              <label
                htmlFor="agent-filter"
                className="block text-sm font-medium text-slate-300 mb-2"
              >
                Agent
              </label>
              <input
                id="agent-filter"
                type="text"
                placeholder="Filter by agent name..."
                value={agentFilter}
                onChange={(e) => setAgentFilter(e.target.value)}
                className="w-full px-3 py-2 bg-slate-900 border border-slate-600 rounded-lg text-slate-100 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
                aria-describedby="agent-filter-description"
              />
              <span id="agent-filter-description" className="sr-only">
                Enter agent name to filter decisions
              </span>
            </div>
            <div className="flex-1 min-w-[200px]">
              <label
                htmlFor="symbol-filter"
                className="block text-sm font-medium text-slate-300 mb-2"
              >
                Symbol
              </label>
              <input
                id="symbol-filter"
                type="text"
                placeholder="Filter by symbol..."
                value={symbolFilter}
                onChange={(e) => setSymbolFilter(e.target.value)}
                className="w-full px-3 py-2 bg-slate-900 border border-slate-600 rounded-lg text-slate-100 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
                aria-describedby="symbol-filter-description"
              />
              <span id="symbol-filter-description" className="sr-only">
                Enter trading symbol to filter decisions
              </span>
            </div>
            <div className="flex-1 min-w-[200px]">
              <label
                htmlFor="outcome-filter"
                className="block text-sm font-medium text-slate-300 mb-2"
              >
                Outcome
              </label>
              <select
                id="outcome-filter"
                value={outcomeFilter}
                onChange={(e) => setOutcomeFilter(e.target.value)}
                className="w-full px-3 py-2 bg-slate-900 border border-slate-600 rounded-lg text-slate-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
                aria-describedby="outcome-filter-description"
              >
                <option value="">All</option>
                <option value="SUCCESS">Success</option>
                <option value="FAILURE">Failure</option>
                <option value="PENDING">Pending</option>
              </select>
              <span id="outcome-filter-description" className="sr-only">
                Select outcome status to filter decisions
              </span>
            </div>
            <div className="flex gap-2">
              <button
                onClick={handleClearFilters}
                className="px-4 py-2 bg-slate-700 hover:bg-slate-600 text-slate-200 rounded-lg transition-colors focus:outline-none focus:ring-2 focus:ring-slate-500 focus:ring-offset-2 focus:ring-offset-slate-900"
                aria-label="Clear all filters"
              >
                Clear Filters
              </button>
              <button
                onClick={() => refetch()}
                className="px-3 py-2 bg-slate-700 hover:bg-slate-600 text-slate-200 rounded-lg transition-colors focus:outline-none focus:ring-2 focus:ring-slate-500 focus:ring-offset-2 focus:ring-offset-slate-900"
                aria-label="Refresh decisions"
                title="Refresh"
              >
                <RefreshCw className="w-4 h-4" aria-hidden="true" />
              </button>
            </div>
          </div>
        </fieldset>
      </div>

      {/* Loading State - Show skeleton cards for better UX */}
      {isLoading && <DecisionCardSkeletonList count={5} />}

      {/* Decision List */}
      {!isLoading && (
        <>
          <div
            className="space-y-3"
            role="list"
            aria-label={`${totalCount} decisions`}
          >
            {decisions.length === 0 ? (
              <div
                className="bg-slate-800 rounded-lg p-8 text-center border border-slate-700"
                role="status"
              >
                <p className="text-slate-400">No decisions found</p>
                <p className="text-slate-500 text-sm mt-2">
                  Try adjusting your filters or check back later
                </p>
              </div>
            ) : (
              decisions.map((decision) => (
                <div key={decision.id} role="listitem">
                  <DecisionCard
                    decision={decision}
                    selected={selectedDecision?.id === decision.id}
                    onClick={() =>
                      onSelectDecision(
                        selectedDecision?.id === decision.id ? null : decision
                      )
                    }
                  />
                </div>
              ))
            )}
          </div>

          {/* Load More Button */}
          {decisions.length > 0 && hasNextPage && (
            <div className="flex justify-center pt-4">
              <button
                onClick={handleLoadMore}
                disabled={isFetchingNextPage}
                className="px-6 py-3 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-800 disabled:cursor-not-allowed text-white rounded-lg transition-colors flex items-center gap-2 font-medium focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 focus:ring-offset-slate-900"
                aria-label={isFetchingNextPage ? 'Loading more decisions' : 'Load more decisions'}
              >
                {isFetchingNextPage ? (
                  <>
                    <Loader2 className="w-4 h-4 animate-spin" aria-hidden="true" />
                    Loading...
                  </>
                ) : (
                  <>
                    Load More
                    <ChevronDown className="w-4 h-4" aria-hidden="true" />
                  </>
                )}
              </button>
            </div>
          )}

          {/* Results Count */}
          <div
            className="text-center text-slate-500 text-sm"
            role="status"
            aria-live="polite"
          >
            Showing {totalCount} decision{totalCount !== 1 ? 's' : ''}
            {!hasNextPage && totalCount > 0 && ' (all loaded)'}
          </div>
        </>
      )}
    </div>
  );
}

export default DecisionTimeline;
