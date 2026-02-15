'use client'

import { useState, useEffect } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import Header from '@/components/layout/Header'
import Sidebar from '@/components/layout/Sidebar'
import { ToastProvider } from '@/components/ui/Toast'

function makeQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: 0,
        staleTime: 30000,
        refetchOnWindowFocus: false,
      },
    },
  })
}

let browserQueryClient: QueryClient | undefined

function getQueryClient() {
  if (typeof window === 'undefined') {
    return makeQueryClient()
  }
  if (!browserQueryClient) browserQueryClient = makeQueryClient()
  return browserQueryClient
}

export function Providers({ children }: { children: React.ReactNode }) {
  const [sidebarCollapsed, setSidebarCollapsed] = useState(true)
  const queryClient = getQueryClient()

  useEffect(() => {
    const mql = window.matchMedia('(min-width: 1024px)')
    setSidebarCollapsed(!mql.matches)
    const handler = (e: MediaQueryListEvent) => setSidebarCollapsed(!e.matches)
    mql.addEventListener('change', handler)
    return () => mql.removeEventListener('change', handler)
  }, [])

  return (
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <div className="min-h-screen bg-background">
          <Sidebar
            collapsed={sidebarCollapsed}
            onToggle={() => setSidebarCollapsed(!sidebarCollapsed)}
          />
          <div className={`min-h-screen flex flex-col transition-all duration-300 ${
            sidebarCollapsed ? 'lg:ml-16' : 'lg:ml-64'
          }`}>
            <Header
              onToggleSidebar={() => setSidebarCollapsed(!sidebarCollapsed)}
              sidebarCollapsed={sidebarCollapsed}
            />
            <main className="flex-1 p-6">
              {children}
            </main>
          </div>
        </div>
      </ToastProvider>
    </QueryClientProvider>
  )
}
