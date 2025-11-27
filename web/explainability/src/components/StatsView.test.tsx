import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import StatsView from './StatsView';
import * as useDecisionsHook from '../hooks/useDecisions';
import type { DecisionStats } from '../types';

// Mock the useDecisionStats hook
vi.mock('../hooks/useDecisions', () => ({
  useDecisionStats: vi.fn(),
}));

const mockStats: DecisionStats = {
  total_decisions: 150,
  success_rate: 0.75,
  avg_confidence: 0.82,
  total_pnl: 1250.50,
  avg_pnl: 8.34,
  avg_latency_ms: 234,
  avg_tokens_used: 512,
  by_outcome: {
    SUCCESS: 112,
    FAILURE: 28,
    PENDING: 10,
  },
  by_agent: {
    'Technical Agent': 50,
    'Risk Agent': 100,
  },
  by_symbol: {
    'BTC/USDT': 100,
    'ETH/USDT': 50,
  },
};

describe('StatsView', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    });
    vi.clearAllMocks();
  });

  const renderWithClient = (component: React.ReactElement) => {
    return render(
      <QueryClientProvider client={queryClient}>
        {component}
      </QueryClientProvider>
    );
  };

  it('renders loading state', () => {
    vi.mocked(useDecisionsHook.useDecisionStats).mockReturnValue({
      data: undefined,
      isLoading: true,
      error: null,
      refetch: vi.fn(),
    } as any);

    renderWithClient(<StatsView />);

    // Check that the stats view is loading by looking for a styled container
    // The loading state shows a bg-slate-800 p-8 rounded-lg div
    const containers = document.querySelectorAll('.bg-slate-800');
    expect(containers.length).toBeGreaterThan(0);
  });

  it('renders error state', () => {
    const mockError = new Error('Failed to fetch stats');
    vi.mocked(useDecisionsHook.useDecisionStats).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: mockError,
      refetch: vi.fn(),
    } as any);

    renderWithClient(<StatsView />);

    expect(screen.getByText(/Error loading statistics/i)).toBeInTheDocument();
    expect(screen.getByText(/Failed to fetch stats/i)).toBeInTheDocument();
  });

  it('renders error state when stats is null', () => {
    vi.mocked(useDecisionsHook.useDecisionStats).mockReturnValue({
      data: null,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as any);

    renderWithClient(<StatsView />);

    expect(screen.getByText(/Error loading statistics/i)).toBeInTheDocument();
    expect(screen.getByText(/Unknown error/i)).toBeInTheDocument();
  });

  it('renders statistics correctly', () => {
    vi.mocked(useDecisionsHook.useDecisionStats).mockReturnValue({
      data: mockStats,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as any);

    renderWithClient(<StatsView />);

    // Check key metrics
    expect(screen.getByText('150')).toBeInTheDocument(); // Total decisions
    expect(screen.getByText('75.0%')).toBeInTheDocument(); // Success rate
    expect(screen.getByText('82.0%')).toBeInTheDocument(); // Avg confidence
    expect(screen.getByText('+$1250.50')).toBeInTheDocument(); // Total P&L
  });

  it('displays outcome distribution correctly', () => {
    vi.mocked(useDecisionsHook.useDecisionStats).mockReturnValue({
      data: mockStats,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as any);

    renderWithClient(<StatsView />);

    // Check outcome counts
    expect(screen.getByText('112')).toBeInTheDocument(); // Success count
    expect(screen.getByText('28')).toBeInTheDocument(); // Failure count
    expect(screen.getByText('10')).toBeInTheDocument(); // Pending count

    // Check labels
    expect(screen.getByText('Success')).toBeInTheDocument();
    expect(screen.getByText('Failure')).toBeInTheDocument();
    expect(screen.getByText('Pending')).toBeInTheDocument();
  });

  it('displays additional metrics correctly', () => {
    vi.mocked(useDecisionsHook.useDecisionStats).mockReturnValue({
      data: mockStats,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as any);

    renderWithClient(<StatsView />);

    expect(screen.getByText('234ms')).toBeInTheDocument(); // Avg latency
    expect(screen.getByText('512')).toBeInTheDocument(); // Avg tokens
    expect(screen.getByText('+$8.34')).toBeInTheDocument(); // Avg P&L per decision
  });

  it('handles negative P&L correctly', () => {
    const negativeStats: DecisionStats = {
      ...mockStats,
      total_pnl: -500.25,
      avg_pnl: -3.34,
    };

    vi.mocked(useDecisionsHook.useDecisionStats).mockReturnValue({
      data: negativeStats,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as any);

    const { container } = renderWithClient(<StatsView />);

    // Find elements with text-red-400 class (negative P&L)
    const pnlElements = container.querySelectorAll('.text-red-400');
    expect(pnlElements.length).toBeGreaterThan(0);

    // Verify negative values are displayed (format is $-500.25, not -$500.25)
    expect(screen.getByText('$-500.25')).toBeInTheDocument();
    expect(screen.getByText('$-3.34')).toBeInTheDocument();
  });

  it('handles zero values correctly', () => {
    const zeroStats: DecisionStats = {
      total_decisions: 0,
      success_rate: 0,
      avg_confidence: 0,
      total_pnl: 0,
      avg_pnl: 0,
      avg_latency_ms: 0,
      avg_tokens_used: 0,
      by_outcome: {},
      by_agent: {},
      by_symbol: {},
    };

    vi.mocked(useDecisionsHook.useDecisionStats).mockReturnValue({
      data: zeroStats,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as any);

    renderWithClient(<StatsView />);

    const zeros = screen.getAllByText('0');
    expect(zeros.length).toBeGreaterThan(0); // Total decisions and other metrics
    const percentages = screen.getAllByText('0.0%');
    expect(percentages.length).toBeGreaterThan(0); // Success rate and avg confidence both show 0.0%
    const pnlValues = screen.getAllByText('+$0.00');
    expect(pnlValues.length).toBeGreaterThan(0); // Zero P&L with + sign (total and avg)
  });

  it('handles missing by_outcome data', () => {
    const statsWithoutOutcome: DecisionStats = {
      ...mockStats,
      by_outcome: undefined as any,
    };

    vi.mocked(useDecisionsHook.useDecisionStats).mockReturnValue({
      data: statsWithoutOutcome,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as any);

    renderWithClient(<StatsView />);

    // Should default to 0 for all outcomes
    const outcomes = screen.getAllByText('0');
    expect(outcomes.length).toBeGreaterThan(0);
  });

  it('calls refetch when refresh button is clicked', () => {
    const mockRefetch = vi.fn();
    vi.mocked(useDecisionsHook.useDecisionStats).mockReturnValue({
      data: mockStats,
      isLoading: false,
      error: null,
      refetch: mockRefetch,
    } as any);

    renderWithClient(<StatsView />);

    const refreshButton = screen.getByText('Refresh').closest('button');
    expect(refreshButton).toBeInTheDocument();

    fireEvent.click(refreshButton!);
    expect(mockRefetch).toHaveBeenCalledTimes(1);
  });

  it('renders statistics header', () => {
    vi.mocked(useDecisionsHook.useDecisionStats).mockReturnValue({
      data: mockStats,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as any);

    renderWithClient(<StatsView />);

    expect(screen.getByText('Statistics')).toBeInTheDocument();
  });

  it('displays correct metric labels', () => {
    vi.mocked(useDecisionsHook.useDecisionStats).mockReturnValue({
      data: mockStats,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as any);

    renderWithClient(<StatsView />);

    expect(screen.getByText('Total Decisions')).toBeInTheDocument();
    expect(screen.getByText('Success Rate')).toBeInTheDocument();
    expect(screen.getByText('Avg Confidence')).toBeInTheDocument();
    expect(screen.getByText('Total P&L')).toBeInTheDocument();
    expect(screen.getByText('Avg Latency')).toBeInTheDocument();
    expect(screen.getByText('Avg Tokens')).toBeInTheDocument();
    expect(screen.getByText('Avg P&L per Decision')).toBeInTheDocument();
  });

  it('displays outcome distribution chart title', () => {
    vi.mocked(useDecisionsHook.useDecisionStats).mockReturnValue({
      data: mockStats,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as any);

    renderWithClient(<StatsView />);

    expect(screen.getByText('Outcome Distribution')).toBeInTheDocument();
  });

  it('handles partial by_outcome data', () => {
    const partialStats = {
      ...mockStats,
      by_outcome: {
        SUCCESS: 50,
        // Missing FAILURE and PENDING
      },
    };

    vi.mocked(useDecisionsHook.useDecisionStats).mockReturnValue({
      data: partialStats,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as any);

    renderWithClient(<StatsView />);

    expect(screen.getByText('50')).toBeInTheDocument(); // Success count
    // Failure and Pending should show 0
    const zeros = screen.getAllByText('0');
    expect(zeros.length).toBeGreaterThan(0);
  });

  it('formats P&L with correct sign', () => {
    vi.mocked(useDecisionsHook.useDecisionStats).mockReturnValue({
      data: mockStats,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as any);

    renderWithClient(<StatsView />);

    // Positive P&L should have + sign
    expect(screen.getByText('+$1250.50')).toBeInTheDocument();
    expect(screen.getByText('+$8.34')).toBeInTheDocument();
  });

  it('handles success rate as decimal correctly', () => {
    const statsWithDifferentRate = {
      ...mockStats,
      success_rate: 0.333,
    };

    vi.mocked(useDecisionsHook.useDecisionStats).mockReturnValue({
      data: statsWithDifferentRate,
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    } as any);

    renderWithClient(<StatsView />);

    expect(screen.getByText('33.3%')).toBeInTheDocument();
  });
});
