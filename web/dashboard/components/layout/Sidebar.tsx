'use client'

import { useState, useEffect } from 'react'
import { useRouter, usePathname } from 'next/navigation'
import { 
  LayoutDashboard,
  TrendingUp,
  Activity,
  Bot,
  Shield,
  Settings,
  ChevronLeft,
  ChevronRight
} from 'lucide-react'
import { cn } from '@/lib/utils'

interface SidebarProps {
  collapsed: boolean
  onToggle: () => void
  className?: string
}

const navigationItems = [
  {
    name: 'Dashboard',
    href: '/',
    icon: LayoutDashboard,
    description: 'Portfolio overview'
  },
  {
    name: 'Live Trades',
    href: '/trades',
    icon: TrendingUp,
    description: 'Real-time trading activity'
  },
  {
    name: 'Performance',
    href: '/performance',
    icon: Activity,
    description: 'Charts and analytics'
  },
  {
    name: 'Agents',
    href: '/agents',
    icon: Bot,
    description: 'AI agent scorecard'
  },
  {
    name: 'Risk',
    href: '/risk',
    icon: Shield,
    description: 'Risk management'
  }
]

const bottomItems = [
  {
    name: 'Settings',
    href: '/settings',
    icon: Settings,
    description: 'Application settings'
  }
]

export function Sidebar({ collapsed, onToggle, className }: SidebarProps) {
  const router = useRouter()
  const pathname = usePathname()
  const [mounted, setMounted] = useState(false)

  useEffect(() => {
    setMounted(true)
  }, [])

  if (!mounted) {
    return null // Prevent hydration issues
  }

  const isActive = (href: string) => {
    if (href === '/') {
      return pathname === '/'
    }
    return pathname.startsWith(href)
  }

  return (
    <>
      {/* Overlay for mobile */}
      {!collapsed && (
        <div
          className="fixed inset-0 z-30 bg-black/50 lg:hidden"
          onClick={onToggle}
        />
      )}

      {/* Sidebar */}
      <aside
        className={cn(
          "sidebar",
          collapsed ? "sidebar-collapsed" : "sidebar-expanded",
          "lg:relative lg:translate-x-0",
          !collapsed ? "translate-x-0" : "-translate-x-full lg:translate-x-0",
          className
        )}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-4 border-b border-border">
          {!collapsed && (
            <div className="flex items-center gap-2">
              <div className="w-8 h-8 bg-gradient-to-br from-purple-500 to-pink-500 rounded-lg flex items-center justify-center">
                <span className="text-white text-sm font-bold">CF</span>
              </div>
              <span className="font-semibold text-sm">CryptoFunk</span>
            </div>
          )}
          
          <button
            onClick={onToggle}
            className="p-1.5 hover:bg-muted/50 rounded-lg transition-colors hidden lg:flex"
            title={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
            aria-label="Toggle sidebar"
          >
            {collapsed ? (
              <ChevronRight className="h-4 w-4" />
            ) : (
              <ChevronLeft className="h-4 w-4" />
            )}
          </button>
        </div>

        {/* Navigation */}
        <nav className="flex-1 p-4 space-y-2">
          {navigationItems.map((item) => {
            const Icon = item.icon
            const active = isActive(item.href)
            
            return (
              <button
                key={item.href}
                onClick={() => {
                  router.push(item.href)
                  // Close mobile sidebar after navigation
                  if (window.innerWidth < 1024) {
                    onToggle()
                  }
                }}
                className={cn(
                  "sidebar-item w-full",
                  active && "sidebar-item-active"
                )}
                title={collapsed ? item.name : undefined}
              >
                <Icon className={cn(
                  "h-5 w-5 flex-shrink-0",
                  collapsed ? "mx-auto" : "mr-3"
                )} />
                {!collapsed && (
                  <div className="flex flex-col items-start flex-1 min-w-0">
                    <span className="text-sm font-medium truncate">
                      {item.name}
                    </span>
                    <span className="text-xs text-muted-foreground truncate">
                      {item.description}
                    </span>
                  </div>
                )}
              </button>
            )
          })}
        </nav>

        {/* Bottom Section */}
        <div className="p-4 border-t border-border space-y-2">
          {bottomItems.map((item) => {
            const Icon = item.icon
            const active = isActive(item.href)
            
            return (
              <button
                key={item.href}
                onClick={() => {
                  router.push(item.href)
                  if (window.innerWidth < 1024) {
                    onToggle()
                  }
                }}
                className={cn(
                  "sidebar-item w-full",
                  active && "sidebar-item-active"
                )}
                title={collapsed ? item.name : undefined}
              >
                <Icon className={cn(
                  "h-5 w-5 flex-shrink-0",
                  collapsed ? "mx-auto" : "mr-3"
                )} />
                {!collapsed && (
                  <span className="text-sm font-medium truncate">
                    {item.name}
                  </span>
                )}
              </button>
            )
          })}

          {/* Version Info */}
          {!collapsed && (
            <div className="pt-4 text-center">
              <p className="text-xs text-muted-foreground">
                v1.0.0
              </p>
              <p className="text-xs text-muted-foreground">
                Build 2024.01.15
              </p>
            </div>
          )}
        </div>

        {/* Collapse indicator for desktop */}
        {collapsed && (
          <div className="hidden lg:block p-2 text-center border-t border-border">
            <div className="w-2 h-2 bg-muted-foreground rounded-full mx-auto" />
          </div>
        )}
      </aside>
    </>
  )
}

export default Sidebar