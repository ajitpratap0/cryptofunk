'use client'

import { useState, useEffect, useRef, useCallback } from 'react'
import type { WebSocketMessage, Trade, Position, Order, Agent } from '@/lib/types'
import { WS_URL, WS_RECONNECT_ATTEMPTS, WS_RECONNECT_INTERVAL } from '@/lib/constants'

export interface WebSocketState {
  isConnected: boolean
  lastMessage: WebSocketMessage | null
  connectionStatus: 'connecting' | 'connected' | 'disconnected' | 'error'
  error: string | null
  reconnectAttempts: number
}

export interface UseWebSocketOptions {
  url?: string
  protocols?: string | string[]
  reconnect?: boolean
  maxReconnectAttempts?: number
  reconnectInterval?: number
  onOpen?: () => void
  onClose?: () => void
  onError?: (error: Event) => void
  onMessage?: (message: WebSocketMessage) => void
}

export function useWebSocket(options: UseWebSocketOptions = {}) {
  const {
    url = WS_URL,
    protocols,
    reconnect = true,
    maxReconnectAttempts = WS_RECONNECT_ATTEMPTS,
    reconnectInterval = WS_RECONNECT_INTERVAL,
    onOpen,
    onClose,
    onError,
    onMessage,
  } = options

  const [state, setState] = useState<WebSocketState>({
    isConnected: false,
    lastMessage: null,
    connectionStatus: 'disconnected',
    error: null,
    reconnectAttempts: 0,
  })

  const ws = useRef<WebSocket | null>(null)
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null)
  const shouldReconnectRef = useRef(true)
  const reconnectAttemptsRef = useRef(0)

  const updateState = useCallback((updates: Partial<WebSocketState>) => {
    setState(prev => ({ ...prev, ...updates }))
  }, [])

  const connect = useCallback(() => {
    if (ws.current?.readyState === WebSocket.OPEN) {
      return
    }

    updateState({ 
      connectionStatus: 'connecting',
      error: null 
    })

    try {
      ws.current = new WebSocket(url, protocols)

      ws.current.onopen = () => {
        updateState({
          isConnected: true,
          connectionStatus: 'connected',
          error: null,
          reconnectAttempts: 0,
        })
        reconnectAttemptsRef.current = 0
        onOpen?.()
      }

      ws.current.onclose = () => {
        updateState({
          isConnected: false,
          connectionStatus: 'disconnected',
        })
        onClose?.()

        // Attempt to reconnect if enabled and we haven't exceeded max attempts
        if (
          reconnect &&
          shouldReconnectRef.current &&
          reconnectAttemptsRef.current < maxReconnectAttempts
        ) {
          reconnectTimeoutRef.current = setTimeout(() => {
            reconnectAttemptsRef.current += 1
            updateState({
              reconnectAttempts: reconnectAttemptsRef.current
            })
            connect()
          }, reconnectInterval)
        } else if (reconnect && shouldReconnectRef.current && reconnectAttemptsRef.current >= maxReconnectAttempts) {
          // Max retries exhausted — surface error so consumers can react
          updateState({
            connectionStatus: 'error',
            error: `WebSocket connection failed after ${maxReconnectAttempts} attempt${maxReconnectAttempts === 1 ? '' : 's'}`,
          })
        }
      }

      ws.current.onerror = (error) => {
        updateState({
          isConnected: false,
          connectionStatus: 'error',
          error: 'WebSocket connection error',
        })
        onError?.(error)
      }

      ws.current.onmessage = (event) => {
        try {
          const message: WebSocketMessage = JSON.parse(event.data)
          updateState({ lastMessage: message })
          onMessage?.(message)
        } catch (error) {
          console.error('Failed to parse WebSocket message:', error)
          updateState({ 
            error: 'Failed to parse message' 
          })
        }
      }
    } catch (error) {
      updateState({
        isConnected: false,
        connectionStatus: 'error',
        error: error instanceof Error ? error.message : 'Connection failed',
      })
    }
  }, [url, protocols, onOpen, onClose, onError, onMessage, reconnect, maxReconnectAttempts, reconnectInterval, updateState])

  const disconnect = useCallback(() => {
    shouldReconnectRef.current = false
    
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current)
      reconnectTimeoutRef.current = null
    }

    if (ws.current) {
      ws.current.close()
      ws.current = null
    }

    updateState({
      isConnected: false,
      connectionStatus: 'disconnected',
    })
  }, [updateState])

  const sendMessage = useCallback((message: Record<string, unknown>) => {
    if (ws.current?.readyState === WebSocket.OPEN) {
      ws.current.send(JSON.stringify(message))
      return true
    }
    return false
  }, [])

  // Auto-connect on mount
  useEffect(() => {
    shouldReconnectRef.current = true
    connect()

    return () => {
      shouldReconnectRef.current = false
      disconnect()
    }
  }, [connect, disconnect])

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current)
      }
    }
  }, [])

  return {
    ...state,
    connect,
    disconnect,
    sendMessage,
  }
}

// Specialized hook for trading data
export function useTradingWebSocket() {
  // #104: replaced Record<string, any>[] with proper types so
  // consumers get type-safe access to trade/position/order/agent
  // fields. WebSocketMessage.data is still `any` at runtime (the
  // backend sends untyped JSON), but the state arrays document the
  // expected shape and catch misuse at compile time.
  //
  // WARNING: the backend WS payload uses the same snake_case JSON
  // encoding as the REST API (encoding/json on Go structs with
  // snake_case tags). The REST path runs mapApiTrade to convert to
  // camelCase Trade shape, but the WS path does NOT — message.data
  // is pushed directly into the Trade[] array. Consumers accessing
  // e.g. trade.entryPrice will get undefined at runtime because the
  // wire format has `price`, not `entryPrice`. A mapApiTrade pass
  // (or a shared mapper per message.type) is needed to close this
  // gap. Tracked as a follow-up alongside #224.
  const [trades, setTrades] = useState<Trade[]>([])
  const [positions, setPositions] = useState<Position[]>([])
  const [orders, setOrders] = useState<Order[]>([])
  const [agents, setAgents] = useState<Agent[]>([])
  const [marketData, setMarketData] = useState<Record<string, unknown>>({})

  const handleMessage = useCallback((message: WebSocketMessage) => {
    switch (message.type) {
      case 'trade':
        setTrades(prev => {
          const newTrades = [message.data, ...prev]
          return newTrades.slice(0, 100) // Keep last 100 trades
        })
        break
      
      case 'position':
        setPositions(prev => {
          const filtered = prev.filter(p => p.symbol !== message.data.symbol)
          return [...filtered, message.data]
        })
        break
      
      case 'order':
        setOrders(prev => {
          const filtered = prev.filter(o => o.id !== message.data.id)
          if (message.data.status !== 'cancelled' && message.data.status !== 'filled') {
            return [...filtered, message.data]
          }
          return filtered
        })
        break
      
      case 'agent':
        setAgents(prev => {
          const filtered = prev.filter(a => a.name !== message.data.name)
          return [...filtered, message.data]
        })
        break
      
      case 'market':
        setMarketData(prev => ({
          ...prev,
          [message.data.symbol]: message.data
        }))
        break
    }
  }, [])

  const wsState = useWebSocket({
    onMessage: handleMessage,
  })

  return {
    ...wsState,
    trades,
    positions,
    orders,
    agents,
    marketData,
  }
}