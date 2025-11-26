import { describe, it, expect, vi, beforeEach, type Mock } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import DecisionTimeline from './DecisionTimeline';
import type { Decision } from '../types';

// Mock the useInfiniteDecisions hook
vi.mock('../hooks/useDecisions', () => ({
  useInfiniteDecisions: vi.fn(),
}));

import { useInfiniteDecisions } from '../hooks/useDecisions';

const mockDecisions: Decision[] = [
  {
    id: '1',
    decision_type: 'BUY_SIGNAL',
    symbol: 'BTC/USDT',
    agent_name: 'Technical Agent',
    prompt: 'Analyze BTC',
    response: 'Strong bullish signals',
    model: 'claude-sonnet',
    tokens_used: 500,
    latency_ms: 250,
    confidence: 0.85,
    outcome: 'SUCCESS',
    pnl: 150.5,
    created_at: new Date().toISOString(),
  },
  {
    id: '2',
    decision_type: 'SELL_SIGNAL',
    symbol: 'ETH/USDT',
    agent_name: 'Risk Agent',
    prompt: 'Analyze ETH',
    response: 'Risk threshold exceeded',
    model: 'gpt-4',
    tokens_used: 300,
    latency_ms: 200,
    confidence: 0.72,
    outcome: 'FAILURE',
    pnl: -50.0,
    created_at: new Date().toISOString(),
  },
];

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
};

