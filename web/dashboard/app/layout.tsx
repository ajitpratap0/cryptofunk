'use client'

import { useState, useEffect } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Inter } from 'next/font/google'
import Header from '@/components/layout/Header'
import Sidebar from '@/components/layout/Sidebar'
import { ToastProvider } from '@/components/ui/Toast'
import './globals.css'

const inter = Inter({ subsets: ['latin'] })

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
    // Server: always make a new query client
    return makeQueryClient()
  }
  // Browser: reuse the same query client
  if (!browserQueryClient) browserQueryClient = makeQueryClient()
  return browserQueryClient
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  const [sidebarCollapsed, setSidebarCollapsed] = useState(true)
  const queryClient = getQueryClient()

  // Auto-expand sidebar on desktop, keep collapsed on mobile
  useEffect(() => {
    const mql = window.matchMedia('(min-width: 1024px)')
    setSidebarCollapsed(!mql.matches)
    const handler = (e: MediaQueryListEvent) => setSidebarCollapsed(!e.matches)
    mql.addEventListener('change', handler)
    return () => mql.removeEventListener('change', handler)
  }, [])

  return (
    <html lang="en" className="dark">
      <body className={inter.className}>
        <QueryClientProvider client={queryClient}>
          <ToastProvider>
            <div className="min-h-screen bg-background">
              {/* Sidebar */}
              <Sidebar 
                collapsed={sidebarCollapsed}
                onToggle={() => setSidebarCollapsed(!sidebarCollapsed)}
              />

              {/* Main Content */}
              <div className={`min-h-screen flex flex-col transition-all duration-300 ${
                sidebarCollapsed ? 'lg:ml-16' : 'lg:ml-64'
              }`}>
                {/* Header */}
                <Header 
                  onToggleSidebar={() => setSidebarCollapsed(!sidebarCollapsed)}
                  sidebarCollapsed={sidebarCollapsed}
                />

                {/* Page Content */}
                <main className="flex-1 p-6">
                  {children}
                </main>
              </div>
            </div>
          </ToastProvider>
        </QueryClientProvider>
      </body>
    </html>
  )
}
