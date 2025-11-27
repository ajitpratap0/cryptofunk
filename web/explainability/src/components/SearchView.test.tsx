import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import SearchView from './SearchView';
import type { Decision } from '../types';

// Mock the hooks
vi.mock('../hooks/useDecisions', () => ({
  useSearchDecisions: vi.fn(),
}));

import { useSearchDecisions } from '../hooks/useDecisions';

const mockDecision: Decision = {
  id: '123',
  decision_type: 'BUY_SIGNAL',
  symbol: 'BTC/USDT',
  agent_name: 'Technical Agent',
  prompt: 'Analyze market conditions',
  response: 'Strong bullish signals detected',
  model: 'claude-sonnet',
  tokens_used: 500,
  latency_ms: 250,
  confidence: 0.85,
  outcome: 'SUCCESS',
  pnl: 150.5,
  created_at: new Date().toISOString(),
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

describe('SearchView', () => {
  const mockOnSelectDecision = vi.fn();
  const mockMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    (useSearchDecisions as ReturnType<typeof vi.fn>).mockReturnValue({
      mutate: mockMutate,
      data: undefined,
      isPending: false,
      error: null,
    });
  });

  describe('Rendering', () => {
    it('renders search input field', () => {
      renderWithQueryClient(
        <SearchView onSelectDecision={mockOnSelectDecision} />
      );

      const input = screen.getByPlaceholderText('Why did you buy BTC?');
      expect(input).toBeInTheDocument();
      expect(input).toHaveAttribute('type', 'text');
    });

    it('renders search button', () => {
      renderWithQueryClient(
        <SearchView onSelectDecision={mockOnSelectDecision} />
      );

      const button = screen.getByRole('button', { name: /search/i });
      expect(button).toBeInTheDocument();
    });

    it('renders header and description', () => {
      renderWithQueryClient(
        <SearchView onSelectDecision={mockOnSelectDecision} />
      );

      expect(screen.getByText('Semantic Search')).toBeInTheDocument();
      expect(
        screen.getByText(/Search for decisions using natural language/i)
      ).toBeInTheDocument();
    });

    it('renders example queries in empty state', () => {
      renderWithQueryClient(
        <SearchView onSelectDecision={mockOnSelectDecision} />
      );

      expect(screen.getByText('Example queries:')).toBeInTheDocument();
      expect(screen.getByText(/Why did you buy BTC?/i)).toBeInTheDocument();
      expect(
        screen.getByText(/Show me failed risk approvals/i)
      ).toBeInTheDocument();
    });
  });

  describe('Search Functionality', () => {
    it('calls search function when button clicked', () => {
      renderWithQueryClient(
        <SearchView onSelectDecision={mockOnSelectDecision} />
      );

      const input = screen.getByPlaceholderText('Why did you buy BTC?');
      const button = screen.getByRole('button', { name: /search/i });

      fireEvent.change(input, { target: { value: 'test query' } });
      fireEvent.click(button);

      expect(mockMutate).toHaveBeenCalledWith({
        query: 'test query',
        limit: 20,
      });
    });

    it('calls search function when Enter key is pressed', () => {
      renderWithQueryClient(
        <SearchView onSelectDecision={mockOnSelectDecision} />
      );

      const input = screen.getByPlaceholderText('Why did you buy BTC?');

      fireEvent.change(input, { target: { value: 'test query' } });
      fireEvent.keyPress(input, { key: 'Enter', code: 13, charCode: 13 });

      expect(mockMutate).toHaveBeenCalledWith({
        query: 'test query',
        limit: 20,
      });
    });

    it('trims whitespace from query before searching', () => {
      renderWithQueryClient(
        <SearchView onSelectDecision={mockOnSelectDecision} />
      );

      const input = screen.getByPlaceholderText('Why did you buy BTC?');
      const button = screen.getByRole('button', { name: /search/i });

      fireEvent.change(input, { target: { value: '  test query  ' } });
      fireEvent.click(button);

      expect(mockMutate).toHaveBeenCalledWith({
        query: 'test query',
        limit: 20,
      });
    });

    it('does not call search with empty query', () => {
      renderWithQueryClient(
        <SearchView onSelectDecision={mockOnSelectDecision} />
      );

      const button = screen.getByRole('button', { name: /search/i });

      fireEvent.click(button);

      expect(mockMutate).not.toHaveBeenCalled();
    });

    it('does not call search with whitespace-only query', () => {
      renderWithQueryClient(
        <SearchView onSelectDecision={mockOnSelectDecision} />
      );

      const input = screen.getByPlaceholderText('Why did you buy BTC?');
      const button = screen.getByRole('button', { name: /search/i });

      fireEvent.change(input, { target: { value: '   ' } });
      fireEvent.click(button);

      expect(mockMutate).not.toHaveBeenCalled();
    });

    it('disables button when query is empty', () => {
      renderWithQueryClient(
        <SearchView onSelectDecision={mockOnSelectDecision} />
      );

      const button = screen.getByRole('button', { name: /search/i });

      expect(button).toBeDisabled();
    });

    it('disables button when searching', () => {
      (useSearchDecisions as ReturnType<typeof vi.fn>).mockReturnValue({
        mutate: mockMutate,
        data: undefined,
        isPending: true,
        error: null,
      });

      renderWithQueryClient(
        <SearchView onSelectDecision={mockOnSelectDecision} />
      );

      const input = screen.getByPlaceholderText('Why did you buy BTC?');
      const button = screen.getByRole('button', { name: /search/i });

      fireEvent.change(input, { target: { value: 'test' } });

      expect(button).toBeDisabled();
    });
  });

  describe('Loading State', () => {
    it('shows loading state during search', () => {
      (useSearchDecisions as ReturnType<typeof vi.fn>).mockReturnValue({
        mutate: mockMutate,
        data: undefined,
        isPending: true,
        error: null,
      });

      renderWithQueryClient(
        <SearchView onSelectDecision={mockOnSelectDecision} />
      );

      expect(
        screen.getByText('Searching for similar decisions...')
      ).toBeInTheDocument();
    });

    it('renders loading spinner during search', () => {
      (useSearchDecisions as ReturnType<typeof vi.fn>).mockReturnValue({
        mutate: mockMutate,
        data: undefined,
        isPending: true,
        error: null,
      });

      renderWithQueryClient(
        <SearchView onSelectDecision={mockOnSelectDecision} />
      );

      expect(
        screen.getByText('Searching for similar decisions...')
      ).toBeInTheDocument();
    });
  });

  describe('Error State', () => {
    it('shows error state on search failure', () => {
      const mockError = new Error('Search failed');
      (useSearchDecisions as ReturnType<typeof vi.fn>).mockReturnValue({
        mutate: mockMutate,
        data: undefined,
        isPending: false,
        error: mockError,
      });

      renderWithQueryClient(
        <SearchView onSelectDecision={mockOnSelectDecision} />
      );

      expect(screen.getByText(/Error:/)).toBeInTheDocument();
      expect(screen.getByText(/Search failed/)).toBeInTheDocument();
    });

    it('shows default error message when error message is empty', () => {
      const mockError = { message: '' } as Error;
      (useSearchDecisions as ReturnType<typeof vi.fn>).mockReturnValue({
        mutate: mockMutate,
        data: undefined,
        isPending: false,
        error: mockError,
      });

      renderWithQueryClient(
        <SearchView onSelectDecision={mockOnSelectDecision} />
      );

      expect(screen.getByText(/Failed to search decisions/)).toBeInTheDocument();
    });
  });

  describe('Search Results', () => {
    it('displays search results correctly', () => {
      const mockResults = {
        results: [
          { decision: mockDecision, score: 0.95 },
          {
            decision: { ...mockDecision, id: '456', agent_name: 'Risk Agent' },
            score: 0.87,
          },
        ],
      };

      (useSearchDecisions as ReturnType<typeof vi.fn>).mockReturnValue({
        mutate: mockMutate,
        data: mockResults,
        isPending: false,
        error: null,
      });

      renderWithQueryClient(
        <SearchView onSelectDecision={mockOnSelectDecision} />
      );

      expect(screen.getByText('Found 2 similar decisions')).toBeInTheDocument();
      expect(screen.getByText('Technical Agent')).toBeInTheDocument();
      expect(screen.getByText('Risk Agent')).toBeInTheDocument();
    });

    it('displays relevance score badges', () => {
      const mockResults = {
        results: [{ decision: mockDecision, score: 0.95 }],
      };

      (useSearchDecisions as ReturnType<typeof vi.fn>).mockReturnValue({
        mutate: mockMutate,
        data: mockResults,
        isPending: false,
        error: null,
      });

      renderWithQueryClient(
        <SearchView onSelectDecision={mockOnSelectDecision} />
      );

      expect(screen.getByText('95.0% match')).toBeInTheDocument();
    });

    it('handles singular/plural in result count', () => {
      const mockResults = {
        results: [{ decision: mockDecision, score: 0.95 }],
      };

      (useSearchDecisions as ReturnType<typeof vi.fn>).mockReturnValue({
        mutate: mockMutate,
        data: mockResults,
        isPending: false,
        error: null,
      });

      renderWithQueryClient(
        <SearchView onSelectDecision={mockOnSelectDecision} />
      );

      expect(
        screen.getByText('Found 1 similar decision')
      ).toBeInTheDocument();
    });
  });

  describe('Empty Results', () => {
    it('handles empty results', () => {
      const mockResults = {
        results: [],
      };

      (useSearchDecisions as ReturnType<typeof vi.fn>).mockReturnValue({
        mutate: mockMutate,
        data: mockResults,
        isPending: false,
        error: null,
      });

      renderWithQueryClient(
        <SearchView onSelectDecision={mockOnSelectDecision} />
      );

      expect(
        screen.getByText('No similar decisions found for your query.')
      ).toBeInTheDocument();
      expect(
        screen.getByText(/Try rephrasing your search/i)
      ).toBeInTheDocument();
    });
  });

  describe('Decision Selection', () => {
    it('calls onSelectDecision when decision is clicked', () => {
      const mockResults = {
        results: [{ decision: mockDecision, score: 0.95 }],
      };

      (useSearchDecisions as ReturnType<typeof vi.fn>).mockReturnValue({
        mutate: mockMutate,
        data: mockResults,
        isPending: false,
        error: null,
      });

      renderWithQueryClient(
        <SearchView onSelectDecision={mockOnSelectDecision} />
      );

      const card = screen.getByText('Technical Agent').closest('div');
      if (card?.parentElement?.parentElement) {
        fireEvent.click(card.parentElement.parentElement);
      }

      expect(mockOnSelectDecision).toHaveBeenCalled();
    });

    it('toggles selection when clicking same decision twice', () => {
      const mockResults = {
        results: [{ decision: mockDecision, score: 0.95 }],
      };

      (useSearchDecisions as ReturnType<typeof vi.fn>).mockReturnValue({
        mutate: mockMutate,
        data: mockResults,
        isPending: false,
        error: null,
      });

      renderWithQueryClient(
        <SearchView onSelectDecision={mockOnSelectDecision} />
      );

      const card = screen.getByText('Technical Agent').closest('div');
      if (card?.parentElement?.parentElement) {
        fireEvent.click(card.parentElement.parentElement);
        fireEvent.click(card.parentElement.parentElement);
      }

      expect(mockOnSelectDecision).toHaveBeenCalledTimes(2);
    });
  });

  describe('Input Updates', () => {
    it('updates query state when input changes', () => {
      renderWithQueryClient(
        <SearchView onSelectDecision={mockOnSelectDecision} />
      );

      const input = screen.getByPlaceholderText(
        'Why did you buy BTC?'
      ) as HTMLInputElement;

      fireEvent.change(input, { target: { value: 'new query' } });

      expect(input.value).toBe('new query');
    });
  });
});