describe('DecisionTimeline', () => {
  const mockOnSelectDecision = vi.fn();
  const mockRefetch = vi.fn();
  const mockFetchNextPage = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
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
  });

  it('renders the timeline with decisions', () => {
    render(
      <DecisionTimeline
        selectedDecision={null}
        onSelectDecision={mockOnSelectDecision}
      />,
      { wrapper: createWrapper() }
    );

    expect(screen.getByText('Technical Agent')).toBeInTheDocument();
    expect(screen.getByText('Risk Agent')).toBeInTheDocument();
    expect(screen.getByText('BTC/USDT')).toBeInTheDocument();
    expect(screen.getByText('ETH/USDT')).toBeInTheDocument();
  });

  it('displays decision count', () => {
    render(
      <DecisionTimeline
        selectedDecision={null}
        onSelectDecision={mockOnSelectDecision}
      />,
      { wrapper: createWrapper() }
    );

    expect(screen.getByText(/Showing 2 decisions/)).toBeInTheDocument();
  });

  it('renders loading skeletons when loading', () => {
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

    render(
      <DecisionTimeline
        selectedDecision={null}
        onSelectDecision={mockOnSelectDecision}
      />,
      { wrapper: createWrapper() }
    );

    // Should show skeleton loading state
    const skeletons = document.querySelectorAll('.animate-pulse');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it('shows error state when there is an error', () => {
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

    render(
      <DecisionTimeline
        selectedDecision={null}
        onSelectDecision={mockOnSelectDecision}
      />,
      { wrapper: createWrapper() }
    );

    expect(screen.getByText('Error loading decisions')).toBeInTheDocument();
    expect(screen.getByText('Failed to fetch')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
  });

  it('calls refetch when retry button is clicked', () => {
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

    render(
      <DecisionTimeline
        selectedDecision={null}
        onSelectDecision={mockOnSelectDecision}
      />,
      { wrapper: createWrapper() }
    );

    fireEvent.click(screen.getByRole('button', { name: /retry/i }));
    expect(mockRefetch).toHaveBeenCalledTimes(1);
  });

  it('shows empty state when no decisions', () => {
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

    render(
      <DecisionTimeline
        selectedDecision={null}
        onSelectDecision={mockOnSelectDecision}
      />,
      { wrapper: createWrapper() }
    );

    expect(screen.getByText('No decisions found')).toBeInTheDocument();
    expect(screen.getByText(/Try adjusting your filters/)).toBeInTheDocument();
  });

  it('shows Load More button when hasNextPage is true', () => {
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

    render(
      <DecisionTimeline
        selectedDecision={null}
        onSelectDecision={mockOnSelectDecision}
      />,
      { wrapper: createWrapper() }
    );

    expect(screen.getByRole('button', { name: /load more/i })).toBeInTheDocument();
  });

  it('calls fetchNextPage when Load More is clicked', () => {
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

    render(
      <DecisionTimeline
        selectedDecision={null}
        onSelectDecision={mockOnSelectDecision}
      />,
      { wrapper: createWrapper() }
    );

    fireEvent.click(screen.getByRole('button', { name: /load more/i }));
    expect(mockFetchNextPage).toHaveBeenCalledTimes(1);
  });

  it('shows loading state in Load More button when fetching next page', () => {
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

    render(
      <DecisionTimeline
        selectedDecision={null}
        onSelectDecision={mockOnSelectDecision}
      />,
      { wrapper: createWrapper() }
    );

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

    render(
      <DecisionTimeline
        selectedDecision={null}
        onSelectDecision={mockOnSelectDecision}
      />,
      { wrapper: createWrapper() }
    );

    expect(screen.queryByRole('button', { name: /load more/i })).not.toBeInTheDocument();
    expect(screen.getByText(/all loaded/i)).toBeInTheDocument();
  });

  describe('Filters', () => {
    it('renders filter inputs', () => {
      render(
        <DecisionTimeline
          selectedDecision={null}
          onSelectDecision={mockOnSelectDecision}
        />,
        { wrapper: createWrapper() }
      );

      expect(screen.getByLabelText('Agent')).toBeInTheDocument();
      expect(screen.getByLabelText('Symbol')).toBeInTheDocument();
      expect(screen.getByLabelText('Outcome')).toBeInTheDocument();
    });

    it('updates agent filter when typing', async () => {
      render(
        <DecisionTimeline
          selectedDecision={null}
          onSelectDecision={mockOnSelectDecision}
        />,
        { wrapper: createWrapper() }
      );

      const agentInput = screen.getByLabelText('Agent');
      fireEvent.change(agentInput, { target: { value: 'Technical' } });

      await waitFor(() => {
        expect(agentInput).toHaveValue('Technical');
      });
    });

    it('updates symbol filter when typing', async () => {
      render(
        <DecisionTimeline
          selectedDecision={null}
          onSelectDecision={mockOnSelectDecision}
        />,
        { wrapper: createWrapper() }
      );

      const symbolInput = screen.getByLabelText('Symbol');
      fireEvent.change(symbolInput, { target: { value: 'BTC' } });

      await waitFor(() => {
        expect(symbolInput).toHaveValue('BTC');
      });
    });

    it('updates outcome filter when selecting', async () => {
      render(
        <DecisionTimeline
          selectedDecision={null}
          onSelectDecision={mockOnSelectDecision}
        />,
        { wrapper: createWrapper() }
      );

      const outcomeSelect = screen.getByLabelText('Outcome');
      fireEvent.change(outcomeSelect, { target: { value: 'SUCCESS' } });

      await waitFor(() => {
        expect(outcomeSelect).toHaveValue('SUCCESS');
      });
    });

    it('clears all filters when Clear Filters is clicked', async () => {
      render(
        <DecisionTimeline
          selectedDecision={null}
          onSelectDecision={mockOnSelectDecision}
        />,
        { wrapper: createWrapper() }
      );

      // Set some filter values
      const agentInput = screen.getByLabelText('Agent');
      const symbolInput = screen.getByLabelText('Symbol');
      const outcomeSelect = screen.getByLabelText('Outcome');

      fireEvent.change(agentInput, { target: { value: 'Technical' } });
      fireEvent.change(symbolInput, { target: { value: 'BTC' } });
      fireEvent.change(outcomeSelect, { target: { value: 'SUCCESS' } });

      // Click Clear Filters
      fireEvent.click(screen.getByRole('button', { name: /clear all filters/i }));

      await waitFor(() => {
        expect(agentInput).toHaveValue('');
        expect(symbolInput).toHaveValue('');
        expect(outcomeSelect).toHaveValue('');
      });
    });

    it('calls refetch when refresh button is clicked', () => {
      render(
        <DecisionTimeline
          selectedDecision={null}
          onSelectDecision={mockOnSelectDecision}
        />,
        { wrapper: createWrapper() }
      );

      fireEvent.click(screen.getByRole('button', { name: /refresh/i }));
      expect(mockRefetch).toHaveBeenCalledTimes(1);
    });
  });

  describe('Selection', () => {
    it('calls onSelectDecision when a decision is clicked', () => {
      render(
        <DecisionTimeline
          selectedDecision={null}
          onSelectDecision={mockOnSelectDecision}
        />,
        { wrapper: createWrapper() }
      );

      // Click on the first decision card
      const firstCard = screen.getByText('Technical Agent').closest('[role="listitem"]');
      if (firstCard) {
        fireEvent.click(firstCard.querySelector('[role="button"]') || firstCard);
      }

      expect(mockOnSelectDecision).toHaveBeenCalled();
    });

    it('deselects when clicking the same decision', () => {
      render(
        <DecisionTimeline
          selectedDecision={mockDecisions[0]}
          onSelectDecision={mockOnSelectDecision}
        />,
        { wrapper: createWrapper() }
      );

      // Click on the selected decision card
      const firstCard = screen.getByText('Technical Agent').closest('[role="listitem"]');
      if (firstCard) {
        fireEvent.click(firstCard.querySelector('[role="button"]') || firstCard);
      }

      // Should be called with null to deselect
      expect(mockOnSelectDecision).toHaveBeenCalledWith(null);
    });
  });

  describe('Accessibility', () => {
    it('has proper aria labels', () => {
      render(
        <DecisionTimeline
          selectedDecision={null}
          onSelectDecision={mockOnSelectDecision}
        />,
        { wrapper: createWrapper() }
      );

      expect(screen.getByRole('region', { name: /decision timeline/i })).toBeInTheDocument();
      expect(screen.getByRole('list', { name: /2 decisions/i })).toBeInTheDocument();
    });

    it('has accessible filter inputs with descriptions', () => {
      render(
        <DecisionTimeline
          selectedDecision={null}
          onSelectDecision={mockOnSelectDecision}
        />,
        { wrapper: createWrapper() }
      );

      // Check that inputs have proper labels
      expect(screen.getByLabelText('Agent')).toHaveAttribute('aria-describedby');
      expect(screen.getByLabelText('Symbol')).toHaveAttribute('aria-describedby');
      expect(screen.getByLabelText('Outcome')).toHaveAttribute('aria-describedby');
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

      render(
        <DecisionTimeline
          selectedDecision={null}
          onSelectDecision={mockOnSelectDecision}
        />,
        { wrapper: createWrapper() }
      );

      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
  });
});
