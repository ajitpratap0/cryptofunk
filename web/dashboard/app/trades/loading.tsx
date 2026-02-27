export default function TradesLoading() {
  return (
    <div className="space-y-6 animate-pulse">
      <div className="h-8 w-36 rounded bg-card/50" />
      <div className="flex gap-4">
        {Array.from({ length: 3 }).map((_, i) => (
          <div key={i} className="h-10 w-24 rounded bg-card/50" />
        ))}
      </div>
      <div className="space-y-3">
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className="h-16 rounded-lg bg-card/50 border border-border" />
        ))}
      </div>
    </div>
  )
}
