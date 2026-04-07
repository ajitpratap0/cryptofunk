import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// These tests verify the X-API-Key auth header plumbing introduced in
// PR #201. A regression here silently breaks the entire dashboard when
// the backend has auth.enabled=true, so the contract is worth pinning.

const ORIGINAL_KEY = process.env.NEXT_PUBLIC_API_KEY
const ORIGINAL_URL = process.env.NEXT_PUBLIC_API_URL

async function loadApi() {
  vi.resetModules()
  return await import('@/lib/api')
}

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
}

describe('apiClient X-API-Key header', () => {
  beforeEach(() => {
    process.env.NEXT_PUBLIC_API_URL = 'http://test.local'
  })

  afterEach(() => {
    if (ORIGINAL_KEY === undefined) delete process.env.NEXT_PUBLIC_API_KEY
    else process.env.NEXT_PUBLIC_API_KEY = ORIGINAL_KEY
    if (ORIGINAL_URL === undefined) delete process.env.NEXT_PUBLIC_API_URL
    else process.env.NEXT_PUBLIC_API_URL = ORIGINAL_URL
    vi.restoreAllMocks()
  })

  it('attaches X-API-Key when NEXT_PUBLIC_API_KEY is set', async () => {
    process.env.NEXT_PUBLIC_API_KEY = 'test-key-123'
    const fetchSpy = vi
      .spyOn(global, 'fetch')
      .mockResolvedValue(jsonResponse({ success: true, data: { status: 'ok' } }))

    const { apiClient } = await loadApi()
    await apiClient.getHealth()

    expect(fetchSpy).toHaveBeenCalledTimes(1)
    const init = fetchSpy.mock.calls[0]![1] as RequestInit
    const headers = init.headers as Record<string, string>
    expect(headers['X-API-Key']).toBe('test-key-123')
  })

  it('omits X-API-Key when NEXT_PUBLIC_API_KEY is unset', async () => {
    delete process.env.NEXT_PUBLIC_API_KEY
    const fetchSpy = vi
      .spyOn(global, 'fetch')
      .mockResolvedValue(jsonResponse({ success: true, data: { status: 'ok' } }))

    const { apiClient } = await loadApi()
    await apiClient.getHealth()

    const init = fetchSpy.mock.calls[0]![1] as RequestInit
    const headers = init.headers as Record<string, string>
    expect(headers['X-API-Key']).toBeUndefined()
  })

  it('caller-supplied headers cannot drop X-API-Key', async () => {
    process.env.NEXT_PUBLIC_API_KEY = 'guarded'
    const fetchSpy = vi
      .spyOn(global, 'fetch')
      .mockResolvedValue(jsonResponse({ success: true, data: {} }))

    const { apiClient } = await loadApi()
    // createOrder spreads `options` (with body) and merges headers; verify
    // the auth header still ends up on the request.
    await apiClient.createOrder({ symbol: 'BTC/USDT' })

    const init = fetchSpy.mock.calls[0]![1] as RequestInit
    const headers = init.headers as Record<string, string>
    expect(headers['X-API-Key']).toBe('guarded')
  })

  it('returns success:false with a friendly message on 401', async () => {
    process.env.NEXT_PUBLIC_API_KEY = 'wrong'
    vi.spyOn(global, 'fetch').mockResolvedValue(
      new Response('unauthorized', { status: 401 })
    )

    const { apiClient } = await loadApi()
    const res = await apiClient.getHealth()

    expect(res.success).toBe(false)
    expect(res.error).toMatch(/NEXT_PUBLIC_API_KEY/)
  })
})

describe('getMarketCandlestick URL encoding', () => {
  beforeEach(() => {
    process.env.NEXT_PUBLIC_API_URL = 'http://test.local'
    delete process.env.NEXT_PUBLIC_API_KEY
  })

  afterEach(() => {
    if (ORIGINAL_KEY === undefined) delete process.env.NEXT_PUBLIC_API_KEY
    else process.env.NEXT_PUBLIC_API_KEY = ORIGINAL_KEY
    vi.restoreAllMocks()
  })

  it('percent-encodes symbols containing slashes', async () => {
    const fetchSpy = vi
      .spyOn(global, 'fetch')
      .mockResolvedValue(jsonResponse({ success: true, data: [] }))

    const { apiClient } = await loadApi()
    await apiClient.getMarketCandlestick('BTC/USDT', '5m')

    const url = fetchSpy.mock.calls[0]![0] as string
    expect(url).toContain('/market/candlestick/BTC%2FUSDT')
    expect(url).toContain('timeRange=5m')
  })
})

describe('decision analytics fetchers', () => {
  beforeEach(() => {
    process.env.NEXT_PUBLIC_API_URL = 'http://test.local'
    process.env.NEXT_PUBLIC_API_KEY = 'k'
  })

  afterEach(() => {
    if (ORIGINAL_KEY === undefined) delete process.env.NEXT_PUBLIC_API_KEY
    else process.env.NEXT_PUBLIC_API_KEY = ORIGINAL_KEY
    vi.restoreAllMocks()
  })

  it('fetchDecisionAnalytics surfaces 500 as success:false instead of returning the error body', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(
      new Response('boom', { status: 500 })
    )

    const { fetchDecisionAnalytics } = await loadApi()
    const res = await fetchDecisionAnalytics('30d')

    expect(res.success).toBe(false)
    expect(res.error).toMatch(/500/)
  })

  it('triggerOutcomeResolution sends X-API-Key on POST', async () => {
    const fetchSpy = vi
      .spyOn(global, 'fetch')
      .mockResolvedValue(jsonResponse({ success: true, data: { polymarket_resolved: 0, binance_resolved: 0 } }))

    const { triggerOutcomeResolution } = await loadApi()
    await triggerOutcomeResolution()

    const init = fetchSpy.mock.calls[0]![1] as RequestInit
    expect(init.method).toBe('POST')
    expect((init.headers as Record<string, string>)['X-API-Key']).toBe('k')
  })
})
