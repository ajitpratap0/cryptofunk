export default function AgentsLoading() {
  return (
    <div className="space-y-6 animate-pulse">
      <div className="h-8 w-48 rounded bg-card/50" />
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="h-48 rounded-lg bg-card/50 border border-border" />
        ))}
      </div>
      <div className="h-64 rounded-lg bg-card/50 border border-border" />
    </div>
  )
}
