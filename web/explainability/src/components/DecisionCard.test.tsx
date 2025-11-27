import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import DecisionCard from './DecisionCard';
import type { Decision } from '../types';

const mockDecision: Decision = {
  id: '123',
  decision_type: 'BUY_SIGNAL',
  symbol: 'BTC/USDT',
  agent_name: 'Technical Agent',
  prompt: 'Analyze market conditions',
  response: 'Strong bullish signals detected based on RSI and MACD indicators',
  model: 'claude-sonnet',
  tokens_used: 500,
  latency_ms: 250,
  confidence: 0.85,
  outcome: 'SUCCESS',
  pnl: 150.5,
  created_at: new Date().toISOString(),
};

describe('DecisionCard', () => {
  it('renders decision information correctly', () => {
    render(
      <DecisionCard decision={mockDecision} selected={false} onClick={() => {}} />
    );

    expect(screen.getByText('Technical Agent')).toBeInTheDocument();
    expect(screen.getByText('BTC/USDT')).toBeInTheDocument();
    expect(screen.getByText('BUY_SIGNAL')).toBeInTheDocument();
  });

  it('displays confidence percentage correctly', () => {
    render(
      <DecisionCard decision={mockDecision} selected={false} onClick={() => {}} />
    );

    expect(screen.getByText('85%')).toBeInTheDocument();
  });

  it('shows success outcome badge', () => {
    render(
      <DecisionCard decision={mockDecision} selected={false} onClick={() => {}} />
    );

    expect(screen.getByText('Success')).toBeInTheDocument();
  });

  it('shows failure outcome badge for failed decisions', () => {
    const failedDecision = { ...mockDecision, outcome: 'FAILURE' as const };
    render(
      <DecisionCard decision={failedDecision} selected={false} onClick={() => {}} />
    );

    expect(screen.getByText('Failure')).toBeInTheDocument();
  });

  it('shows pending outcome badge when outcome is not set', () => {
    const pendingDecision = { ...mockDecision, outcome: undefined };
    render(
      <DecisionCard decision={pendingDecision} selected={false} onClick={() => {}} />
    );

    expect(screen.getByText('Pending')).toBeInTheDocument();
  });

  it('calls onClick when clicked', () => {
    const handleClick = vi.fn();
    render(
      <DecisionCard decision={mockDecision} selected={false} onClick={handleClick} />
    );

    fireEvent.click(screen.getByText('Technical Agent').closest('div')!.parentElement!.parentElement!);
    expect(handleClick).toHaveBeenCalledTimes(1);
  });

  it('applies selected styling when selected', () => {
    const { container } = render(
      <DecisionCard decision={mockDecision} selected={true} onClick={() => {}} />
    );

    const card = container.firstChild as HTMLElement;
    expect(card.className).toContain('border-blue-500');
  });

  it('truncates long response text', () => {
    const longResponseDecision = {
      ...mockDecision,
      response: 'A'.repeat(200),
    };
    render(
      <DecisionCard decision={longResponseDecision} selected={false} onClick={() => {}} />
    );

    // Should show truncated text with ellipsis
    const responseElement = screen.getByText(/A+\.\.\.$/);
    expect(responseElement).toBeInTheDocument();
  });

  it('displays correct icon for BUY decisions', () => {
    render(
      <DecisionCard decision={mockDecision} selected={false} onClick={() => {}} />
    );

    // TrendingUp icon should be rendered for BUY decisions
    const icon = document.querySelector('.text-green-400');
    expect(icon).toBeInTheDocument();
  });

  it('displays correct icon for SELL decisions', () => {
    const sellDecision = { ...mockDecision, decision_type: 'SELL_SIGNAL' };
    render(
      <DecisionCard decision={sellDecision} selected={false} onClick={() => {}} />
    );

    // TrendingDown icon should be rendered for SELL decisions
    const icon = document.querySelector('.text-red-400');
    expect(icon).toBeInTheDocument();
  });
});
