/**
 * End-to-End Tests for Explainability Dashboard
 *
 * Tests the complete user flow across the dashboard including:
 * - Dashboard loading and navigation
 * - Timeline interactions
 * - Search functionality
 * - Filters
 * - Statistics view
 * - Error handling
 */

import { describe, it, expect, vi, beforeEach, afterEach, type Mock } from 'vitest';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import App from '../App';
import type { Decision, DecisionStats, SearchResult } from '../types';

// Mock the hooks
vi.mock('../hooks/useDecisions', () => ({
  useInfiniteDecisions: vi.fn(),
  useDecision: vi.fn(),
  useSearchDecisions: vi.fn(),
  useDecisionStats: vi.fn(),
}));

import {
  useInfiniteDecisions,
  useDecision,
  useSearchDecisions,
  useDecisionStats
} from '../hooks/useDecisions';

// Mock data
const mockDecisions: Decision[] = [
  {
    id: 'dec-1',
    decision_type: 'BUY_SIGNAL',
    symbol: 'BTC/USDT',
    agent_name: 'Technical Agent',
    prompt: 'Analyze BTC market conditions for potential buy signal',
    response: 'Strong bullish signals detected. RSI oversold at 32, MACD showing positive divergence, price bouncing off support at $65,000.',
    model: 'claude-sonnet-4.5',
    tokens_used: 587,
    latency_ms: 245,
    confidence: 0.87,
    outcome: 'SUCCESS',
    pnl: 1250.50,
    created_at: new Date('2024-11-26T10:30:00Z').toISOString(),
  },
  {
    id: 'dec-2',
    decision_type: 'SELL_SIGNAL',
    symbol: 'ETH/USDT',
    agent_name: 'Trend Agent',
    prompt: 'Analyze ETH trend for potential sell signal',
    response: 'Bearish trend confirmed. Moving averages showing death cross, price breaking below key support levels.',
    model: 'gpt-4',
    tokens_used: 423,
    latency_ms: 189,
    confidence: 0.72,
    outcome: 'FAILURE',
    pnl: -320.00,
    created_at: new Date('2024-11-26T09:15:00Z').toISOString(),
  },
  {
    id: 'dec-3',
    decision_type: 'RISK_APPROVAL',
    symbol: 'SOL/USDT',
    agent_name: 'Risk Agent',
    prompt: 'Evaluate risk for SOL position',
    response: 'Risk within acceptable parameters. Position size 2.5% of portfolio, stop-loss at 3%.',
    model: 'claude-sonnet-4.5',
    tokens_used: 312,
    latency_ms: 156,
    confidence: 0.91,
    outcome: 'SUCCESS',
    pnl: 85.25,
    created_at: new Date('2024-11-26T08:45:00Z').toISOString(),
  },
];

const mockStats: DecisionStats = {
  total_decisions: 150,
  by_type: {
    BUY_SIGNAL: 45,
    SELL_SIGNAL: 42,
    RISK_APPROVAL: 38,
    POSITION_SIZING: 25,
  },
  by_outcome: {
    SUCCESS: 95,
    FAILURE: 40,
    PENDING: 15,
  },
  by_model: {
    'claude-sonnet-4.5': 90,
    'gpt-4': 60,
  },
  avg_confidence: 0.78,
  avg_latency_ms: 215,
  avg_tokens_used: 456,
  success_rate: 0.63,
  total_pnl: 5420.50,
  avg_pnl: 36.14,
};

const mockSearchResults: SearchResult[] = [
  {
    decision: mockDecisions[0],
    score: 0.95,
  },
  {
    decision: mockDecisions[2],
    score: 0.82,
  },
];

// Helper to create a fresh QueryClient for each test
const createTestQueryClient = () => new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
      gcTime: 0,
    },
    mutations: {
      retry: false,
    },
  },
});

// Helper to render with providers
const renderApp = () => {
  const queryClient = createTestQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  );
};

