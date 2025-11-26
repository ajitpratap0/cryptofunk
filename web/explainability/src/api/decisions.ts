/**
 * API client for LLM decision explainability endpoints
 * Communicates with the CryptoFunk REST API
 */

import type {
  Decision,
  DecisionFilter,
  DecisionStats,
  ListDecisionsResponse,
  SearchDecisionsResponse,
  SimilarDecisionsResponse,
  ApiError,
} from '../types/decision';

// Use Vite's import.meta.env for environment variables
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '';
const API_PREFIX = '/api/v1';

/**
 * Custom error class for API errors
 */
export class DecisionApiError extends Error {
  status?: number;

  constructor(message: string, status?: number) {
    super(message);
    this.name = 'DecisionApiError';
    this.status = status;
  }
}

/**
 * Helper function to build query string from filter object
 */
function buildQueryString(filter: DecisionFilter): string {
  const params = new URLSearchParams();

  if (filter.symbol) params.append('symbol', filter.symbol);
  if (filter.decision_type) params.append('decision_type', filter.decision_type);
  if (filter.outcome) params.append('outcome', filter.outcome);
  if (filter.model) params.append('model', filter.model);
  if (filter.from_date) params.append('from_date', filter.from_date);
  if (filter.to_date) params.append('to_date', filter.to_date);
  if (filter.limit !== undefined) params.append('limit', filter.limit.toString());
  if (filter.offset !== undefined) params.append('offset', filter.offset.toString());

  const query = params.toString();
  return query ? `?${query}` : '';
}

/**
 * Helper function to handle fetch responses
 */
async function handleResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let errorMessage = `HTTP ${response.status}: ${response.statusText}`;

    try {
      const errorData: ApiError = await response.json();
      errorMessage = errorData.message || errorData.error || errorMessage;
    } catch (e) {
      // If parsing JSON fails, use the default error message
      if (import.meta.env.DEV) {
        console.warn('Failed to parse error response:', e);
      }
    }

    throw new DecisionApiError(errorMessage, response.status);
  }

  return response.json();
}

/**
 * List decisions with optional filtering
 *
 * @param filter - Filtering and pagination options
 * @returns Promise resolving to decisions and total count
 */
export async function listDecisions(filter: DecisionFilter = {}): Promise<ListDecisionsResponse> {
  const queryString = buildQueryString(filter);
  const url = `${API_BASE_URL}${API_PREFIX}/decisions${queryString}`;

  const response = await fetch(url, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
  });

  return handleResponse<ListDecisionsResponse>(response);
}

/**
 * Get a single decision by ID
 *
 * @param id - Decision UUID
 * @returns Promise resolving to decision details
 */
export async function getDecision(id: string): Promise<Decision> {
  const url = `${API_BASE_URL}${API_PREFIX}/decisions/${id}`;

  const response = await fetch(url, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
  });

  return handleResponse<Decision>(response);
}

/**
 * Get aggregated statistics for decisions
 *
 * @param filter - Optional filtering options
 * @returns Promise resolving to decision statistics
 */
export async function getDecisionStats(filter?: DecisionFilter): Promise<DecisionStats> {
  const queryString = filter ? buildQueryString(filter) : '';
  const url = `${API_BASE_URL}${API_PREFIX}/decisions/stats${queryString}`;

  const response = await fetch(url, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
  });

  return handleResponse<DecisionStats>(response);
}

/**
 * Search decisions using text or semantic search
 *
 * @param query - Search query string
 * @param limit - Maximum number of results (default: 20, max: 100)
 * @returns Promise resolving to search results with relevance scores
 */
export async function searchDecisions(
  query: string,
  limit?: number
): Promise<SearchDecisionsResponse> {
  const url = `${API_BASE_URL}${API_PREFIX}/decisions/search`;

  const body: { query: string; limit?: number } = { query };
  if (limit !== undefined) {
    body.limit = limit;
  }

  const response = await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(body),
  });

  return handleResponse<SearchDecisionsResponse>(response);
}

/**
 * Find decisions similar to a given decision using vector similarity
 *
 * @param id - Decision UUID to find similar decisions for
 * @param limit - Maximum number of results (default: 10, max: 50)
 * @returns Promise resolving to similar decisions
 */
export async function getSimilarDecisions(
  id: string,
  limit?: number
): Promise<SimilarDecisionsResponse> {
  const params = new URLSearchParams();
  if (limit !== undefined) params.append('limit', limit.toString());

  const queryString = params.toString() ? `?${params.toString()}` : '';
  const url = `${API_BASE_URL}${API_PREFIX}/decisions/${id}/similar${queryString}`;

  const response = await fetch(url, {
    method: 'GET',
    headers: {
      'Content-Type': 'application/json',
    },
  });

  return handleResponse<SimilarDecisionsResponse>(response);
}

/**
 * Default export with all API functions
 */
export default {
  listDecisions,
  getDecision,
  getDecisionStats,
  searchDecisions,
  getSimilarDecisions,
};
