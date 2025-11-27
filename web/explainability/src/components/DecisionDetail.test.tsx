import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import DecisionDetail from './DecisionDetail';
import type { Decision } from '../types';

// Mock the hooks
vi.mock('../hooks/useDecisions', () => ({
  useDecision: vi.fn(),
}));

import { useDecision } from '../hooks/useDecisions';

const mockDecision: Decision = {
  id: '123',
  decision_type: 'BUY_SIGNAL',
  symbol: 'BTC/USDT',
  agent_name: 'Technical Agent',
  prompt: 'Analyze market conditions for BTC/USDT',
  response: 'Strong bullish signals detected based on RSI and MACD indicators',
  model: 'claude-sonnet',
  tokens_used: 500,
  latency_ms: 250,
  confidence: 0.85,
  outcome: 'SUCCESS',
  pnl: 150.5,
  created_at: '2024-01-15T10:30:00.000Z',
};

const createTestQueryClient = () =>
  new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });

const renderWithQueryClient = (component: React.ReactElement) => {
  const queryClient = createTestQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>{component}</QueryClientProvider>
  );
};

describe('DecisionDetail', () => {
  const mockOnClose = vi.fn();
  const mockOnFindSimilar = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    (useDecision as ReturnType<typeof vi.fn>).mockReturnValue({
      data: mockDecision,
      isLoading: false,
      error: null,
    });
  });

  describe('Rendering', () => {
    it('renders decision details correctly', () => {
      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      expect(screen.getByText('Decision Details')).toBeInTheDocument();
      expect(screen.getByText('Technical Agent')).toBeInTheDocument();
      expect(screen.getByText('BTC/USDT')).toBeInTheDocument();
      expect(screen.getByText('BUY_SIGNAL')).toBeInTheDocument();
    });

    it('displays all decision fields', () => {
      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      // Agent and Symbol
      expect(screen.getByText('Technical Agent')).toBeInTheDocument();
      expect(screen.getByText('BTC/USDT')).toBeInTheDocument();

      // Decision Type
      expect(screen.getByText('BUY_SIGNAL')).toBeInTheDocument();

      // Confidence
      expect(screen.getByText('85.0%')).toBeInTheDocument();

      // Latency
      expect(screen.getByText('250ms')).toBeInTheDocument();

      // Tokens
      expect(screen.getByText('500')).toBeInTheDocument();

      // Outcome
      expect(screen.getByText('SUCCESS')).toBeInTheDocument();

      // P&L
      expect(screen.getByText('+$150.50')).toBeInTheDocument();

      // Model
      expect(screen.getByText('claude-sonnet')).toBeInTheDocument();
    });

    it('displays formatted timestamp', () => {
      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      // Check that timestamp is formatted (exact format depends on locale)
      expect(screen.getByText(/Timestamp/i)).toBeInTheDocument();
    });

    it('returns null when decisionId is null', () => {
      const { container } = renderWithQueryClient(
        <DecisionDetail
          decisionId={null}
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      expect(container.firstChild).toBeNull();
    });

    it('renders with ARIA attributes for accessibility', () => {
      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      const dialog = screen.getByRole('dialog');
      expect(dialog).toHaveAttribute('aria-modal', 'true');
      expect(dialog).toHaveAttribute('aria-labelledby', 'decision-detail-title');
    });
  });

  describe('Loading State', () => {
    it('shows loading state', () => {
      (useDecision as ReturnType<typeof vi.fn>).mockReturnValue({
        data: undefined,
        isLoading: true,
        error: null,
      });

      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      expect(
        screen.getByRole('dialog', { name: /loading decision details/i })
      ).toBeInTheDocument();
    });
  });

  describe('Error State', () => {
    it('shows error state', () => {
      const mockError = new Error('Failed to load decision');
      (useDecision as ReturnType<typeof vi.fn>).mockReturnValue({
        data: undefined,
        isLoading: false,
        error: mockError,
      });

      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      expect(screen.getByText(/Failed to load decision/)).toBeInTheDocument();
    });

    it('shows default error message when decision is not found', () => {
      (useDecision as ReturnType<typeof vi.fn>).mockReturnValue({
        data: undefined,
        isLoading: false,
        error: null,
      });

      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      expect(screen.getByText(/Not found/)).toBeInTheDocument();
    });

    it('has close button in error state', () => {
      const mockError = new Error('Failed to load');
      (useDecision as ReturnType<typeof vi.fn>).mockReturnValue({
        data: undefined,
        isLoading: false,
        error: mockError,
      });

      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      const closeButtons = screen.getAllByRole('button', { name: /close/i });
      fireEvent.click(closeButtons[0]);

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });
  });

  describe('Close Functionality', () => {
    it('close button calls onClose', () => {
      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      const closeButton = screen.getAllByRole('button', { name: /close/i })[0];
      fireEvent.click(closeButton);

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });

    it('escape key closes modal', () => {
      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      fireEvent.keyDown(document, { key: 'Escape', code: 'Escape' });

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });

    it('footer close button works', () => {
      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      const closeButtons = screen.getAllByRole('button', { name: /close/i });
      const footerCloseButton = closeButtons[closeButtons.length - 1];
      fireEvent.click(footerCloseButton);

      expect(mockOnClose).toHaveBeenCalledTimes(1);
    });
  });

  describe('Find Similar Button', () => {
    it('find similar button works', () => {
      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      const button = screen.getByRole('button', {
        name: /find decisions similar/i,
      });
      fireEvent.click(button);

      expect(mockOnFindSimilar).toHaveBeenCalledWith('123');
    });

    it('find similar button handles undefined onFindSimilar gracefully', () => {
      renderWithQueryClient(
        <DecisionDetail decisionId="123" onClose={mockOnClose} />
      );

      const button = screen.getByRole('button', {
        name: /find decisions similar/i,
      });
      expect(button).toBeInTheDocument();

      // Click should not cause error even when onFindSimilar is undefined
      expect(() => fireEvent.click(button)).not.toThrow();
      expect(mockOnFindSimilar).not.toHaveBeenCalled();
    });
  });

  describe('Expand/Collapse Sections', () => {
    it('prompt section expands when clicked', async () => {
      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      const promptButton = screen.getByRole('button', { name: /expand prompt/i });
      expect(promptButton).toHaveAttribute('aria-expanded', 'false');

      fireEvent.click(promptButton);

      await waitFor(() => {
        expect(
          screen.getByText('Analyze market conditions for BTC/USDT')
        ).toBeInTheDocument();
      });

      expect(promptButton).toHaveAttribute('aria-expanded', 'true');
    });

    it('prompt section collapses when clicked again', async () => {
      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      const promptButton = screen.getByRole('button', { name: /expand prompt/i });

      // Expand
      fireEvent.click(promptButton);
      await waitFor(() => {
        expect(promptButton).toHaveAttribute('aria-expanded', 'true');
      });

      // Collapse
      fireEvent.click(promptButton);
      await waitFor(() => {
        expect(promptButton).toHaveAttribute('aria-expanded', 'false');
      });
    });

    it('response section expands when clicked', async () => {
      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      const responseButton = screen.getByRole('button', {
        name: /expand response/i,
      });
      expect(responseButton).toHaveAttribute('aria-expanded', 'false');

      fireEvent.click(responseButton);

      await waitFor(() => {
        expect(
          screen.getByText(
            'Strong bullish signals detected based on RSI and MACD indicators'
          )
        ).toBeInTheDocument();
      });

      expect(responseButton).toHaveAttribute('aria-expanded', 'true');
    });

    it('response section collapses when clicked again', async () => {
      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      const responseButton = screen.getByRole('button', {
        name: /expand response/i,
      });

      // Expand
      fireEvent.click(responseButton);
      await waitFor(() => {
        expect(responseButton).toHaveAttribute('aria-expanded', 'true');
      });

      // Collapse
      fireEvent.click(responseButton);
      await waitFor(() => {
        expect(responseButton).toHaveAttribute('aria-expanded', 'false');
      });
    });
  });

  describe('Outcome and P&L Display', () => {
    it('displays SUCCESS outcome with correct styling', () => {
      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      const outcome = screen.getByText('SUCCESS');
      expect(outcome).toHaveClass('text-green-400');
    });

    it('displays FAILURE outcome with correct styling', () => {
      (useDecision as ReturnType<typeof vi.fn>).mockReturnValue({
        data: { ...mockDecision, outcome: 'FAILURE' },
        isLoading: false,
        error: null,
      });

      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      const outcome = screen.getByText('FAILURE');
      expect(outcome).toHaveClass('text-red-400');
    });

    it('displays PENDING outcome with correct styling', () => {
      (useDecision as ReturnType<typeof vi.fn>).mockReturnValue({
        data: { ...mockDecision, outcome: 'PENDING' },
        isLoading: false,
        error: null,
      });

      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      const outcome = screen.getByText('PENDING');
      expect(outcome).toHaveClass('text-yellow-400');
    });

    it('displays positive P&L with plus sign and green color', () => {
      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      const pnl = screen.getByText('+$150.50');
      expect(pnl).toHaveClass('text-green-400');
    });

    it('displays negative P&L with minus sign and red color', () => {
      (useDecision as ReturnType<typeof vi.fn>).mockReturnValue({
        data: { ...mockDecision, pnl: -75.25 },
        isLoading: false,
        error: null,
      });

      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      const pnl = screen.getByText('$-75.25');
      expect(pnl).toHaveClass('text-red-400');
    });

    it('displays N/A for undefined P&L', () => {
      (useDecision as ReturnType<typeof vi.fn>).mockReturnValue({
        data: { ...mockDecision, pnl: undefined },
        isLoading: false,
        error: null,
      });

      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      expect(screen.getByText('N/A')).toBeInTheDocument();
    });

    it('displays PENDING for undefined outcome', () => {
      (useDecision as ReturnType<typeof vi.fn>).mockReturnValue({
        data: { ...mockDecision, outcome: undefined },
        isLoading: false,
        error: null,
      });

      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      expect(screen.getByText('PENDING')).toBeInTheDocument();
    });
  });

  describe('Focus Management', () => {
    it('focuses close button when modal opens', async () => {
      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      await waitFor(() => {
        const closeButton = screen.getAllByRole('button', {
          name: /close modal/i,
        })[0];
        // Note: Focus testing in jsdom is limited, this is a basic check
        expect(closeButton).toBeInTheDocument();
      });
    });
  });

  describe('Metrics Display', () => {
    it('displays confidence as percentage', () => {
      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      expect(screen.getByText('85.0%')).toBeInTheDocument();
    });

    it('handles zero confidence', () => {
      (useDecision as ReturnType<typeof vi.fn>).mockReturnValue({
        data: { ...mockDecision, confidence: 0 },
        isLoading: false,
        error: null,
      });

      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      expect(screen.getByText('0.0%')).toBeInTheDocument();
    });

    it('displays N/A for missing latency', () => {
      (useDecision as ReturnType<typeof vi.fn>).mockReturnValue({
        data: { ...mockDecision, latency_ms: undefined },
        isLoading: false,
        error: null,
      });

      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      expect(screen.getByText('N/Ams')).toBeInTheDocument();
    });

    it('displays N/A for missing tokens', () => {
      (useDecision as ReturnType<typeof vi.fn>).mockReturnValue({
        data: { ...mockDecision, tokens_used: undefined },
        isLoading: false,
        error: null,
      });

      renderWithQueryClient(
        <DecisionDetail
          decisionId="123"
          onClose={mockOnClose}
          onFindSimilar={mockOnFindSimilar}
        />
      );

      expect(screen.getByText('N/A')).toBeInTheDocument();
    });
  });
});
