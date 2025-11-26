import { formatDistanceToNow } from 'date-fns';
import { Clock, TrendingUp, TrendingDown, Activity } from 'lucide-react';
import type { Decision } from '../types';

export interface DecisionCardProps {
  decision: Decision;
  selected: boolean;
  onClick: () => void;
}

function DecisionCard({ decision, selected, onClick }: DecisionCardProps) {
  const confidence = decision.confidence ?? 0;
  const confidencePercent = Math.round(confidence * 100);

  // Determine confidence color
  const getConfidenceColor = () => {
    if (confidence > 0.7) return 'bg-green-500';
    if (confidence > 0.5) return 'bg-yellow-500';
    return 'bg-red-500';
  };

  const getConfidenceTextColor = () => {
    if (confidence > 0.7) return 'text-green-400';
    if (confidence > 0.5) return 'text-yellow-400';
    return 'text-red-400';
  };

  // Determine outcome badge
  const getOutcomeBadge = () => {
    switch (decision.outcome) {
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

  // Determine decision type icon
  const getDecisionIcon = () => {
    const type = decision.decision_type?.toUpperCase() || '';
    if (type.includes('BUY')) {
      return <TrendingUp className="w-5 h-5 text-green-400" />;
    }
    if (type.includes('SELL')) {
      return <TrendingDown className="w-5 h-5 text-red-400" />;
    }
    return <Activity className="w-5 h-5 text-blue-400" />;
  };

  const relativeTime = formatDistanceToNow(new Date(decision.created_at), {
    addSuffix: true,
  });

  return (
    <div
      onClick={onClick}
      className={`bg-slate-800 rounded-lg p-4 border transition-all cursor-pointer hover:shadow-lg ${
        selected
          ? 'border-blue-500 shadow-xl shadow-blue-500/20'
          : 'border-slate-700 hover:border-slate-600'
      }`}
    >
      <div className="flex items-start justify-between gap-4">
        {/* Left: Icon and Main Info */}
        <div className="flex items-start gap-3 flex-1">
          <div className="mt-1">{getDecisionIcon()}</div>
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
        <div className="flex-shrink-0">{getOutcomeBadge()}</div>
      </div>

      {/* Confidence Bar */}
      <div className="mt-4">
        <div className="flex items-center justify-between text-sm mb-1">
          <span className="text-slate-400">Confidence</span>
          <span className={`font-semibold ${getConfidenceTextColor()}`}>
            {confidencePercent}%
          </span>
        </div>
        <div className="w-full h-2 bg-slate-700 rounded-full overflow-hidden">
          <div
            className={`h-full ${getConfidenceColor()} transition-all duration-300`}
            style={{ width: `${confidencePercent}%` }}
          />
        </div>
      </div>

      {/* Footer: Timestamp */}
      <div className="flex items-center gap-2 mt-3 text-slate-500 text-xs">
        <Clock className="w-3 h-3" />
        <span>{relativeTime}</span>
      </div>
    </div>
  );
}

export default DecisionCard;
