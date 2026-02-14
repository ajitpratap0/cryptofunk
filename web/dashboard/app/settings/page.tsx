'use client'

import { useState, useEffect } from 'react'
import { Settings, Wifi, WifiOff, Sun, Moon, Globe, Info } from 'lucide-react'

export default function SettingsPage() {
  const [theme, setTheme] = useState<'dark' | 'light'>('dark')
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
    if (theme === 'light') {
      document.documentElement.classList.remove('dark')
    } else {
      document.documentElement.classList.add('dark')
    }
  }, [theme])

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
          {theme === 'dark' ? <Moon className="h-5 w-5" /> : <Sun className="h-5 w-5" />}
          Appearance
        </h2>
        <div className="flex gap-3">
          {(['dark', 'light'] as const).map((t) => (
            <button
              key={t}
              onClick={() => setTheme(t)}
              className={`px-4 py-2 rounded-lg text-sm font-medium transition-colors capitalize ${
                theme === t
                  ? 'bg-primary text-primary-foreground'
                  : 'bg-muted text-muted-foreground hover:bg-muted/80'
              }`}
            >
              {t}
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
