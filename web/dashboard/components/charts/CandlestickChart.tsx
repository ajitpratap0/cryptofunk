'use client'

import { useEffect, useRef } from 'react'
import { createChart, IChartApi, ISeriesApi, CandlestickData, Time, UTCTimestamp } from 'lightweight-charts'
import { CandlestickData as CustomCandlestickData, TradeMarker } from '@/lib/types'
import { cn } from '@/lib/utils'

interface CandlestickChartProps {
  data: CustomCandlestickData[]
  tradeMarkers?: TradeMarker[]
  symbol?: string
  timeframe?: string
  height?: number
  loading?: boolean
  className?: string
  onCrosshairMove?: (data: any) => void
}

export function CandlestickChart({
  data,
  tradeMarkers = [],
  symbol = '',
  timeframe = '5m',
  height = 400,
  loading = false,
  className,
  onCrosshairMove
}: CandlestickChartProps) {
  const chartContainerRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<IChartApi | null>(null)
  const seriesRef = useRef<ISeriesApi<"Candlestick"> | null>(null)

  // Single effect: create chart, set data, handle cleanup
  useEffect(() => {
    if (!chartContainerRef.current) return

    const container = chartContainerRef.current
    const containerWidth = container.clientWidth || 800

    const chart = createChart(container, {
      width: containerWidth,
      height,
      layout: {
        background: { color: 'transparent' },
        textColor: '#6B7280',
      },
      grid: {
        vertLines: { color: '#374151' },
        horzLines: { color: '#374151' },
      },
      crosshair: {
        mode: 1,
        vertLine: { width: 1, color: '#6B7280', style: 2 },
        horzLine: { width: 1, color: '#6B7280', style: 2 },
      },
      rightPriceScale: { borderColor: '#374151' },
      timeScale: {
        borderColor: '#374151',
        timeVisible: true,
        secondsVisible: false,
      },
      handleScroll: { mouseWheel: true, pressedMouseMove: true },
      handleScale: { axisPressedMouseMove: true, mouseWheel: true, pinch: true },
    })

    const series = chart.addCandlestickSeries({
      upColor: '#22C55E',
      downColor: '#EF4444',
      borderDownColor: '#EF4444',
      borderUpColor: '#22C55E',
      wickDownColor: '#EF4444',
      wickUpColor: '#22C55E',
    })

    chartRef.current = chart
    seriesRef.current = series

    // Set data if available
    if (data.length > 0) {
      const chartData: CandlestickData<Time>[] = data.map(item => ({
        time: (new Date(item.time).getTime() / 1000) as UTCTimestamp,
        open: item.open,
        high: item.high,
        low: item.low,
        close: item.close,
      }))
      series.setData(chartData)
      chart.timeScale().fitContent()
    }

    // Set trade markers
    if (tradeMarkers.length > 0) {
      const markers = tradeMarkers.map(marker => ({
        time: (new Date(marker.time).getTime() / 1000) as UTCTimestamp,
        position: marker.position,
        color: marker.color,
        shape: marker.shape,
        text: marker.text,
        size: marker.size,
      }))
      series.setMarkers(markers as any)
    }

    // Crosshair
    if (onCrosshairMove) {
      chart.subscribeCrosshairMove(onCrosshairMove)
    }

    // Resize handler
    const handleResize = () => {
      if (container && chart) {
        chart.applyOptions({ width: container.clientWidth, height })
      }
    }
    window.addEventListener('resize', handleResize)

    return () => {
      window.removeEventListener('resize', handleResize)
      chart.remove()
      chartRef.current = null
      seriesRef.current = null
    }
  }, [data, tradeMarkers, height, onCrosshairMove])

  if (loading) {
    return (
      <div className={cn("chart-container", className)} style={{ height }}>
        <div className="chart-loading">
          <div className="flex items-center gap-2">
            <div className="w-4 h-4 border-2 border-muted-foreground border-t-transparent rounded-full animate-spin" />
            <span className="text-sm text-muted-foreground">Loading chart...</span>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className={cn("relative w-full overflow-hidden", className)}>
      {/* Chart Header */}
      <div className="absolute top-4 left-4 z-10 bg-card/80 backdrop-blur-sm rounded-lg p-3 border border-border">
        <div className="flex items-center gap-3">
          <h3 className="font-semibold text-sm">{symbol}</h3>
          {timeframe && (
            <span className="text-xs text-muted-foreground bg-muted px-2 py-1 rounded">
              {timeframe.toUpperCase()}
            </span>
          )}
        </div>
        
        {data.length > 0 && (
          <div className="mt-2 grid grid-cols-4 gap-3 text-xs">
            <div>
              <span className="text-muted-foreground">O</span>
              <span className="ml-1 font-mono">{data[data.length - 1]?.open.toFixed(2)}</span>
            </div>
            <div>
              <span className="text-muted-foreground">H</span>
              <span className="ml-1 font-mono">{data[data.length - 1]?.high.toFixed(2)}</span>
            </div>
            <div>
              <span className="text-muted-foreground">L</span>
              <span className="ml-1 font-mono">{data[data.length - 1]?.low.toFixed(2)}</span>
            </div>
            <div>
              <span className="text-muted-foreground">C</span>
              <span className={cn(
                "ml-1 font-mono",
                data[data.length - 1]?.close > data[data.length - 1]?.open 
                  ? "text-profit" 
                  : "text-loss"
              )}>
                {data[data.length - 1]?.close.toFixed(2)}
              </span>
            </div>
          </div>
        )}
      </div>

      {/* Chart Container */}
      <div 
        ref={chartContainerRef} 
        style={{ height }}
        className="w-full"
      />

      {/* Legend */}
      {tradeMarkers.length > 0 && (
        <div className="absolute bottom-4 right-4 bg-card/80 backdrop-blur-sm rounded-lg p-3 border border-border">
          <h4 className="text-xs font-medium text-muted-foreground mb-2">Trade Markers</h4>
          <div className="space-y-1">
            <div className="flex items-center gap-2 text-xs">
              <div className="w-3 h-3 bg-profit rounded-full" />
              <span>Long Entry</span>
            </div>
            <div className="flex items-center gap-2 text-xs">
              <div className="w-3 h-3 bg-loss rounded-full" />
              <span>Short Entry</span>
            </div>
            <div className="flex items-center gap-2 text-xs">
              <div className="w-3 h-3 bg-warning rounded-full" />
              <span>Exit</span>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
