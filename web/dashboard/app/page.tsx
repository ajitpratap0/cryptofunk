import { Suspense } from 'react'
import DashboardContent from '@/components/dashboard/DashboardContent'

export default function DashboardPage() {
  return (
    <div className="space-y-6">
      {/* Static header — renders instantly via SSR (LCP element) */}
      <div>
        <h1 className="text-3xl font-bold">Portfolio Overview</h1>
        <p className="text-muted-foreground">
          Real-time performance and trading activity
        </p>
      </div>

      {/* Data-dependent content — streams in after hydration */}
      <Suspense fallback={<DashboardSkeleton />}>
        <DashboardContent />
      </Suspense>
    </div>
  )
}

function DashboardSkeleton() {
  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-6">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="h-32 rounded-lg bg-muted/50 animate-pulse" />
        ))}
      </div>
      <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
        <div className="h-80 rounded-lg bg-muted/50 animate-pulse" />
        <div className="h-80 rounded-lg bg-muted/50 animate-pulse" />
      </div>
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="h-48 rounded-lg bg-muted/50 animate-pulse" />
        ))}
      </div>
    </div>
  )
}
