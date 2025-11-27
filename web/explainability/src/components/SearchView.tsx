import React, { useState } from 'react';
import { Search } from 'lucide-react';
import { useSearchDecisions } from '../hooks/useDecisions';
import DecisionCard from './DecisionCard';
import LoadingSpinner from './LoadingSpinner';
import type { Decision } from '../types';

interface SearchViewProps {
  onSelectDecision: (decision: Decision | null) => void;
}

const SearchView: React.FC<SearchViewProps> = ({ onSelectDecision }) => {
  const [query, setQuery] = useState('');
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const { mutate: search, data, isPending, error } = useSearchDecisions();

  const handleSearch = () => {
    if (query.trim()) {
      search({ query: query.trim(), limit: 20 });
    }
  };

  const handleKeyPress = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter') {
      handleSearch();
    }
  };

  const handleSelectDecision = (decision: Decision) => {
    setSelectedId(decision.id);
    onSelectDecision(selectedId === decision.id ? null : decision);
  };

  const results = data?.results || [];

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h2 className="text-2xl font-bold text-white mb-2">
          Semantic Search
        </h2>
        <p className="text-slate-400">
          Search for decisions using natural language. The system uses vector
          embeddings to find semantically similar decisions.
        </p>
      </div>

      {/* Search Input */}
      <div className="bg-slate-800 p-6 rounded-lg">
        <div className="flex gap-4">
          <input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyPress={handleKeyPress}
            placeholder="Why did you buy BTC?"
            className="flex-1 px-4 py-3 bg-slate-700 border border-slate-600 rounded-lg text-white placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500 text-lg"
          />
          <button
            onClick={handleSearch}
            disabled={isPending || !query.trim()}
            className="flex items-center gap-2 px-6 py-3 bg-blue-600 hover:bg-blue-700 disabled:bg-slate-600 text-white rounded-lg transition-colors"
          >
            <Search className="w-5 h-5" />
            Search
          </button>
        </div>
      </div>

      {/* Loading State */}
      {isPending && (
        <div className="bg-slate-800 p-8 rounded-lg">
          <LoadingSpinner />
          <p className="text-center text-slate-400 mt-4">
            Searching for similar decisions...
          </p>
        </div>
      )}

      {/* Error State */}
      {error && (
        <div className="bg-red-900/20 border border-red-700 rounded-lg p-4">
          <p className="text-red-400">
            Error: {error.message || 'Failed to search decisions'}
          </p>
        </div>
      )}

      {/* Results */}
      {results.length > 0 && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-lg font-semibold text-white">
              Found {results.length} similar decision{results.length !== 1 ? 's' : ''}
            </h3>
          </div>
          <div className="space-y-4">
            {results.map((result) => (
              <div key={result.decision.id} className="relative">
                <DecisionCard
                  decision={result.decision}
                  selected={selectedId === result.decision.id}
                  onClick={() => handleSelectDecision(result.decision)}
                />
                {/* Relevance Score Badge */}
                {result.score !== undefined && (
                  <div className="absolute top-4 right-4 bg-blue-600 text-white px-3 py-1 rounded-full text-sm font-semibold shadow-lg">
                    {(result.score * 100).toFixed(1)}% match
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* No Results */}
      {data && results.length === 0 && (
        <div className="bg-slate-800 border border-slate-700 rounded-lg p-8 text-center">
          <Search className="w-16 h-16 text-slate-600 mx-auto mb-4" />
          <p className="text-slate-400 text-lg">
            No similar decisions found for your query.
          </p>
          <p className="text-slate-500 mt-2">
            Try rephrasing your search or using different keywords.
          </p>
        </div>
      )}

      {/* Empty State */}
      {!data && !isPending && !error && (
        <div className="bg-slate-800 border border-slate-700 rounded-lg p-8 text-center">
          <Search className="w-16 h-16 text-slate-600 mx-auto mb-4" />
          <p className="text-slate-400 text-lg">
            Enter a search query to find similar decisions.
          </p>
          <div className="mt-4 text-left max-w-md mx-auto">
            <p className="text-sm font-semibold text-slate-300 mb-2">
              Example queries:
            </p>
            <ul className="space-y-1 text-sm text-slate-400">
              <li>- Why did you buy BTC?</li>
              <li>- Show me failed risk approvals</li>
              <li>- What position sizing decisions were made for ETH?</li>
              <li>- Find signals with high confidence</li>
            </ul>
          </div>
        </div>
      )}
    </div>
  );
};

export default SearchView;
