/**
 * Type definitions for LLM decision explainability
 * Based on Go structs in internal/api/decisions.go
 */

/**
 * Represents an LLM decision record from the database
 */
export interface Decision {
  id: string;
  session_id?: string;
  decision_type: string;
  symbol: string;
  agent_name?: string;
  prompt: string;
  response: string;
  model: string;
  tokens_used?: number;
  latency_ms?: number;
  confidence?: number;
  outcome?: 'SUCCESS' | 'FAILURE' | 'PENDING';
  pnl?: number;
  created_at: string;
}

/**
 * Filter options for listing decisions
 */
export interface DecisionFilter {
  symbol?: string;
  decision_type?: string;
  outcome?: string;
  model?: string;
  agent_name?: string;
  from_date?: string;
  to_date?: string;
  limit?: number;
  offset?: number;
}

/**
 * Aggregated statistics for decisions
 */
export interface DecisionStats {
  total_decisions: number;
  by_type: Record<string, number>;
  by_outcome: Record<string, number>;
  by_model: Record<string, number>;
  avg_confidence: number;
  avg_latency_ms: number;
  avg_tokens_used: number;
  success_rate: number;
  total_pnl: number;
  avg_pnl: number;
}

/**
 * Search result with relevance score
 */
export interface SearchResult {
  decision: Decision;
  score: number;
}

/**
 * Response from list decisions endpoint
 */
export interface ListDecisionsResponse {
  decisions: Decision[];
  count: number;
}

/**
 * Response from search decisions endpoint
 */
export interface SearchDecisionsResponse {
  results: SearchResult[];
  count: number;
}

/**
 * Response from similar decisions endpoint
 */
export interface SimilarDecisionsResponse {
  similar: Decision[];
  count: number;
}

/**
 * API error response
 */
export interface ApiError {
  error: string;
  message?: string;
  status?: number;
}
