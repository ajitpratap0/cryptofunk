# Explainability Dashboard - TypeScript API Client

This directory contains TypeScript type definitions and API client for the LLM Decision Explainability Dashboard.

## Structure

```
src/
├── types/
│   ├── decision.ts       # TypeScript type definitions
│   └── index.ts          # Barrel export
├── api/
│   ├── decisions.ts      # API client functions
│   └── index.ts          # Barrel export
└── hooks/
    ├── useDecisions.ts   # React Query hooks
    └── index.ts          # Barrel export
```

## Type Definitions (`types/decision.ts`)

All types are based on the Go structs in `/internal/api/decisions.go`:

- `Decision` - Main decision record
- `DecisionFilter` - Filter options for listing
- `DecisionStats` - Aggregated statistics
- `SearchResult` - Search result with relevance score
- `ListDecisionsResponse` - List endpoint response
- `SearchDecisionsResponse` - Search endpoint response
- `SimilarDecisionsResponse` - Similar decisions response

## API Client (`api/decisions.ts`)

The API client provides these functions:

### `listDecisions(filter?: DecisionFilter): Promise<ListDecisionsResponse>`
List decisions with optional filtering and pagination.

**Example:**
```typescript
import { listDecisions } from './api/decisions';

const { decisions, count } = await listDecisions({
  symbol: 'BTCUSDT',
  outcome: 'SUCCESS',
  limit: 50,
  offset: 0
});
```

### `getDecision(id: string): Promise<Decision>`
Get a single decision by UUID.

**Example:**
```typescript
import { getDecision } from './api/decisions';

const decision = await getDecision('123e4567-e89b-12d3-a456-426614174000');
```

### `getDecisionStats(filter?: DecisionFilter): Promise<DecisionStats>`
Get aggregated statistics.

**Example:**
```typescript
import { getDecisionStats } from './api/decisions';

const stats = await getDecisionStats({ symbol: 'BTCUSDT' });
console.log(`Success rate: ${stats.success_rate * 100}%`);
```

### `searchDecisions(query: string, limit?: number): Promise<SearchDecisionsResponse>`
Search decisions using text or semantic search.

**Example:**
```typescript
import { searchDecisions } from './api/decisions';

const { results, count } = await searchDecisions('bullish signal', 20);
results.forEach(({ decision, score }) => {
  console.log(`Match: ${decision.response} (score: ${score})`);
});
```

### `getSimilarDecisions(id: string, limit?: number): Promise<SimilarDecisionsResponse>`
Find similar decisions using vector similarity.

**Example:**
```typescript
import { getSimilarDecisions } from './api/decisions';

const { similar, count } = await getSimilarDecisions('123e4567-...', 10);
```

## React Query Hooks (`hooks/useDecisions.ts`)

All hooks use `@tanstack/react-query` for caching, automatic refetching, and state management.

### `useDecisions(filter?: DecisionFilter, options?)`
Hook for listing decisions.

**Example:**
```typescript
import { useDecisions } from './hooks/useDecisions';

function DecisionList() {
  const { data, isLoading, error } = useDecisions({
    symbol: 'BTCUSDT',
    limit: 50
  });

  if (isLoading) return <div>Loading...</div>;
  if (error) return <div>Error: {error.message}</div>;

  return (
    <ul>
      {data.decisions.map(decision => (
        <li key={decision.id}>{decision.response}</li>
      ))}
    </ul>
  );
}
```

### `useDecision(id: string, options?)`
Hook for getting a single decision.

**Example:**
```typescript
import { useDecision } from './hooks/useDecisions';

function DecisionDetail({ id }: { id: string }) {
  const { data, isLoading } = useDecision(id);

  if (isLoading) return <div>Loading...</div>;
  if (!data) return <div>Not found</div>;

  return (
    <div>
      <h2>{data.decision_type}</h2>
      <p>{data.response}</p>
    </div>
  );
}
```

### `useDecisionStats(filter?, options?)`
Hook for getting statistics.

**Example:**
```typescript
import { useDecisionStats } from './hooks/useDecisions';

function StatsCard() {
  const { data, isLoading } = useDecisionStats({
    symbol: 'BTCUSDT'
  });

  if (isLoading) return <div>Loading...</div>;

  return (
    <div>
      <p>Total: {data.total_decisions}</p>
      <p>Success Rate: {(data.success_rate * 100).toFixed(2)}%</p>
      <p>Avg Confidence: {(data.avg_confidence * 100).toFixed(2)}%</p>
    </div>
  );
}
```

### `useSearchDecisions()`
Mutation hook for searching (manual trigger).