describe('Explainability Dashboard E2E', () => {
  const mockRefetch = vi.fn();
  const mockFetchNextPage = vi.fn();
  const mockSearchMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();

    // Default mock implementations
    (useInfiniteDecisions as Mock).mockReturnValue({
      data: { pages: [{ decisions: mockDecisions, count: mockDecisions.length }] },
      isLoading: false,
      isFetching: false,
      error: null,
      refetch: mockRefetch,
      fetchNextPage: mockFetchNextPage,
      hasNextPage: false,
      isFetchingNextPage: false,
    });

    (useDecision as Mock).mockImplementation((id: string) => ({
      data: mockDecisions.find(d => d.id === id),
      isLoading: false,
      error: null,
    }));

    (useSearchDecisions as Mock).mockReturnValue({
      mutate: mockSearchMutate,
      data: null,
      isPending: false,
      error: null,
    });

    (useDecisionStats as Mock).mockReturnValue({
      data: mockStats,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    });
  });

  afterEach(() => {
    vi.clearAllTimers();
  });

  describe('1. Dashboard loads successfully', () => {
    it('renders the dashboard header and navigation', () => {
      renderApp();

      expect(screen.getByText('CryptoFunk Explainability Dashboard')).toBeInTheDocument();
      expect(screen.getByText('Multi-Agent AI Trading System Decision Analysis')).toBeInTheDocument();
      expect(screen.getByText('Timeline')).toBeInTheDocument();
      expect(screen.getByText('Statistics')).toBeInTheDocument();
      expect(screen.getByText('Search')).toBeInTheDocument();
    });

    it('displays decisions in the timeline by default', () => {
      renderApp();

      expect(screen.getByText('Technical Agent')).toBeInTheDocument();
      expect(screen.getByText('Trend Agent')).toBeInTheDocument();
      expect(screen.getByText('Risk Agent')).toBeInTheDocument();
      expect(screen.getByText('BTC/USDT')).toBeInTheDocument();
      expect(screen.getByText('ETH/USDT')).toBeInTheDocument();
      expect(screen.getByText('SOL/USDT')).toBeInTheDocument();
    });

    it('shows the correct decision count', () => {
      renderApp();

      expect(screen.getByText(/Showing 3 decisions/i)).toBeInTheDocument();
    });
  });

  describe('2. Timeline interactions', () => {
    it('can scroll through decision timeline using Load More', () => {
      (useInfiniteDecisions as Mock).mockReturnValue({
        data: { pages: [{ decisions: mockDecisions }] },
        isLoading: false,
        isFetching: false,
        error: null,
        refetch: mockRefetch,
        fetchNextPage: mockFetchNextPage,
        hasNextPage: true,
        isFetchingNextPage: false,
      });

      renderApp();

      const loadMoreButton = screen.getByRole('button', { name: /load more/i });
      expect(loadMoreButton).toBeInTheDocument();

      fireEvent.click(loadMoreButton);
      expect(mockFetchNextPage).toHaveBeenCalledTimes(1);
    });

    it('can click on a decision to select it', async () => {
      renderApp();

      // Find and click the first decision card (it's an article with role="button")
      const firstCard = screen.getByRole('button', { name: /technical agent/i });
      expect(firstCard).toBeInTheDocument();

      fireEvent.click(firstCard);

      // The card should show as selected
      await waitFor(() => {
        expect(firstCard).toHaveClass('border-blue-500');
      });
    });

    it('displays loading skeletons while loading', () => {
      (useInfiniteDecisions as Mock).mockReturnValue({
        data: null,
        isLoading: true,
        isFetching: true,
        error: null,
        refetch: mockRefetch,
        fetchNextPage: mockFetchNextPage,
        hasNextPage: false,
        isFetchingNextPage: false,
      });

      renderApp();

      const skeletons = document.querySelectorAll('.animate-pulse');
      expect(skeletons.length).toBeGreaterThan(0);
    });
  });

  describe('3. Search functionality', () => {
    it('can navigate to search tab', () => {
      renderApp();

      const searchTab = screen.getByRole('button', { name: 'Search' });
      fireEvent.click(searchTab);

      expect(screen.getByText('Semantic Search')).toBeInTheDocument();
      expect(screen.getByPlaceholderText('Why did you buy BTC?')).toBeInTheDocument();
    });

    it('can enter search query and search', async () => {
      renderApp();

      // Navigate to search tab
      const searchTab = screen.getByRole('button', { name: 'Search' });
      fireEvent.click(searchTab);

      // Enter search query
      const searchInput = screen.getByPlaceholderText('Why did you buy BTC?');
      fireEvent.change(searchInput, { target: { value: 'bullish signals BTC' } });

      // Click search button (within SearchView, has an icon)
      const buttons = screen.getAllByRole('button');
      const searchButton = buttons.find(btn => btn.textContent?.includes('Search') && btn.querySelector('svg'));
      expect(searchButton).toBeDefined();

      if (searchButton) {
        fireEvent.click(searchButton);
      }

      expect(mockSearchMutate).toHaveBeenCalledWith({
        query: 'bullish signals BTC',
        limit: 20,
      });
    });

    it('can search by pressing Enter key', async () => {
      renderApp();

      // Navigate to search
      fireEvent.click(screen.getByRole('button', { name: 'Search' }));

      // Enter search query and press Enter
      const searchInput = screen.getByPlaceholderText('Why did you buy BTC?');
      fireEvent.change(searchInput, { target: { value: 'risk analysis' } });
      fireEvent.keyPress(searchInput, { key: 'Enter', code: 'Enter', charCode: 13 });

      expect(mockSearchMutate).toHaveBeenCalledWith({
        query: 'risk analysis',
        limit: 20,
      });
    });

    it('search shows relevant results with scores', async () => {
      (useSearchDecisions as Mock).mockReturnValue({
        mutate: mockSearchMutate,
        data: { results: mockSearchResults, count: mockSearchResults.length },
        isPending: false,
        error: null,
      });

      renderApp();

      // Navigate to search
      fireEvent.click(screen.getByRole('button', { name: 'Search' }));

      // Results should be displayed
      await waitFor(() => {
        expect(screen.getByText(/Found 2 similar decision/i)).toBeInTheDocument();
        expect(screen.getByText('95.0% match')).toBeInTheDocument();
        expect(screen.getByText('82.0% match')).toBeInTheDocument();
      });
    });

    it('shows no results message when search returns empty', () => {
      (useSearchDecisions as Mock).mockReturnValue({
        mutate: mockSearchMutate,
        data: { results: [], count: 0 },
        isPending: false,
        error: null,
      });

      renderApp();

      // Navigate to search
      fireEvent.click(screen.getByRole('button', { name: 'Search' }));

      expect(screen.getByText('No similar decisions found for your query.')).toBeInTheDocument();
    });
  });

  describe('4. Filters', () => {
    it('can filter by agent name', async () => {
      renderApp();

      const agentInput = screen.getByLabelText('Agent');
      fireEvent.change(agentInput, { target: { value: 'Technical' } });

      await waitFor(() => {
        expect(agentInput).toHaveValue('Technical');
      });
    });

    it('can filter by symbol', async () => {
      renderApp();

      const symbolInput = screen.getByLabelText('Symbol');
      fireEvent.change(symbolInput, { target: { value: 'BTC' } });

      await waitFor(() => {
        expect(symbolInput).toHaveValue('BTC');
      });
    });

    it('can filter by outcome', async () => {
      renderApp();

      const outcomeSelect = screen.getByLabelText('Outcome');
      fireEvent.change(outcomeSelect, { target: { value: 'SUCCESS' } });

      await waitFor(() => {
        expect(outcomeSelect).toHaveValue('SUCCESS');
      });
    });

    it('can clear all filters', async () => {
      renderApp();

      // Set filter values
      const agentInput = screen.getByLabelText('Agent');
      const symbolInput = screen.getByLabelText('Symbol');
      const outcomeSelect = screen.getByLabelText('Outcome');

      fireEvent.change(agentInput, { target: { value: 'Technical' } });
      fireEvent.change(symbolInput, { target: { value: 'BTC' } });
      fireEvent.change(outcomeSelect, { target: { value: 'SUCCESS' } });

      // Click Clear Filters
      const clearButton = screen.getByRole('button', { name: /clear.*filters/i });
      fireEvent.click(clearButton);

      await waitFor(() => {
        expect(agentInput).toHaveValue('');
        expect(symbolInput).toHaveValue('');
        expect(outcomeSelect).toHaveValue('');
      });
    });
  });

  describe('5. Stats view', () => {
    it('displays statistics correctly', () => {
      renderApp();

      // Navigate to stats
      const statsTab = screen.getByRole('button', { name: 'Statistics' });
      fireEvent.click(statsTab);

      // Check for key stat headers to confirm stats are displayed
      expect(screen.getByText('Total Decisions')).toBeInTheDocument();
      expect(screen.getByText('Success Rate')).toBeInTheDocument();
      expect(screen.getByText('Avg Confidence')).toBeInTheDocument();
      expect(screen.getByText('Total P&L')).toBeInTheDocument();

      // Check for specific stat values
      expect(screen.getByText('150')).toBeInTheDocument(); // total decisions
      expect(screen.getByText('63.0%')).toBeInTheDocument(); // success rate
      expect(screen.getByText('78.0%')).toBeInTheDocument(); // avg confidence
    });

    it('can refresh statistics', () => {
      const mockStatsRefetch = vi.fn();
      (useDecisionStats as Mock).mockReturnValue({
        data: mockStats,
        isLoading: false,
        error: null,
        refetch: mockStatsRefetch,
      });

      renderApp();

      // Navigate to stats
      fireEvent.click(screen.getByRole('button', { name: 'Statistics' }));

      // Find refresh button by its text content "Refresh"
      const refreshButton = screen.getByRole('button', { name: /refresh/i });
      fireEvent.click(refreshButton);

      expect(mockStatsRefetch).toHaveBeenCalledTimes(1);
    });

    it('shows loading state while fetching stats', () => {
      (useDecisionStats as Mock).mockReturnValue({
        data: null,
        isLoading: true,
        error: null,
        refetch: vi.fn(),
      });

      renderApp();

      // Navigate to stats
      fireEvent.click(screen.getByRole('button', { name: 'Statistics' }));

      // Should show loading skeletons
      const skeletons = document.querySelectorAll('.animate-pulse');
      expect(skeletons.length).toBeGreaterThan(0);
    });
  });

  describe('6. Pagination/infinite scroll', () => {
    it('loads more decisions when clicking Load More', () => {
      (useInfiniteDecisions as Mock).mockReturnValue({
        data: { pages: [{ decisions: mockDecisions }] },
        isLoading: false,
        isFetching: false,
        error: null,
        refetch: mockRefetch,
        fetchNextPage: mockFetchNextPage,
        hasNextPage: true,
        isFetchingNextPage: false,
      });

      renderApp();

      const loadMoreButton = screen.getByRole('button', { name: /load more/i });
      expect(loadMoreButton).toBeInTheDocument();

      fireEvent.click(loadMoreButton);
      expect(mockFetchNextPage).toHaveBeenCalledTimes(1);
    });

    it('shows loading state while fetching next page', () => {
      (useInfiniteDecisions as Mock).mockReturnValue({
        data: { pages: [{ decisions: mockDecisions }] },
        isLoading: false,
        isFetching: true,
        error: null,
        refetch: mockRefetch,
        fetchNextPage: mockFetchNextPage,
        hasNextPage: true,
        isFetchingNextPage: true,
      });

      renderApp();

      expect(screen.getByText('Loading...')).toBeInTheDocument();
    });

    it('hides Load More button when no more pages', () => {
      (useInfiniteDecisions as Mock).mockReturnValue({
        data: { pages: [{ decisions: mockDecisions }] },
        isLoading: false,
        isFetching: false,
        error: null,
        refetch: mockRefetch,
        fetchNextPage: mockFetchNextPage,
        hasNextPage: false,
        isFetchingNextPage: false,
      });

      renderApp();

      expect(screen.queryByRole('button', { name: /load more/i })).not.toBeInTheDocument();
      expect(screen.getByText(/all loaded/i)).toBeInTheDocument();
    });

    it('shows correct count across multiple pages', () => {
      const page1 = mockDecisions.slice(0, 2);
      const page2 = [mockDecisions[2]];

      (useInfiniteDecisions as Mock).mockReturnValue({
        data: {
          pages: [
            { decisions: page1 },
            { decisions: page2 }
          ]
        },
        isLoading: false,
        isFetching: false,
        error: null,
        refetch: mockRefetch,
        fetchNextPage: mockFetchNextPage,
        hasNextPage: false,
        isFetchingNextPage: false,
      });

      renderApp();

      // Should show total count across all pages
      expect(screen.getByText(/Showing 3 decisions/i)).toBeInTheDocument();
    });
  });

  describe('7. Error handling', () => {
    it('displays error state when timeline fails to load', () => {
      (useInfiniteDecisions as Mock).mockReturnValue({
        data: null,
        isLoading: false,
        isFetching: false,
        error: new Error('Network error'),
        refetch: mockRefetch,
        fetchNextPage: mockFetchNextPage,
        hasNextPage: false,
        isFetchingNextPage: false,
      });

      renderApp();

      expect(screen.getByText('Error loading decisions')).toBeInTheDocument();
      expect(screen.getByText('Network error')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
    });

    it('can retry after error', () => {
      (useInfiniteDecisions as Mock).mockReturnValue({
        data: null,
        isLoading: false,
        isFetching: false,
        error: new Error('Network error'),
        refetch: mockRefetch,
        fetchNextPage: mockFetchNextPage,
        hasNextPage: false,
        isFetchingNextPage: false,
      });

      renderApp();

      const retryButton = screen.getByRole('button', { name: /retry/i });
      fireEvent.click(retryButton);

      expect(mockRefetch).toHaveBeenCalledTimes(1);
    });

    it('displays error state in stats view', () => {
      (useDecisionStats as Mock).mockReturnValue({
        data: null,
        isLoading: false,
        error: new Error('Failed to load statistics'),
        refetch: vi.fn(),
      });

      renderApp();

      // Navigate to stats
      fireEvent.click(screen.getByRole('button', { name: 'Statistics' }));

      // Error is displayed (component shows error but doesn't use role="alert")
      expect(screen.getByText(/Error loading statistics/i)).toBeInTheDocument();
      expect(screen.getByText(/Failed to load statistics/i)).toBeInTheDocument();
    });

    it('displays error in search', () => {
      (useSearchDecisions as Mock).mockReturnValue({
        mutate: mockSearchMutate,
        data: null,
        isPending: false,
        error: new Error('Search service unavailable'),
      });

      renderApp();

      // Navigate to search
      fireEvent.click(screen.getByRole('button', { name: 'Search' }));

      expect(screen.getByText(/Search service unavailable/i)).toBeInTheDocument();
    });

    it('shows empty state when no decisions exist', () => {
      (useInfiniteDecisions as Mock).mockReturnValue({
        data: { pages: [{ decisions: [] }] },
        isLoading: false,
        isFetching: false,
        error: null,
        refetch: mockRefetch,
        fetchNextPage: mockFetchNextPage,
        hasNextPage: false,
        isFetchingNextPage: false,
      });

      renderApp();

      expect(screen.getByText('No decisions found')).toBeInTheDocument();
      expect(screen.getByText(/Try adjusting your filters/i)).toBeInTheDocument();
    });
  });

  describe('8. Navigation and state management', () => {
    it('preserves selected decision when switching tabs', async () => {
      renderApp();

      // Select a decision in timeline
      const card = screen.getByRole('button', { name: /technical agent/i });
      fireEvent.click(card);

      await waitFor(() => {
        expect(card).toHaveClass('border-blue-500');
      });

      // Switch to stats tab
      fireEvent.click(screen.getByRole('button', { name: 'Statistics' }));

      // Switch back to timeline
      fireEvent.click(screen.getByRole('button', { name: 'Timeline' }));

      // The card should still be marked as selected
      const selectedCard = screen.getByRole('button', { name: /technical agent/i });
      expect(selectedCard).toHaveClass('border-blue-500');
    });

    it('filter state resets when switching tabs (expected behavior)', async () => {
      renderApp();

      // Set filters
      const agentInput = screen.getByLabelText('Agent');
      fireEvent.change(agentInput, { target: { value: 'Technical' } });

      // Verify filter is set
      expect(agentInput).toHaveValue('Technical');

      // Switch tabs
      fireEvent.click(screen.getByRole('button', { name: 'Search' }));
      fireEvent.click(screen.getByRole('button', { name: 'Timeline' }));

      // Filter resets because DecisionTimeline is unmounted/remounted
      await waitFor(() => {
        const agentInputAfter = screen.getByLabelText('Agent');
        expect(agentInputAfter).toHaveValue('');
      });
    });
  });

  describe('9. Accessibility', () => {
    it('timeline has proper ARIA attributes', () => {
      renderApp();

      expect(screen.getByRole('region', { name: /decision timeline/i })).toBeInTheDocument();
      expect(screen.getByRole('list')).toBeInTheDocument();
    });

    it('filter inputs have proper labels and descriptions', () => {
      renderApp();

      const agentInput = screen.getByLabelText('Agent');
      const symbolInput = screen.getByLabelText('Symbol');
      const outcomeSelect = screen.getByLabelText('Outcome');

      expect(agentInput).toHaveAttribute('aria-describedby');
      expect(symbolInput).toHaveAttribute('aria-describedby');
      expect(outcomeSelect).toHaveAttribute('aria-describedby');
    });

    it('error state has proper alert role', () => {
      (useInfiniteDecisions as Mock).mockReturnValue({
        data: null,
        isLoading: false,
        isFetching: false,
        error: new Error('Failed to fetch'),
        refetch: mockRefetch,
        fetchNextPage: mockFetchNextPage,
        hasNextPage: false,
        isFetchingNextPage: false,
      });

      renderApp();

      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
  });
});
