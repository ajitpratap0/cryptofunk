import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"
import { format, formatDistanceToNow, parseISO } from "date-fns"
import { COLORS, FORMATTING } from "./constants"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// Number Formatting
export function formatCurrency(
  value: number,
  currency: string = 'USD',
  options?: Intl.NumberFormatOptions
): string {
  const merged = {
    style: 'currency' as const,
    currency,
    ...FORMATTING.currency,
    ...options,
  }
  // Ensure min doesn't exceed max
  if (merged.maximumFractionDigits !== undefined && merged.minimumFractionDigits !== undefined) {
    merged.minimumFractionDigits = Math.min(merged.minimumFractionDigits, merged.maximumFractionDigits)
  }
  return new Intl.NumberFormat('en-US', merged).format(value)
}

export function formatPercentage(
  value: number,
  options?: Intl.NumberFormatOptions
): string {
  return new Intl.NumberFormat('en-US', {
    style: 'percent',
    ...FORMATTING.percentage,
    ...options,
  }).format(value / 100)
}

export function formatNumber(
  value: number,
  options?: Intl.NumberFormatOptions
): string {
  return new Intl.NumberFormat('en-US', options).format(value)
}

export function formatCrypto(
  value: number,
  symbol?: string,
  options?: Intl.NumberFormatOptions
): string {
  const formatted = new Intl.NumberFormat('en-US', {
    ...FORMATTING.crypto,
    ...options,
  }).format(value)
  
  return symbol ? `${formatted} ${symbol}` : formatted
}

export function formatCompactNumber(value: number): string {
  const formatter = new Intl.NumberFormat('en-US', {
    notation: 'compact',
    maximumFractionDigits: 1,
  })
  return formatter.format(value)
}

// Date Formatting
export function formatDate(date: string | Date, formatStr: string = 'MMM dd, yyyy'): string {
  const dateObj = typeof date === 'string' ? parseISO(date) : date
  return format(dateObj, formatStr)
}

export function formatDateTime(date: string | Date): string {
  return formatDate(date, 'MMM dd, yyyy HH:mm')
}

export function formatTime(date: string | Date): string {
  return formatDate(date, 'HH:mm:ss')
}

export function formatRelativeTime(date: string | Date): string {
  const dateObj = typeof date === 'string' ? parseISO(date) : date
  return formatDistanceToNow(dateObj, { addSuffix: true })
}

// Color Utilities
export function getPnlColor(value: number): string {
  if (value > 0) return COLORS.profit
  if (value < 0) return COLORS.loss
  return COLORS.neutral
}

export function getStatusColor(status: string): string {
  switch (status.toLowerCase()) {
    case 'active':
    case 'healthy':
    case 'filled':
    case 'open':
      return COLORS.profit
    case 'error':
    case 'down':
    case 'rejected':
    case 'cancelled':
      return COLORS.loss
    case 'warning':
    case 'degraded':
    case 'pending':
      return COLORS.warning
    case 'idle':
    case 'disabled':
    case 'closed':
      return COLORS.neutral
    default:
      return COLORS.info
  }
}

export function getAgentStatusColor(status: string): string {
  switch (status) {
    case 'active': return COLORS.profit
    case 'idle': return COLORS.info
    case 'error': return COLORS.loss
    case 'disabled': return COLORS.neutral
    default: return COLORS.neutral
  }
}

// Chart Utilities
export function generateChartColor(index: number): string {
  const colors = [
    COLORS.primary, COLORS.profit, COLORS.loss, COLORS.warning,
    COLORS.info, '#ec4899', '#10b981', '#f97316',
    '#06b6d4', '#84cc16', '#a855f7', '#e11d48'
  ]
  return colors[index % colors.length]
}

// Data Processing
export function calculatePnlPercent(entryPrice: number, currentPrice: number, side: 'long' | 'short'): number {
  const priceDiff = currentPrice - entryPrice
  const multiplier = side === 'long' ? 1 : -1
  return (priceDiff / entryPrice) * multiplier * 100
}

