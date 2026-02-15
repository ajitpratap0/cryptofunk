export default function Loading() {
  return (
    <div className="space-y-6 animate-pulse">
      {/* Stats row */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="h-28 rounded-lg bg-card/50 border border-border" />
        ))}
      </div>
      {/* Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="h-80 rounded-lg bg-card/50 border border-border" />
        <div className="h-80 rounded-lg bg-card/50 border border-border" />
      </div>
      {/* Table */}
      <div className="h-64 rounded-lg bg-card/50 border border-border" />
    </div>
  )
}
