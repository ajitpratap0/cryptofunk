/**
 * Reusable skeleton loading components
 * Provides visual feedback during data loading to prevent layout shift
 */

interface SkeletonProps {
  className?: string;
}

/**
 * Base skeleton element with pulsing animation
 */
export function Skeleton({ className = '' }: SkeletonProps) {
  return (
    <div
      className={`bg-slate-700 rounded animate-pulse ${className}`}
      aria-hidden="true"
      role="presentation"
    />
  );
}

/**
 * Skeleton for text lines
 */
export function SkeletonText({ className = '', lines = 1 }: SkeletonProps & { lines?: number }) {
  return (
    <div className="space-y-2" aria-hidden="true" role="presentation">
      {Array.from({ length: lines }, (_, index) => (
        <Skeleton key={index} className={`h-4 ${className}`} />
      ))}
    </div>
  );
}

/**
 * Skeleton for metric cards
 */
export function SkeletonMetricCard() {
  return (
    <div
      className="bg-slate-800 p-6 rounded-lg border-l-4 border-slate-700"
      aria-hidden="true"
      role="presentation"
    >
      <div className="space-y-3">
        <Skeleton className="h-4 w-24" />
        <Skeleton className="h-8 w-20" />
      </div>
    </div>
  );
}

/**
 * Skeleton for chart bars
 */
export function SkeletonChartBar() {
  return (
    <div className="flex-1 flex flex-col items-center" aria-hidden="true" role="presentation">
      <div className="w-full flex items-end justify-center h-48">
        <Skeleton className="w-full max-w-24 h-32 rounded-t-lg" />
      </div>
      <div className="mt-2 text-center space-y-1">
        <Skeleton className="h-4 w-16 mx-auto" />
        <Skeleton className="h-6 w-12 mx-auto" />
      </div>
    </div>
  );
}

/**
 * Skeleton for the complete stats view
 */
export function SkeletonStatsView() {
  return (
    <div className="space-y-6" aria-label="Loading statistics" role="status">
      <span className="sr-only">Loading statistics...</span>

      {/* Header */}
      <div className="flex items-center justify-between">
        <Skeleton className="h-8 w-40" />
        <Skeleton className="h-10 w-28" />
      </div>

      {/* Key Metrics Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        <SkeletonMetricCard />
        <SkeletonMetricCard />
        <SkeletonMetricCard />
        <SkeletonMetricCard />
      </div>

      {/* Outcome Distribution Chart */}
      <div className="bg-slate-800 p-6 rounded-lg">
        <Skeleton className="h-6 w-48 mb-4" />
        <div className="flex items-end justify-around h-64 gap-4">
          <SkeletonChartBar />
          <SkeletonChartBar />
          <SkeletonChartBar />
        </div>
      </div>

      {/* Additional Metrics */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <div className="bg-slate-800 p-4 rounded-lg">
          <Skeleton className="h-4 w-24 mb-2" />
          <Skeleton className="h-7 w-16" />
        </div>
        <div className="bg-slate-800 p-4 rounded-lg">
          <Skeleton className="h-4 w-24 mb-2" />
          <Skeleton className="h-7 w-16" />
        </div>
        <div className="bg-slate-800 p-4 rounded-lg">
          <Skeleton className="h-4 w-32 mb-2" />
          <Skeleton className="h-7 w-20" />
        </div>
      </div>
    </div>
  );
}

export default Skeleton;