export function calculateSharpeRatio(returns: number[], riskFreeRate: number = 0): number {
  if (returns.length === 0) return 0
  
  const avgReturn = returns.reduce((sum, r) => sum + r, 0) / returns.length
  const avgExcessReturn = avgReturn - riskFreeRate
  
  const variance = returns.reduce((sum, r) => sum + Math.pow(r - avgReturn, 2), 0) / returns.length
  const stdDev = Math.sqrt(variance)
  
  return stdDev === 0 ? 0 : avgExcessReturn / stdDev
}

export function calculateMaxDrawdown(equityPoints: Array<{ value: number }>): number {
  if (equityPoints.length === 0) return 0
  
  let maxDrawdown = 0
  let peak = equityPoints[0].value
  
  for (const point of equityPoints) {
    if (point.value > peak) {
      peak = point.value
    } else {
      const drawdown = (peak - point.value) / peak
      maxDrawdown = Math.max(maxDrawdown, drawdown)
    }
  }
  
  return maxDrawdown
}

export function calculateWinRate(trades: Array<{ pnl: number }>): number {
  if (trades.length === 0) return 0
  const winningTrades = trades.filter(trade => trade.pnl > 0).length
  return (winningTrades / trades.length) * 100
}

// Validation
export function isValidNumber(value: any): boolean {
  return typeof value === 'number' && !isNaN(value) && isFinite(value)
}

export function isValidDate(date: any): boolean {
  return date instanceof Date && !isNaN(date.getTime())
}

export function isValidString(value: any): boolean {
  return typeof value === 'string' && value.trim().length > 0
}

// Array Utilities
export function groupBy<T, K extends keyof any>(
  array: T[],
  getKey: (item: T) => K
): Record<K, T[]> {
  return array.reduce((groups, item) => {
    const key = getKey(item)
    groups[key] = groups[key] || []
    groups[key].push(item)
    return groups
  }, {} as Record<K, T[]>)
}

export function sortBy<T>(
  array: T[],
  getter: (item: T) => string | number,
  direction: 'asc' | 'desc' = 'asc'
): T[] {
  return [...array].sort((a, b) => {
    const aVal = getter(a)
    const bVal = getter(b)
    
    if (direction === 'asc') {
      return aVal < bVal ? -1 : aVal > bVal ? 1 : 0
    } else {
      return aVal > bVal ? -1 : aVal < bVal ? 1 : 0
    }
  })
}

// Debounce
export function debounce<T extends (...args: any[]) => any>(
  func: T,
  wait: number
): (...args: Parameters<T>) => void {
  let timeout: NodeJS.Timeout | null = null
  
  return (...args: Parameters<T>) => {
    if (timeout) clearTimeout(timeout)
    timeout = setTimeout(() => func(...args), wait)
  }
}

// Throttle
export function throttle<T extends (...args: any[]) => any>(
  func: T,
  limit: number
): (...args: Parameters<T>) => void {
  let inThrottle: boolean
  
  return (...args: Parameters<T>) => {
    if (!inThrottle) {
      func(...args)
      inThrottle = true
      setTimeout(() => inThrottle = false, limit)
    }
  }
}

// Local Storage
export function getStorageItem(key: string, defaultValue: any = null): any {
  try {
    if (typeof window === 'undefined') return defaultValue
    const item = localStorage.getItem(key)
    return item ? JSON.parse(item) : defaultValue
  } catch {
    return defaultValue
  }
}

export function setStorageItem(key: string, value: any): void {
  try {
    if (typeof window !== 'undefined') {
      localStorage.setItem(key, JSON.stringify(value))
    }
  } catch {
    // Ignore localStorage errors
  }
}

// URL Utilities
export function buildApiUrl(endpoint: string): string {
  const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'
  const cleanEndpoint = endpoint.startsWith('/') ? endpoint : `/${endpoint}`
  return `${baseUrl}/api/v1${cleanEndpoint}`
}