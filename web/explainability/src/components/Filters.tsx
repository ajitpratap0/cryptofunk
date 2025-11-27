import React from 'react';
import { X } from 'lucide-react';
import type { DecisionFilter } from '../types';

interface FiltersProps {
  filter: DecisionFilter;
  onChange: (filter: DecisionFilter) => void;
}

const Filters: React.FC<FiltersProps> = ({ filter, onChange }) => {
  const handleChange = (key: keyof DecisionFilter, value: string) => {
    onChange({
      ...filter,
      [key]: value || undefined,
    });
  };

  const handleClear = () => {
    onChange({});
  };

  const hasActiveFilters = Object.values(filter).some(
    (value) => value !== undefined && value !== ''
  );

  return (
    <div className="bg-slate-800 p-4 rounded-lg">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-lg font-semibold text-white">Filters</h3>
        {hasActiveFilters && (
          <button
            onClick={handleClear}
            className="flex items-center gap-1 text-sm text-slate-400 hover:text-white transition-colors"
          >
            <X className="w-4 h-4" />
            Clear
          </button>
        )}
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-4">
        {/* Symbol Filter */}
        <div>
          <label
            htmlFor="symbol"
            className="block text-sm font-medium text-slate-400 mb-1"
          >
            Symbol
          </label>
          <input
            id="symbol"
            type="text"
            value={filter.symbol || ''}
            onChange={(e) => handleChange('symbol', e.target.value)}
            placeholder="e.g., BTC/USDT"
            className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-md text-white placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>

        {/* Decision Type Filter */}
        <div>
          <label
            htmlFor="decision_type"
            className="block text-sm font-medium text-slate-400 mb-1"
          >
            Decision Type
          </label>
          <select
            id="decision_type"
            value={filter.decision_type || ''}
            onChange={(e) => handleChange('decision_type', e.target.value)}
            className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            <option value="">All Types</option>
            <option value="signal">Signal</option>
            <option value="risk_approval">Risk Approval</option>
            <option value="position_sizing">Position Sizing</option>
          </select>
        </div>

        {/* Outcome Filter */}
        <div>
          <label
            htmlFor="outcome"
            className="block text-sm font-medium text-slate-400 mb-1"
          >
            Outcome
          </label>
          <select
            id="outcome"
            value={filter.outcome || ''}
            onChange={(e) => handleChange('outcome', e.target.value)}
            className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            <option value="">All Outcomes</option>
            <option value="SUCCESS">Success</option>
            <option value="FAILURE">Failure</option>
            <option value="PENDING">Pending</option>
          </select>
        </div>

        {/* From Date Filter */}
        <div>
          <label
            htmlFor="from_date"
            className="block text-sm font-medium text-slate-400 mb-1"
          >
            From Date
          </label>
          <input
            id="from_date"
            type="datetime-local"
            value={filter.from_date || ''}
            onChange={(e) => handleChange('from_date', e.target.value)}
            className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>

        {/* To Date Filter */}
        <div>
          <label
            htmlFor="to_date"
            className="block text-sm font-medium text-slate-400 mb-1"
          >
            To Date
          </label>
          <input
            id="to_date"
            type="datetime-local"
            value={filter.to_date || ''}
            onChange={(e) => handleChange('to_date', e.target.value)}
            className="w-full px-3 py-2 bg-slate-700 border border-slate-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>
      </div>
    </div>
  );
};

export default Filters;
