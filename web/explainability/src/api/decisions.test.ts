import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  listDecisions,
  getDecision,
  searchDecisions,
  getDecisionStats,
  DecisionApiError,
} from './decisions';

// Mock fetch
const mockFetch = vi.fn();
global.fetch = mockFetch;

describe('Decision API Client', () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  describe('listDecisions', () => {
    it('should fetch decisions with default parameters', async () => {
      const mockResponse = {
        decisions: [
          {
            id: '123',
            decision_type: 'signal',
            symbol: 'BTC/USDT',
            prompt: 'Test prompt',
            response: 'Test response',
            model: 'claude',
            created_at: '2024-01-01T00:00:00Z',
          },
        ],
        count: 1,
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const result = await listDecisions();

      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/decisions',
        expect.objectContaining({
          method: 'GET',
          headers: { 'Content-Type': 'application/json' },
        })
      );
      expect(result).toEqual(mockResponse);
    });

    it('should include filter parameters in query string', async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve({ decisions: [], count: 0 }),
      });

      await listDecisions({
        symbol: 'ETH/USDT',
        outcome: 'SUCCESS',
        limit: 10,
      });

      const callUrl = mockFetch.mock.calls[0][0];
      expect(callUrl).toContain('symbol=ETH%2FUSDT');
      expect(callUrl).toContain('outcome=SUCCESS');
      expect(callUrl).toContain('limit=10');
    });

    it('should throw DecisionApiError on HTTP error', async () => {
      const errorResponse = {
        ok: false,
        status: 500,
        statusText: 'Internal Server Error',
        json: () => Promise.resolve({ message: 'Database error' }),
      };
      mockFetch.mockResolvedValue(errorResponse);

      await expect(listDecisions()).rejects.toThrow(DecisionApiError);
      await expect(listDecisions()).rejects.toThrow('Database error');
    });
  });

  describe('getDecision', () => {
    it('should fetch a single decision by ID', async () => {
      const mockDecision = {
        id: '123',
        decision_type: 'signal',
        symbol: 'BTC/USDT',
        prompt: 'Test prompt',
        response: 'Test response',
        model: 'claude',
        created_at: '2024-01-01T00:00:00Z',
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockDecision),
      });

      const result = await getDecision('123');

      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/decisions/123',
        expect.objectContaining({
          method: 'GET',
        })
      );
      expect(result).toEqual(mockDecision);
    });
  });

  describe('searchDecisions', () => {
    it('should POST search query with JSON body', async () => {
      const mockResponse = {
        results: [
          {
            decision: {
              id: '123',
              decision_type: 'signal',
              symbol: 'BTC/USDT',
              prompt: 'Why did you buy BTC?',
              response: 'Market indicators positive',
              model: 'claude',
              created_at: '2024-01-01T00:00:00Z',
            },
            score: 0.95,
          },
        ],
        count: 1,
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockResponse),
      });

      const result = await searchDecisions('buy BTC', 10);

      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/decisions/search',
        expect.objectContaining({
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ query: 'buy BTC', limit: 10 }),
        })
      );
      expect(result).toEqual(mockResponse);
    });
  });

  describe('getDecisionStats', () => {
    it('should fetch decision statistics', async () => {
      const mockStats = {
        total_decisions: 100,
        by_type: { signal: 50, risk_approval: 30, position_sizing: 20 },
        by_outcome: { SUCCESS: 60, FAILURE: 30, PENDING: 10 },
        by_model: { claude: 80, 'gpt-4': 20 },
        avg_confidence: 0.75,
        avg_latency_ms: 250,
        avg_tokens_used: 500,
        success_rate: 0.6,
        total_pnl: 1500.5,
        avg_pnl: 15.0,
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: () => Promise.resolve(mockStats),
      });

      const result = await getDecisionStats();

      expect(mockFetch).toHaveBeenCalledWith(
        '/api/v1/decisions/stats',
        expect.objectContaining({
          method: 'GET',
        })
      );
      expect(result).toEqual(mockStats);
    });
  });
});
