import React from 'react';
import { RefreshCw, TrendingUp, TrendingDown, Activity } from 'lucide-react';
import { useDecisionStats } from '../hooks/useDecisions';
import { SkeletonStatsView } from './Skeleton';

// Helper functions moved outside component to prevent re-creation on every render
const getPnLColor = (pnl: number): string => {
  return pnl >= 0 ? 'text-green-400' : 'text-red-400';
};

const formatPnL = (pnl: number): string => {
  const sign = pnl >= 0 ? '+' : '';
  return `${sign}$${pnl.toFixed(2)}`;
};

const StatsView: React.FC = () => {
  const { data: stats, isLoading, error, refetch } = useDecisionStats();

  if (isLoading) {
    return <SkeletonStatsView />;
  }

  if (error || !stats) {
    return (
      <div className="bg-slate-800 p-8 rounded-lg">
        <div className="text-red-400">
          Error loading statistics: {error?.message || 'Unknown error'}
        </div>
      </div>
    );
  }

  // Extract outcome counts from by_outcome map
  const successCount = stats.by_outcome?.['SUCCESS'] || 0;
  const failureCount = stats.by_outcome?.['FAILURE'] || 0;
  const pendingCount = stats.by_outcome?.['PENDING'] || 0;

  const successRate = stats.success_rate ? (stats.success_rate * 100).toFixed(1) : '0.0';

  // Calculate chart heights for outcome distribution
  const total = stats.total_decisions || 1;
  const successHeight = (successCount / total) * 100;
  const failureHeight = (failureCount / total) * 100;
  const pendingHeight = (pendingCount / total) * 100;

  return (
    <div className="space-y-6">
      {/* Header with Refresh */}
      <div className="flex items-center justify-between">
        <h2 className="text-2xl font-bold text-white">Statistics</h2>
        <button
          onClick={() => refetch()}
          className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
        >
          <RefreshCw className="w-4 h-4" />
          Refresh
        </button>
      </div>

      {/* Key Metrics Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {/* Total Decisions */}
        <div className="bg-slate-800 p-6 rounded-lg border-l-4 border-blue-500">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-slate-400">Total Decisions</p>
              <p className="text-3xl font-bold text-white mt-2">
                {stats.total_decisions}
              </p>
            </div>
            <Activity className="w-12 h-12 text-blue-500 opacity-20" />
          </div>
        </div>

        {/* Success Rate */}
        <div className="bg-slate-800 p-6 rounded-lg border-l-4 border-green-500">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-slate-400">Success Rate</p>
              <p className="text-3xl font-bold text-green-400 mt-2">
                {successRate}%
              </p>
            </div>
            <TrendingUp className="w-12 h-12 text-green-500 opacity-20" />
          </div>
        </div>

        {/* Average Confidence */}
        <div className="bg-slate-800 p-6 rounded-lg border-l-4 border-purple-500">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-slate-400">Avg Confidence</p>
              <p className="text-3xl font-bold text-purple-400 mt-2">
                {((stats.avg_confidence || 0) * 100).toFixed(1)}%
              </p>
            </div>
            <Activity className="w-12 h-12 text-purple-500 opacity-20" />
          </div>
        </div>

        {/* Total P&L */}
        <div className="bg-slate-800 p-6 rounded-lg border-l-4 border-yellow-500">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm font-medium text-slate-400">Total P&L</p>
              <p className={`text-3xl font-bold mt-2 ${getPnLColor(stats.total_pnl || 0)}`}>
                {formatPnL(stats.total_pnl || 0)}
              </p>
            </div>
            {(stats.total_pnl || 0) >= 0 ? (
              <TrendingUp className="w-12 h-12 text-green-500 opacity-20" />
            ) : (
              <TrendingDown className="w-12 h-12 text-red-500 opacity-20" />
            )}
          </div>
        </div>
      </div>

      {/* Outcome Distribution Chart */}
      <div className="bg-slate-800 p-6 rounded-lg">
        <h3 className="text-lg font-semibold text-white mb-4">
          Outcome Distribution
        </h3>
        <div className="flex items-end justify-around h-64 gap-4">
          {/* Success Bar */}
          <div className="flex-1 flex flex-col items-center">
            <div className="w-full flex items-end justify-center h-48">
              <div
                className="w-full max-w-24 bg-green-500 rounded-t-lg transition-all duration-500"
                style={{ height: `${successHeight}%`, minHeight: successCount > 0 ? '4px' : '0' }}
              />
            </div>
            <div className="mt-2 text-center">
              <p className="text-sm font-medium text-slate-400">Success</p>
              <p className="text-2xl font-bold text-green-400">
                {successCount}
              </p>
            </div>
          </div>

          {/* Failure Bar */}
          <div className="flex-1 flex flex-col items-center">
            <div className="w-full flex items-end justify-center h-48">
              <div
                className="w-full max-w-24 bg-red-500 rounded-t-lg transition-all duration-500"
                style={{ height: `${failureHeight}%`, minHeight: failureCount > 0 ? '4px' : '0' }}
              />
            </div>
            <div className="mt-2 text-center">
              <p className="text-sm font-medium text-slate-400">Failure</p>
              <p className="text-2xl font-bold text-red-400">
                {failureCount}
              </p>
            </div>
          </div>

          {/* Pending Bar */}
          <div className="flex-1 flex flex-col items-center">
            <div className="w-full flex items-end justify-center h-48">
              <div
                className="w-full max-w-24 bg-yellow-500 rounded-t-lg transition-all duration-500"
                style={{ height: `${pendingHeight}%`, minHeight: pendingCount > 0 ? '4px' : '0' }}
              />
            </div>
            <div className="mt-2 text-center">
              <p className="text-sm font-medium text-slate-400">Pending</p>
              <p className="text-2xl font-bold text-yellow-400">
                {pendingCount}
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Additional Metrics */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="bg-slate-800 p-4 rounded-lg">
          <p className="text-sm font-medium text-slate-400">Avg Latency</p>
          <p className="text-2xl font-bold text-white mt-1">
            {(stats.avg_latency_ms || 0).toFixed(0)}ms
          </p>
        </div>
        <div className="bg-slate-800 p-4 rounded-lg">
          <p className="text-sm font-medium text-slate-400">Avg Tokens</p>
          <p className="text-2xl font-bold text-white mt-1">
            {(stats.avg_tokens_used || 0).toFixed(0)}
          </p>
        </div>
        <div className="bg-slate-800 p-4 rounded-lg">
          <p className="text-sm font-medium text-slate-400">Avg P&L per Decision</p>
          <p className={`text-2xl font-bold mt-1 ${getPnLColor(stats.avg_pnl || 0)}`}>
            {formatPnL(stats.avg_pnl || 0)}
          </p>
        </div>
      </div>
    </div>
  );
};

export default StatsView;
