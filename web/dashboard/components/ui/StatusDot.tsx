'use client'

import { cn } from '@/lib/utils'

type Status = 'connected' | 'disconnected' | 'connecting' | 'error' | 'active' | 'idle' | 'warning' | 'healthy' | 'degraded' | 'down' | 'disabled' | 'stopped' | 'running'

interface StatusDotProps {
  status: Status
  label?: string | null
  size?: 'sm' | 'md' | 'lg'
  animated?: boolean
  className?: string
}

const statusConfig: Record<Status, { color: string; label: string }> = {
  connected: { color: 'bg-profit', label: 'Connected' },
  disconnected: { color: 'bg-muted-foreground', label: 'Disconnected' },
  connecting: { color: 'bg-warning', label: 'Connecting' },
  error: { color: 'bg-loss', label: 'Error' },
  active: { color: 'bg-profit', label: 'Active' },
  idle: { color: 'bg-info', label: 'Idle' },
  warning: { color: 'bg-warning', label: 'Warning' },
  healthy: { color: 'bg-profit', label: 'Healthy' },
  degraded: { color: 'bg-warning', label: 'Degraded' },
  down: { color: 'bg-loss', label: 'Down' },
  disabled: { color: 'bg-muted-foreground', label: 'Disabled' },
  stopped: { color: 'bg-muted-foreground', label: 'Stopped' },
  running: { color: 'bg-profit', label: 'Running' },
}

const sizeConfig = {
  sm: 'h-2 w-2',
  md: 'h-3 w-3',
  lg: 'h-4 w-4',
}

export function StatusDot({ 
  status, 
  label, 
  size = 'md', 
  animated = false, 
  className 
}: StatusDotProps) {
  const normalizedStatus = (status?.toLowerCase?.() || 'disconnected') as Status
  const config = statusConfig[normalizedStatus] || statusConfig['disconnected']
  const displayLabel = label || config.label

  return (
    <div className={cn("flex items-center gap-2", className)}>
      <div
        className={cn(
          "rounded-full",
          sizeConfig[size],
          config?.color || 'bg-muted-foreground',
          animated && normalizedStatus === 'connecting' && "animate-pulse",
          animated && normalizedStatus === 'connected' && "animate-pulse"
        )}
        title={displayLabel}
        aria-label={label === null ? displayLabel : undefined}
        role={label === null ? "img" : undefined}
      />
      {label !== null && (
        <span className="text-sm text-muted-foreground">
          {displayLabel}
        </span>
      )}
    </div>
  )
}

interface ConnectionStatusProps {
  isConnected: boolean
  lastUpdate?: string
  className?: string
}

export function ConnectionStatus({ isConnected, lastUpdate, className }: ConnectionStatusProps) {
  const formatLastUpdate = (timestamp: string) => {
    const now = new Date()
    const update = new Date(timestamp)
    const diffMs = now.getTime() - update.getTime()
    const diffSec = Math.floor(diffMs / 1000)
    
    if (diffSec < 60) return `${diffSec}s ago`
    if (diffSec < 3600) return `${Math.floor(diffSec / 60)}m ago`
    return `${Math.floor(diffSec / 3600)}h ago`
  }

  return (
    <div className={cn("flex items-center gap-3", className)}>
      <StatusDot 
        status={isConnected ? 'connected' : 'disconnected'} 
        animated={isConnected}
        label={null}
      />
      <div className="flex flex-col text-sm">
        <span className="text-foreground font-medium">
          {isConnected ? 'Live' : 'Disconnected'}
        </span>
        {lastUpdate && (
          <span className="text-muted-foreground text-xs">
            {isConnected ? `Updated ${formatLastUpdate(lastUpdate)}` : 'No connection'}
          </span>
        )}
      </div>
    </div>
  )
}

interface SystemStatusIndicatorProps {
  status: 'healthy' | 'degraded' | 'down'
  services: Record<string, 'up' | 'down' | 'degraded'>
  className?: string
}

export function SystemStatusIndicator({ 
  status, 
  services, 
  className 
}: SystemStatusIndicatorProps) {
  const serviceEntries = Object.entries(services ?? {})
  const upServices = serviceEntries.filter(([, status]) => status === 'up').length
  const totalServices = serviceEntries.length

  return (
    <div className={cn("flex items-center gap-3", className)}>
      <StatusDot status={status} animated={status === 'degraded'} label={null} />
      <div className="flex flex-col text-sm">
        <span className="text-foreground font-medium capitalize">
          {status}
        </span>
        <span className="text-muted-foreground text-xs">
          {upServices}/{totalServices} services
        </span>
      </div>
    </div>
  )
}

interface AgentStatusListProps {
  agents: Array<{
    name: string
    status: 'active' | 'idle' | 'error' | 'disabled'
  }>
  className?: string
}

export function AgentStatusList({ agents, className }: AgentStatusListProps) {
  return (
    <div className={cn("space-y-2", className)}>
      {agents.map((agent) => (
        <div key={agent.name} className="flex items-center justify-between">
          <span className="text-sm text-foreground">{agent.name}</span>
          <StatusDot 
            status={agent.status} 
            label={null}
            size="sm"
          />
        </div>
      ))}
    </div>
  )
}