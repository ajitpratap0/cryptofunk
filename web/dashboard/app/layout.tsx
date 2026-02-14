'use client'

import { useState } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Inter } from 'next/font/google'
import Header from '@/components/layout/Header'
import Sidebar from '@/components/layout/Sidebar'
import { ToastProvider } from '@/components/ui/Toast'
import './globals.css'

const inter = Inter({ subsets: ['latin'] })

// Create a client
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 0,
      staleTime: 30000,
      refetchOnWindowFocus: false,
    },
  },
})

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false)

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
              <div className={`transition-all duration-300 ${
                sidebarCollapsed ? 'lg:ml-16' : 'lg:ml-64'
              }`}>
                {/* Header */}
                <Header 
                  onToggleSidebar={() => setSidebarCollapsed(!sidebarCollapsed)}
                  sidebarCollapsed={sidebarCollapsed}
                />

                {/* Page Content */}
                <main className="p-6">
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