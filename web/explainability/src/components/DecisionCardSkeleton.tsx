/**
 * Skeleton loading component for DecisionCard
 * Provides visual feedback while decisions are being fetched
 */
function DecisionCardSkeleton() {
  return (
    <div
      className="bg-slate-800 rounded-lg p-4 border border-slate-700 animate-pulse"
      aria-hidden="true"
      role="presentation"
    >
      <div className="flex items-start justify-between gap-4">
        {/* Left: Icon and Main Info skeleton */}
        <div className="flex items-start gap-3 flex-1">
          {/* Icon placeholder */}
          <div className="mt-1 w-5 h-5 bg-slate-700 rounded" />

          <div className="flex-1 min-w-0 space-y-2">
            {/* Title row */}
            <div className="flex items-center gap-2">
              <div className="h-5 w-32 bg-slate-700 rounded" />
              <div className="h-4 w-4 bg-slate-700 rounded-full" />
              <div className="h-5 w-20 bg-slate-700 rounded" />
            </div>

            {/* Decision type */}
            <div className="h-4 w-24 bg-slate-700 rounded" />

            {/* Response preview */}
            <div className="space-y-1.5 mt-2">
              <div className="h-3 w-full bg-slate-700 rounded" />
              <div className="h-3 w-3/4 bg-slate-700 rounded" />
            </div>
          </div>
        </div>

        {/* Right: Outcome Badge placeholder */}
        <div className="flex-shrink-0">
          <div className="h-6 w-16 bg-slate-700 rounded" />
        </div>
      </div>

      {/* Confidence Bar skeleton */}
      <div className="mt-4">
        <div className="flex items-center justify-between mb-1">
          <div className="h-3 w-20 bg-slate-700 rounded" />
          <div className="h-4 w-10 bg-slate-700 rounded" />
        </div>
        <div className="w-full h-2 bg-slate-700 rounded-full" />
      </div>

      {/* Footer: Timestamp skeleton */}
      <div className="flex items-center gap-2 mt-3">
        <div className="w-3 h-3 bg-slate-700 rounded" />
        <div className="h-3 w-24 bg-slate-700 rounded" />
      </div>
    </div>
  );
}

export interface DecisionCardSkeletonListProps {
  count?: number;
}

/**
 * Renders multiple skeleton cards for loading state
 */
export function DecisionCardSkeletonList({ count = 5 }: DecisionCardSkeletonListProps) {
  return (
    <div className="space-y-3" aria-label="Loading decisions" role="status">
      <span className="sr-only">Loading decisions...</span>
      {Array.from({ length: count }, (_, index) => (
        <DecisionCardSkeleton key={index} />
      ))}
    </div>
  );
}

export default DecisionCardSkeleton;
