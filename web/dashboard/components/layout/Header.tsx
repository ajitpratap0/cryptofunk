'use client'

import { useState } from 'react'
import { useToast } from '@/hooks/useToast'
import { 
  Play, 
  Pause, 
  Square, 
  Settings, 
  Download,
  RefreshCw,
  Bell,
  Menu,
  MoreVertical
} from 'lucide-react'
import { ConnectionStatus } from '@/components/ui/StatusDot'
import { useTradingWebSocket } from '@/hooks/useWebSocket'
import { 
  useStartTrading, 
  useStopTrading, 
  usePauseTrading, 
  useResumeTrading 
} from '@/hooks/useTradeData'
import { cn, formatDateTime } from '@/lib/utils'

interface HeaderProps {
  onToggleSidebar: () => void
  sidebarCollapsed: boolean
  className?: string
}

export function Header({ onToggleSidebar, sidebarCollapsed, className }: HeaderProps) {
  const [tradingStatus, setTradingStatus] = useState<'active' | 'paused' | 'stopped'>('active')
  const [showActions, setShowActions] = useState(false)
  
  const { isConnected, lastMessage } = useTradingWebSocket()
  const startTrading = useStartTrading()
  const stopTrading = useStopTrading()
  const pauseTrading = usePauseTrading()
  const resumeTrading = useResumeTrading()
  const { toast } = useToast()

  const handleTradingAction = async (action: 'start' | 'stop' | 'pause' | 'resume') => {
    try {
      switch (action) {
        case 'start':
          await startTrading.mutateAsync()
          setTradingStatus('active')
          toast({ 
            title: 'Trading Started', 
            description: 'AI agents are now actively trading',
            variant: 'success' 
          })
          break
        case 'stop':
          await stopTrading.mutateAsync()
          setTradingStatus('stopped')
          toast({ 
            title: 'Trading Stopped', 
            description: 'All trading activity has been halted',
            variant: 'warning' 
          })
          break
        case 'pause':
          await pauseTrading.mutateAsync()
          setTradingStatus('paused')
          toast({ 
            title: 'Trading Paused', 
            description: 'Trading activity temporarily suspended',
            variant: 'warning' 
          })
          break
        case 'resume':
          await resumeTrading.mutateAsync()
          setTradingStatus('active')
          toast({ 
            title: 'Trading Resumed', 
            description: 'AI agents have resumed trading',
            variant: 'success' 
          })
          break
      }
    } catch (error) {
      console.error('Trading action failed:', error)
      toast({ 
        title: 'Action Failed', 
        description: error instanceof Error ? error.message : 'An unexpected error occurred',
        variant: 'error' 
      })
    }
  }

  const getTradingControls = () => {
    switch (tradingStatus) {
      case 'active':
        return (
          <>
            <button
              onClick={() => handleTradingAction('pause')}
              className="flex items-center gap-2 px-3 py-2 bg-warning/20 text-warning border border-warning/30 rounded-lg hover:bg-warning/30 transition-colors text-sm font-medium"
              disabled={pauseTrading.isPending}
            >
              <Pause className="h-4 w-4" />
              Pause
            </button>
            <button
              onClick={() => handleTradingAction('stop')}
              className="flex items-center gap-2 px-3 py-2 bg-loss/20 text-loss border border-loss/30 rounded-lg hover:bg-loss/30 transition-colors text-sm font-medium"
              disabled={stopTrading.isPending}
            >
              <Square className="h-4 w-4" />
              Stop
            </button>
          </>
        )
      
      case 'paused':
        return (
          <>
            <button
              onClick={() => handleTradingAction('resume')}
              className="flex items-center gap-2 px-3 py-2 bg-profit/20 text-profit border border-profit/30 rounded-lg hover:bg-profit/30 transition-colors text-sm font-medium"
              disabled={resumeTrading.isPending}
            >
              <Play className="h-4 w-4" />
              Resume
            </button>
            <button
              onClick={() => handleTradingAction('stop')}
              className="flex items-center gap-2 px-3 py-2 bg-loss/20 text-loss border border-loss/30 rounded-lg hover:bg-loss/30 transition-colors text-sm font-medium"
              disabled={stopTrading.isPending}
            >
              <Square className="h-4 w-4" />
              Stop
            </button>
          </>
        )
      
      case 'stopped':
        return (
          <button
            onClick={() => handleTradingAction('start')}
            className="flex items-center gap-2 px-3 py-2 bg-profit/20 text-profit border border-profit/30 rounded-lg hover:bg-profit/30 transition-colors text-sm font-medium"
            disabled={startTrading.isPending}
          >
            <Play className="h-4 w-4" />
            Start Trading
          </button>
        )
    }
  }

  return (
    <header className={cn(
      "sticky top-0 z-30 bg-card/80 backdrop-blur-sm border-b border-border",
      className
    )}>
      <div className="flex items-center justify-between h-16 px-6">
        {/* Left Section */}
        <div className="flex items-center gap-4">
          <button
            onClick={onToggleSidebar}
            className="p-2 hover:bg-muted/50 rounded-lg transition-colors lg:hidden"
            aria-label="Open menu"
          >
            <Menu className="h-5 w-5" />
          </button>
          
          <div className="flex items-center gap-3">
            <h1 className="text-xl font-bold bg-gradient-to-r from-purple-400 to-pink-400 bg-clip-text text-transparent">
              CryptoFunk
            </h1>
            <div className="h-6 w-px bg-border" />
            <span className="text-sm text-muted-foreground font-medium">
              AI Trading Dashboard
            </span>
          </div>
        </div>

        {/* Center Section - Trading Status */}
        <div className="hidden md:flex items-center gap-4">
          <div className="flex items-center gap-2 px-3 py-2 bg-muted/50 rounded-lg">
            <div className={cn(
              "h-2 w-2 rounded-full",
              tradingStatus === 'active' && "bg-profit animate-pulse",
              tradingStatus === 'paused' && "bg-warning",
              tradingStatus === 'stopped' && "bg-loss"
            )} />
            <span className="text-sm font-medium capitalize">
              {tradingStatus === 'active' ? 'Trading Live' : tradingStatus}
            </span>
          </div>
        </div>

        {/* Right Section */}
        <div className="flex items-center gap-3">
          {/* Connection Status */}
          <ConnectionStatus
            isConnected={isConnected}
            lastUpdate={lastMessage?.timestamp}
            className="hidden sm:flex"
          />

          {/* Trading Controls */}
          <div className="hidden lg:flex items-center gap-2">
            {getTradingControls()}
          </div>

          {/* Action Menu */}
          <div className="relative">
            <button
              onClick={() => setShowActions(!showActions)}
              className="p-2 hover:bg-muted/50 rounded-lg transition-colors"
              aria-label="More actions"
              aria-expanded={showActions}
              aria-haspopup="menu"
            >
              <MoreVertical className="h-5 w-5" />
            </button>

            {showActions && (
              <div 
                className="absolute right-0 top-full mt-2 w-48 bg-card border border-border rounded-lg shadow-lg py-2 z-50" 
                role="menu"
                aria-label="More actions menu"
              >
                <button 
                  className="flex items-center gap-3 w-full px-4 py-2 text-sm hover:bg-muted/50 transition-colors"
                  role="menuitem"
                >
                  <RefreshCw className="h-4 w-4" />
                  Refresh Data
                </button>
                <button 
                  className="flex items-center gap-3 w-full px-4 py-2 text-sm hover:bg-muted/50 transition-colors"
                  role="menuitem"
                >
                  <Download className="h-4 w-4" />
                  Export Report
                </button>
                <button 
                  className="flex items-center gap-3 w-full px-4 py-2 text-sm hover:bg-muted/50 transition-colors"
                  role="menuitem"
                >
                  <Settings className="h-4 w-4" />
                  Settings
                </button>
                <div className="border-t border-border my-2" />
                <button 
                  className="flex items-center gap-3 w-full px-4 py-2 text-sm hover:bg-muted/50 transition-colors"
                  role="menuitem"
                >
                  <Bell className="h-4 w-4" />
                  Notifications
                </button>
              </div>
            )}
          </div>

          {/* Mobile Trading Controls */}
          <div className="lg:hidden">
            {getTradingControls()}
          </div>
        </div>
      </div>

      {/* Mobile Connection Status */}
      <div className="sm:hidden px-6 pb-3">
        <ConnectionStatus
          isConnected={isConnected}
          lastUpdate={lastMessage?.timestamp}
        />
      </div>
    </header>
  )
}

export default Header