**Example:**
```typescript
import { useSearchDecisions } from './hooks/useDecisions';

function SearchForm() {
  const searchMutation = useSearchDecisions();

  const handleSubmit = (query: string) => {
    searchMutation.mutate({ query, limit: 20 });
  };

  return (
    <div>
      {searchMutation.isLoading && <div>Searching...</div>}
      {searchMutation.data && (
        <ul>
          {searchMutation.data.results.map(({ decision, score }) => (
            <li key={decision.id}>
              {decision.response} (score: {score.toFixed(2)})
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
```

### `useSearchDecisionsQuery(query: string, limit?, options?)`
Query hook for searching (automatic execution).

**Example:**
```typescript
import { useSearchDecisionsQuery } from './hooks/useDecisions';

function SearchResults({ query }: { query: string }) {
  const { data, isLoading } = useSearchDecisionsQuery(query, 20);

  if (isLoading) return <div>Searching...</div>;
  if (!data) return null;

  return (
    <ul>
      {data.results.map(({ decision, score }) => (
        <li key={decision.id}>
          {decision.response} (score: {score.toFixed(2)})
        </li>
      ))}
    </ul>
  );
}
```

### `useSimilarDecisions(id: string, limit?, options?)`
Hook for finding similar decisions.

**Example:**
```typescript
import { useSimilarDecisions } from './hooks/useDecisions';

function SimilarDecisions({ id }: { id: string }) {
  const { data, isLoading } = useSimilarDecisions(id, 10);

  if (isLoading) return <div>Loading...</div>;

  return (
    <div>
      <h3>Similar Decisions</h3>
      <ul>
        {data.similar.map(decision => (
          <li key={decision.id}>{decision.response}</li>
        ))}
      </ul>
    </div>
  );
}
```

### `useInvalidateDecisions()`
Hook to manually invalidate cached queries.

**Example:**
```typescript
import { useInvalidateDecisions } from './hooks/useDecisions';

function RefreshButton() {
  const invalidate = useInvalidateDecisions();

  return (
    <button onClick={() => invalidate()}>
      Refresh All Data
    </button>
  );
}
```

## Configuration

The API client reads the base URL from the environment variable (Vite convention):

```env
VITE_API_BASE_URL=http://localhost:8080
```

Default: Empty string (uses relative URLs, works with Vite proxy or same-origin deployment)

See `.env.example` for all available configuration options.

## Error Handling

All API functions throw `DecisionApiError` on failure:

```typescript
import { DecisionApiError, getDecision } from './api/decisions';

try {
  const decision = await getDecision(id);
} catch (error) {
  if (error instanceof DecisionApiError) {
    console.error(`API Error (${error.status}): ${error.message}`);
  }
}
```

React Query hooks automatically handle errors and provide them in the `error` property:

```typescript
const { data, error } = useDecisions();

if (error) {
  console.error('Query failed:', error.message);
}
```

## Caching Strategy

React Query hooks use these default caching settings:

- **List queries**: 30s stale time, 5min cache time
- **Detail queries**: 1min stale time, 10min cache time
- **Stats queries**: 1min stale time, 5min cache time
- **Search queries**: 2min stale time, 5min cache time
- **Similar queries**: 5min stale time, 10min cache time

You can override these with the `options` parameter.

## Query Keys

All query keys are exported from `decisionKeys`:

```typescript
import { decisionKeys } from './hooks/useDecisions';

// Use in custom queries or mutations
const queryKey = decisionKeys.list({ symbol: 'BTCUSDT' });
```

## Testing

Example test with React Query:

```typescript
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useDecisions } from './hooks/useDecisions';

test('useDecisions fetches data', async () => {
  const queryClient = new QueryClient();
  const wrapper = ({ children }) => (
    <QueryClientProvider client={queryClient}>
      {children}
    </QueryClientProvider>
  );

  const { result } = renderHook(() => useDecisions({ limit: 10 }), { wrapper });

  await waitFor(() => expect(result.current.isSuccess).toBe(true));
  expect(result.current.data.decisions).toHaveLength(10);
});
```

## Backend API Endpoints

The TypeScript client expects these REST endpoints on the Go backend:

- `GET /api/v1/decisions` - List decisions
- `GET /api/v1/decisions/:id` - Get single decision
- `GET /api/v1/decisions/stats` - Get statistics
- `GET /api/v1/decisions/search` - Search decisions
- `GET /api/v1/decisions/:id/similar` - Get similar decisions

These endpoints need to be implemented in `/cmd/api/main.go`.
