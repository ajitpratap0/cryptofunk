import { useState } from 'react';
import { Loader2, ChevronDown } from 'lucide-react';
import { useDecisions } from '../hooks/useDecisions';
import DecisionCard from './DecisionCard';
import type { Decision } from '../types';

interface DecisionTimelineProps {
  selectedDecision: Decision | null;
  onSelectDecision: (decision: Decision | null) => void;
}

function DecisionTimeline({ selectedDecision, onSelectDecision }: DecisionTimelineProps) {
  const [limit, setLimit] = useState(20);
  const [agentFilter, setAgentFilter] = useState<string>('');
  const [symbolFilter, setSymbolFilter] = useState<string>('');
  const [outcomeFilter, setOutcomeFilter] = useState<string>('');

  const { data, isLoading, error, refetch } = useDecisions({
    limit,
    agent_name: agentFilter || undefined,
    symbol: symbolFilter || undefined,
    outcome: outcomeFilter || undefined,
  });

  const handleLoadMore = () => {
    setLimit((prev) => prev + 20);
  };

  const handleClearFilters = () => {
    setAgentFilter('');
    setSymbolFilter('');
    setOutcomeFilter('');
  };

  if (error) {
    return (
      <div className="bg-red-900/20 border border-red-500 rounded-lg p-6 text-center">
        <p className="text-red-400 font-medium">Error loading decisions</p>
        <p className="text-slate-400 text-sm mt-2">{error.message}</p>
        <button
          onClick={() => refetch()}
          className="mt-4 px-4 py-2 bg-red-600 hover:bg-red-700 rounded-lg transition-colors"
        >
          Retry
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Filters */}
      <div className="bg-slate-800 rounded-lg p-4 border border-slate-700">
        <div className="flex flex-wrap gap-4 items-end">
          <div className="flex-1 min-w-[200px]">
            <label className="block text-sm font-medium text-slate-300 mb-2">
              Agent
            </label>
            <input
              type="text"
              placeholder="Filter by agent name..."
              value={agentFilter}
              onChange={(e) => setAgentFilter(e.target.value)}
              className="w-full px-3 py-2 bg-slate-900 border border-slate-600 rounded-lg text-slate-100 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
          <div className="flex-1 min-w-[200px]">
            <label className="block text-sm font-medium text-slate-300 mb-2">
              Symbol
            </label>
            <input
              type="text"
              placeholder="Filter by symbol..."
              value={symbolFilter}
              onChange={(e) => setSymbolFilter(e.target.value)}
              className="w-full px-3 py-2 bg-slate-900 border border-slate-600 rounded-lg text-slate-100 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>
          <div className="flex-1 min-w-[200px]">
            <label className="block text-sm font-medium text-slate-300 mb-2">
              Outcome
            </label>
            <select
              value={outcomeFilter}
              onChange={(e) => setOutcomeFilter(e.target.value)}
              className="w-full px-3 py-2 bg-slate-900 border border-slate-600 rounded-lg text-slate-100 focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option value="">All</option>
              <option value="SUCCESS">Success</option>
              <option value="FAILURE">Failure</option>
              <option value="PENDING">Pending</option>
            </select>
          </div>
          <button
            onClick={handleClearFilters}
            className="px-4 py-2 bg-slate-700 hover:bg-slate-600 text-slate-200 rounded-lg transition-colors"
          >
            Clear Filters
          </button>
        </div>
      </div>

      {/* Loading State */}
      {isLoading && (
        <div className="flex items-center justify-center py-12">
          <Loader2 className="w-8 h-8 text-blue-400 animate-spin" />
          <span className="ml-3 text-slate-400">Loading decisions...</span>
        </div>
      )}

      {/* Decision List */}
      {!isLoading && data && (
        <>
          <div className="space-y-3">
            {data.decisions.length === 0 ? (
              <div className="bg-slate-800 rounded-lg p-8 text-center border border-slate-700">
                <p className="text-slate-400">No decisions found</p>
                <p className="text-slate-500 text-sm mt-2">
                  Try adjusting your filters or check back later
                </p>
              </div>
            ) : (
              data.decisions.map((decision) => (
                <DecisionCard
                  key={decision.id}
                  decision={decision}
                  selected={selectedDecision?.id === decision.id}
                  onClick={() =>
                    onSelectDecision(
                      selectedDecision?.id === decision.id ? null : decision
                    )
                  }
                />
              ))
            )}
          </div>

          {/* Load More Button */}
          {data.decisions.length > 0 && data.decisions.length >= limit && (
            <div className="flex justify-center pt-4">
              <button
                onClick={handleLoadMore}
                className="px-6 py-3 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors flex items-center gap-2 font-medium"
              >
                Load More
                <ChevronDown className="w-4 h-4" />
              </button>
            </div>
          )}

          {/* Results Count */}
          <div className="text-center text-slate-500 text-sm">
            Showing {data.decisions.length} decisions
          </div>
        </>
      )}
    </div>
  );
}

export default DecisionTimeline;
