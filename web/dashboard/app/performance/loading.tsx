export default function PerformanceLoading() {
  return (
    <div className="space-y-6 animate-pulse">
      <div className="h-8 w-44 rounded bg-card/50" />
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="h-28 rounded-lg bg-card/50 border border-border" />
        ))}
      </div>
      <div className="h-96 rounded-lg bg-card/50 border border-border" />
    </div>
  )
}
