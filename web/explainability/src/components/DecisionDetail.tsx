import React, { useState, useEffect, useRef } from 'react';
import { X, ChevronDown, ChevronUp, Search } from 'lucide-react';
import { useDecision } from '../hooks/useDecisions';
import LoadingSpinner from './LoadingSpinner';

interface DecisionDetailProps {
  decisionId: string | null;
  onClose: () => void;
  onFindSimilar?: (decisionId: string) => void;
}

const DecisionDetail: React.FC<DecisionDetailProps> = ({
  decisionId,
  onClose,
  onFindSimilar,
}) => {
  const { data: decision, isLoading, error } = useDecision(decisionId || '');
  const [promptExpanded, setPromptExpanded] = useState(false);
  const [responseExpanded, setResponseExpanded] = useState(false);
  const modalRef = useRef<HTMLDivElement>(null);
  const closeButtonRef = useRef<HTMLButtonElement>(null);

  // Handle escape key to close modal
  useEffect(() => {
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        onClose();
      }
    };

    document.addEventListener('keydown', handleEscape);
    return () => {
      document.removeEventListener('keydown', handleEscape);
    };
  }, [onClose]);

  // Focus trap implementation
  useEffect(() => {
    if (!modalRef.current || isLoading) return;

    const modal = modalRef.current;
    const focusableElements = modal.querySelectorAll<HTMLElement>(
      'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
    );
    const firstFocusable = focusableElements[0];
    const lastFocusable = focusableElements[focusableElements.length - 1];

    // Focus the close button when modal opens
    closeButtonRef.current?.focus();

    const handleTabKey = (event: KeyboardEvent) => {
      if (event.key !== 'Tab') return;

      if (event.shiftKey) {
        // Shift + Tab
        if (document.activeElement === firstFocusable) {
          event.preventDefault();
          lastFocusable?.focus();
        }
      } else {
        // Tab
        if (document.activeElement === lastFocusable) {
          event.preventDefault();
          firstFocusable?.focus();
        }
      }
    };

    modal.addEventListener('keydown', handleTabKey);
    return () => {
      modal.removeEventListener('keydown', handleTabKey);
    };
  }, [isLoading]);

  if (!decisionId) {
    return null;
  }

  if (isLoading) {
    return (
      <div
        className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
        role="dialog"
        aria-modal="true"
        aria-label="Loading decision details"
      >
        <div className="bg-slate-800 rounded-lg p-8 max-w-4xl w-full mx-4">
          <LoadingSpinner />
        </div>
      </div>
    );
  }

  if (error || !decision) {
    return (
      <div
        className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50"
        role="dialog"
        aria-modal="true"
        aria-label="Error loading decision"
      >
        <div className="bg-slate-800 rounded-lg p-8 max-w-4xl w-full mx-4">
          <div className="text-red-400" role="alert">
            Error loading decision: {error?.message || 'Not found'}
          </div>
          <button
            onClick={onClose}
            className="mt-4 px-4 py-2 bg-slate-700 hover:bg-slate-600 text-white rounded focus:outline-none focus:ring-2 focus:ring-blue-500"
            aria-label="Close modal"
          >
            Close
          </button>
        </div>
      </div>
    );
  }

  const getOutcomeColor = (outcome?: string) => {
    switch (outcome) {
      case 'SUCCESS':
        return 'text-green-400 bg-green-900/30';
      case 'FAILURE':
        return 'text-red-400 bg-red-900/30';
      case 'PENDING':
        return 'text-yellow-400 bg-yellow-900/30';
      default:
        return 'text-slate-400 bg-slate-700';
    }
  };

  const formatTimestamp = (timestamp: string) => {
    return new Date(timestamp).toLocaleString();
  };

  const formatPnL = (pnl?: number) => {
    if (pnl === undefined || pnl === null) return 'N/A';
    const sign = pnl >= 0 ? '+' : '';
    return `${sign}$${pnl.toFixed(2)}`;
  };

  const getPnLColor = (pnl?: number) => {
    if (pnl === undefined || pnl === null) return 'text-slate-400';
    return pnl >= 0 ? 'text-green-400' : 'text-red-400';
  };

  return (
    <div
      className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4 overflow-y-auto"
      role="dialog"
      aria-modal="true"
      aria-labelledby="decision-detail-title"
    >
      <div ref={modalRef} className="bg-slate-800 rounded-lg shadow-xl max-w-4xl w-full my-8">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-slate-700">
          <h2 id="decision-detail-title" className="text-2xl font-bold text-white">
            Decision Details
          </h2>
          <button
            ref={closeButtonRef}
            onClick={onClose}
            className="p-2 hover:bg-slate-700 rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500"
            aria-label="Close modal"
          >
            <X className="w-5 h-5 text-slate-400" aria-hidden="true" />
          </button>
        </div>

        {/* Content */}
        <div className="p-6 space-y-6 max-h-[calc(100vh-200px)] overflow-y-auto">
          {/* Main Info Section */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="text-sm font-medium text-slate-500">Agent</label>
              <p className="text-lg font-semibold text-white">
                {decision.agent_name || 'Unknown'}
              </p>
            </div>
            <div>
              <label className="text-sm font-medium text-slate-500">Symbol</label>
              <p className="text-lg font-semibold text-white">
                {decision.symbol || 'N/A'}
              </p>
            </div>
            <div>
              <label className="text-sm font-medium text-slate-500">
                Decision Type
              </label>
              <p className="text-lg font-semibold text-white">
                {decision.decision_type}
              </p>
            </div>
            <div>
              <label className="text-sm font-medium text-slate-500">Timestamp</label>
              <p className="text-sm text-slate-300">
                {formatTimestamp(decision.created_at)}
              </p>
            </div>
          </div>

          {/* Metrics Section */}
          <div className="grid grid-cols-2 md:grid-cols-3 gap-4 p-4 bg-slate-900 rounded-lg">
            <div>
              <label className="text-sm font-medium text-slate-500">
                Confidence
              </label>
              <p className="text-xl font-bold text-blue-400">
                {((decision.confidence ?? 0) * 100).toFixed(1)}%
              </p>
            </div>
            <div>
              <label className="text-sm font-medium text-slate-500">Latency</label>
              <p className="text-xl font-bold text-white">
                {decision.latency_ms ?? 'N/A'}ms
              </p>
            </div>
            <div>
              <label className="text-sm font-medium text-slate-500">Tokens</label>
              <p className="text-xl font-bold text-white">
                {decision.tokens_used ?? 'N/A'}
              </p>
            </div>
            <div>
              <label className="text-sm font-medium text-slate-500">Outcome</label>
              <span
                className={`inline-block px-3 py-1 rounded-full text-sm font-semibold ${getOutcomeColor(
                  decision.outcome
                )}`}
              >
                {decision.outcome || 'PENDING'}
              </span>
            </div>
            <div>
              <label className="text-sm font-medium text-slate-500">P&L</label>
              <p className={`text-xl font-bold ${getPnLColor(decision.pnl)}`}>
                {formatPnL(decision.pnl)}
              </p>
            </div>
            <div>
              <label className="text-sm font-medium text-slate-500">Model</label>
              <p className="text-sm text-white">{decision.model}</p>
            </div>
          </div>

          {/* Prompt Section */}
          <div className="border border-slate-700 rounded-lg overflow-hidden">
            <button
              onClick={() => setPromptExpanded(!promptExpanded)}
              className="w-full flex items-center justify-between p-4 bg-slate-900 hover:bg-slate-800 transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-inset"
              aria-expanded={promptExpanded}
              aria-controls="prompt-content"
              aria-label={promptExpanded ? 'Collapse prompt' : 'Expand prompt'}
            >
              <span className="font-semibold text-white">Prompt</span>
              {promptExpanded ? (
                <ChevronUp className="w-5 h-5 text-slate-400" aria-hidden="true" />
              ) : (
                <ChevronDown className="w-5 h-5 text-slate-400" aria-hidden="true" />
              )}
            </button>
            {promptExpanded && (
              <div id="prompt-content" className="p-4 bg-slate-900/50">
                <pre className="text-sm text-slate-300 whitespace-pre-wrap font-mono bg-slate-900 p-4 rounded max-h-96 overflow-y-auto">
                  {decision.prompt}
                </pre>
              </div>
            )}
          </div>

          {/* Response Section */}
          <div className="border border-slate-700 rounded-lg overflow-hidden">
            <button
              onClick={() => setResponseExpanded(!responseExpanded)}
              className="w-full flex items-center justify-between p-4 bg-slate-900 hover:bg-slate-800 transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-inset"
              aria-expanded={responseExpanded}
              aria-controls="response-content"
              aria-label={responseExpanded ? 'Collapse response' : 'Expand response'}
            >
              <span className="font-semibold text-white">Response</span>
              {responseExpanded ? (
                <ChevronUp className="w-5 h-5 text-slate-400" aria-hidden="true" />
              ) : (
                <ChevronDown className="w-5 h-5 text-slate-400" aria-hidden="true" />
              )}
            </button>
            {responseExpanded && (
              <div id="response-content" className="p-4 bg-slate-900/50">
                <pre className="text-sm text-slate-300 whitespace-pre-wrap font-mono bg-slate-900 p-4 rounded max-h-96 overflow-y-auto">
                  {decision.response}
                </pre>
              </div>
            )}
          </div>
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between p-6 border-t border-slate-700 bg-slate-900">
          <button
            onClick={() => onFindSimilar && onFindSimilar(decision.id)}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500"
            aria-label="Find decisions similar to this one"
          >
            <Search className="w-4 h-4" aria-hidden="true" />
            Find Similar
          </button>
          <button
            onClick={onClose}
            className="px-6 py-2 bg-slate-700 hover:bg-slate-600 text-white rounded-lg transition-colors focus:outline-none focus:ring-2 focus:ring-slate-500"
            aria-label="Close modal"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
};

export default DecisionDetail;
