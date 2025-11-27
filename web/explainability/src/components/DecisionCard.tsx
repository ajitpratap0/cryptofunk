import { formatDistanceToNow } from 'date-fns';
import { Clock, TrendingUp, TrendingDown, Activity } from 'lucide-react';
import type { Decision } from '../types';

export interface DecisionCardProps {
  decision: Decision;
  selected: boolean;
  onClick: () => void;
}

// Helper functions moved outside component to prevent re-creation on every render
const getConfidenceColor = (confidence: number): string => {
  if (confidence > 0.7) return 'bg-green-500';
  if (confidence > 0.5) return 'bg-yellow-500';
  return 'bg-red-500';
};

const getConfidenceTextColor = (confidence: number): string => {
  if (confidence > 0.7) return 'text-green-400';
  if (confidence > 0.5) return 'text-yellow-400';
  return 'text-red-400';
};

const getOutcomeBadge = (outcome?: string) => {
  switch (outcome) {
    case 'SUCCESS':
      return (
        <span className="px-2 py-1 bg-green-900/30 text-green-400 text-xs font-medium rounded border border-green-700">
          Success
        </span>
      );
    case 'FAILURE':
      return (
        <span className="px-2 py-1 bg-red-900/30 text-red-400 text-xs font-medium rounded border border-red-700">
          Failure
        </span>
      );
    default:
      return (
        <span className="px-2 py-1 bg-slate-700 text-slate-400 text-xs font-medium rounded border border-slate-600">
          Pending
        </span>
      );
  }
};

const getDecisionIcon = (decisionType?: string) => {
  const type = decisionType?.toUpperCase() || '';
  if (type.includes('BUY')) {
    return <TrendingUp className="w-5 h-5 text-green-400" />;
  }
  if (type.includes('SELL')) {
    return <TrendingDown className="w-5 h-5 text-red-400" />;
  }
  return <Activity className="w-5 h-5 text-blue-400" />;
};

function DecisionCard({ decision, selected, onClick }: DecisionCardProps) {
  const confidence = decision.confidence ?? 0;
  const confidencePercent = Math.round(confidence * 100);

  const relativeTime = formatDistanceToNow(new Date(decision.created_at), {
    addSuffix: true,
  });

  // Handle keyboard navigation
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      onClick();
    }
  };

  return (
    <article
      onClick={onClick}
      onKeyDown={handleKeyDown}
      tabIndex={0}
      role="button"
      aria-pressed={selected}
      aria-label={`${decision.agent_name || 'Unknown Agent'} decision for ${decision.symbol}: ${decision.decision_type}. Confidence ${confidencePercent}%. Outcome: ${decision.outcome || 'Pending'}`}
      className={`bg-slate-800 rounded-lg p-4 border transition-all cursor-pointer hover:shadow-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 focus:ring-offset-slate-900 ${
        selected
          ? 'border-blue-500 shadow-xl shadow-blue-500/20'
          : 'border-slate-700 hover:border-slate-600'
      }`}
    >
      <div className="flex items-start justify-between gap-4">
        {/* Left: Icon and Main Info */}
        <div className="flex items-start gap-3 flex-1">
          <div className="mt-1">{getDecisionIcon(decision.decision_type)}</div>
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 flex-wrap">
              <h3 className="font-semibold text-slate-100">
                {decision.agent_name || 'Unknown Agent'}
              </h3>
              <span className="text-slate-400">•</span>
              <span className="font-mono text-blue-400">{decision.symbol}</span>
            </div>
            <p className="text-slate-300 mt-1">{decision.decision_type}</p>

            {/* Response preview (truncated) */}
            {decision.response && (
              <p className="text-slate-400 text-sm mt-2 line-clamp-2">
                {decision.response.substring(0, 150)}
                {decision.response.length > 150 && '...'}
              </p>
            )}
          </div>
        </div>

        {/* Right: Outcome Badge */}
        <div className="flex-shrink-0">{getOutcomeBadge(decision.outcome)}</div>
      </div>

      {/* Confidence Bar */}
      <div className="mt-4">
        <div className="flex items-center justify-between text-sm mb-1">
          <span className="text-slate-400" id={`confidence-label-${decision.id}`}>
            Confidence
          </span>
          <span className={`font-semibold ${getConfidenceTextColor(confidence)}`}>
            {confidencePercent}%
          </span>
        </div>
        <div
          className="w-full h-2 bg-slate-700 rounded-full overflow-hidden"
          role="progressbar"
          aria-valuenow={confidencePercent}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-labelledby={`confidence-label-${decision.id}`}
        >
          <div
            className={`h-full ${getConfidenceColor(confidence)} transition-all duration-300`}
            style={{ width: `${confidencePercent}%` }}
          />
        </div>
      </div>

      {/* Footer: Timestamp */}
      <div className="flex items-center gap-2 mt-3 text-slate-500 text-xs">
        <Clock className="w-3 h-3" aria-hidden="true" />
        <time dateTime={decision.created_at}>{relativeTime}</time>
      </div>
    </article>
  );
}

export default DecisionCard;
