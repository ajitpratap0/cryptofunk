'use client'

import { useState, useEffect } from 'react'
import { useTheme } from 'next-themes'
import { Settings, Wifi, WifiOff, Sun, Moon, Monitor, Globe, Info } from 'lucide-react'

export default function SettingsPage() {
  const { theme, setTheme } = useTheme()
  const [mounted, setMounted] = useState(false)
  const [wsStatus, setWsStatus] = useState<'connected' | 'disconnected'>('disconnected')

  const apiUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
  const wsUrl = apiUrl.replace(/^http/, 'ws') + '/api/v1/ws'

  useEffect(() => {
    // Check WebSocket connectivity
    try {
      const ws = new WebSocket(wsUrl)
      ws.onopen = () => {
        setWsStatus('connected')
        ws.close()
      }
      ws.onerror = () => setWsStatus('disconnected')
      return () => ws.close()
    } catch {
      setWsStatus('disconnected')
    }
  }, [wsUrl])

  useEffect(() => {
    setMounted(true)
  }, [])

  return (
    <div className="space-y-6 max-w-2xl">
      <div>
        <h1 className="text-3xl font-bold flex items-center gap-3">
          <Settings className="h-8 w-8" />
          Settings
        </h1>
        <p className="text-muted-foreground mt-1">Dashboard configuration and status</p>
      </div>

      {/* API Configuration */}
      <div className="bg-card border border-border rounded-lg p-6 space-y-4">
        <h2 className="text-lg font-semibold flex items-center gap-2">
          <Globe className="h-5 w-5" />
          API Configuration
        </h2>
        <div className="space-y-3">
          <div>
            <label className="block text-sm text-muted-foreground mb-1">API URL</label>
            <input
              type="text"
              readOnly
              value={apiUrl}
              className="w-full bg-muted border border-border rounded-lg px-3 py-2 text-sm font-mono cursor-not-allowed"
            />
            <p className="text-xs text-muted-foreground mt-1">
              Set via NEXT_PUBLIC_API_URL environment variable
            </p>
          </div>
          <div>
            <label className="block text-sm text-muted-foreground mb-1">WebSocket URL</label>
            <input
              type="text"
              readOnly
              value={wsUrl}
              className="w-full bg-muted border border-border rounded-lg px-3 py-2 text-sm font-mono cursor-not-allowed"
            />
          </div>
        </div>
      </div>

      {/* WebSocket Status */}
      <div className="bg-card border border-border rounded-lg p-6 space-y-4">
        <h2 className="text-lg font-semibold flex items-center gap-2">
          {wsStatus === 'connected' ? (
            <Wifi className="h-5 w-5 text-green-400" />
          ) : (
            <WifiOff className="h-5 w-5 text-red-400" />
          )}
          WebSocket Status
        </h2>
        <div className="flex items-center gap-3">
          <span
            className={`h-3 w-3 rounded-full ${
              wsStatus === 'connected' ? 'bg-green-400 animate-pulse' : 'bg-red-400'
            }`}
          />
          <span className="text-sm font-medium capitalize">{wsStatus}</span>
        </div>
      </div>

      {/* Theme */}
      <div className="bg-card border border-border rounded-lg p-6 space-y-4">
        <h2 className="text-lg font-semibold flex items-center gap-2">
          {mounted && theme === 'light' ? <Sun className="h-5 w-5" /> : <Moon className="h-5 w-5" />}
          Appearance
        </h2>
        <div className="flex gap-3">
          {([
            { value: 'light', label: 'Light', icon: <Sun className="h-4 w-4" /> },
            { value: 'dark', label: 'Dark', icon: <Moon className="h-4 w-4" /> },
            { value: 'system', label: 'System', icon: <Monitor className="h-4 w-4" /> },
          ] as const).map((t) => (
            <button
              key={t.value}
              onClick={() => setTheme(t.value)}
              className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors flex items-center gap-2 ${
                mounted && theme === t.value
                  ? 'bg-primary text-primary-foreground'
                  : 'bg-muted text-muted-foreground hover:bg-muted/80'
              }`}
            >
              {t.icon}
              {t.label}
            </button>
          ))}
        </div>
      </div>

      {/* Version Info */}
      <div className="bg-card border border-border rounded-lg p-6 space-y-4">
        <h2 className="text-lg font-semibold flex items-center gap-2">
          <Info className="h-5 w-5" />
          About
        </h2>
        <div className="space-y-2 text-sm">
          <div className="flex justify-between">
            <span className="text-muted-foreground">Dashboard</span>
            <span className="font-mono">CryptoFunk v0.1.0</span>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">Framework</span>
            <span className="font-mono">Next.js 15</span>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">API Backend</span>
            <span className="font-mono">Go / Gin</span>
          </div>
        </div>
      </div>
    </div>
  )
}
