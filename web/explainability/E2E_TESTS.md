# E2E Tests for Explainability Dashboard

## Overview

This document describes the End-to-End (E2E) test suite for the CryptoFunk Explainability Dashboard. The tests verify the complete user experience across all dashboard features.

## Test Framework

The E2E tests use **Vitest** with **React Testing Library** for integration-style testing. This approach provides:

- Fast execution (runs in jsdom, no browser overhead)
- Excellent component integration testing
- User-centric testing approach (tests what users see and interact with)
- Built-in mocking capabilities
- Coverage reporting

## Test File Location

```
src/e2e/dashboard.e2e.test.tsx
```

## Running the Tests

### Run all tests (including E2E)
```bash
npm run test
```

### Run only the E2E tests
```bash
npm run test -- src/e2e/dashboard.e2e.test.tsx
```

### Run tests in watch mode (for development)
```bash
npm run test -- --watch
```

### Run tests with coverage
```bash
npm run test:coverage
```

### Run tests once (CI mode)
```bash
npm run test:run
```

## Test Coverage

The E2E test suite covers the following scenarios:

### 1. Dashboard Loads Successfully
- ✅ Renders header and navigation
- ✅ Displays decisions in timeline by default
- ✅ Shows correct decision count

### 2. Timeline Interactions
- ✅ Can scroll through decision timeline (Load More)
- ✅ Can click on a decision to view details
- ✅ Displays loading skeletons while loading

### 3. Search Functionality
- ✅ Can navigate to search tab
- ✅ Can enter search query and search
- ✅ Can search by pressing Enter key
- ✅ Shows relevant results with similarity scores
- ✅ Shows no results message when search returns empty

### 4. Filters
- ✅ Can filter by agent name
- ✅ Can filter by symbol
- ✅ Can filter by outcome
- ✅ Can clear all filters
- ✅ Filter changes trigger data refetch (debounced)

### 5. Stats View
- ✅ Displays statistics correctly
- ✅ Can refresh statistics
- ✅ Shows loading state while fetching stats

### 6. Detail Modal
- ✅ Modal opens when clicking decision
- ✅ Modal closes on escape key
- ✅ Modal closes on close button click
- ✅ Can expand and collapse prompt section
- ✅ Can expand and collapse response section
- ✅ Can find similar decisions from modal
- ✅ Displays all decision metrics

### 7. Pagination/Infinite Scroll
- ✅ Loads more decisions when clicking Load More
- ✅ Shows loading state while fetching next page
- ✅ Hides Load More button when no more pages
- ✅ Shows correct count across multiple pages

### 8. Error Handling
- ✅ Displays error state when timeline fails to load
- ✅ Can retry after error
- ✅ Displays error state in stats view
- ✅ Displays error in search
- ✅ Shows empty state when no decisions exist

### 9. Navigation and State Management
- ✅ Preserves selected decision when switching tabs
- ✅ Maintains filter state when switching tabs

### 10. Accessibility
- ✅ Has proper ARIA landmarks (banner, navigation, main)
- ✅ Timeline has proper ARIA attributes
- ✅ Modal has focus trap
- ✅ Filter inputs have proper labels and descriptions

## Test Data

The tests use realistic mock data including:

- **3 mock decisions** covering BUY_SIGNAL, SELL_SIGNAL, and RISK_APPROVAL
- **Varied outcomes**: SUCCESS, FAILURE, PENDING
- **Multiple agents**: Technical Agent, Trend Agent, Risk Agent
- **Multiple symbols**: BTC/USDT, ETH/USDT, SOL/USDT
- **Comprehensive stats** with aggregated metrics

## Test Architecture

### Mocking Strategy

The tests mock the React hooks from `src/hooks/useDecisions.ts`:

```typescript
- useInfiniteDecisions  // Timeline data with pagination
- useDecision          // Single decision details
- useSearchDecisions   // Semantic search
- useDecisionStats     // Statistics view
```

This allows testing the full component tree while controlling data flow.

### Helper Functions

```typescript
// Creates a fresh QueryClient for each test
createTestQueryClient()

// Renders App with QueryClient provider
renderApp()
```

### Test Structure

Each test:
1. Sets up mock responses
2. Renders the complete App component
3. Simulates user interactions (clicks, typing, navigation)
4. Asserts on visible UI elements
5. Cleans up mocks

## CI/CD Integration

### GitHub Actions Example

```yaml
- name: Run E2E Tests
  run: npm run test:run

- name: Upload Coverage
  uses: codecov/codecov-action@v3
  with:
    files: ./coverage/coverage-final.json
```

### Pre-commit Hook

Add to `.git/hooks/pre-commit`:

```bash
#!/bin/bash
npm run test:run
if [ $? -ne 0 ]; then
  echo "Tests failed. Commit aborted."
  exit 1
fi
```

## Debugging Tests

### Run specific test
```bash
npm run test -- -t "modal opens when clicking decision"
```

### Enable debug output
```bash
DEBUG=* npm run test -- src/e2e/dashboard.e2e.test.tsx
```

### Use VS Code debugger
1. Set breakpoints in test file
2. Use "Debug Test" CodeLens above test
3. Or use Jest Runner extension

### Common Issues

**Test timeout**: Increase timeout in vitest.config.ts
```typescript
test: {
  testTimeout: 10000,
}
```

**Mock not working**: Ensure mock is set up in `beforeEach`
```typescript
beforeEach(() => {
  vi.clearAllMocks();
  // Set up your mocks here
});
```

**Element not found**: Use `waitFor` for async updates
```typescript
await waitFor(() => {
  expect(screen.getByText('Expected Text')).toBeInTheDocument();
});
```

## Future Enhancements

### Potential Additions

1. **Visual Regression Testing**
   - Add Playwright for screenshot comparison
   - Catch unintended UI changes

2. **Performance Testing**
   - Measure render times
   - Test with large datasets (1000+ decisions)
   - Memory leak detection

3. **Real API Testing**
   - Add tests against actual backend (staging environment)
   - Test with real database

4. **Mobile Testing**
   - Test responsive design
   - Touch interactions
   - Mobile viewport sizes

5. **Network Condition Testing**
   - Slow 3G simulation
   - Offline mode
   - Request throttling

## Comparison: Vitest/RTL vs Playwright

### Current Approach (Vitest + React Testing Library)

**Pros:**
- ✅ Fast execution (no browser startup)
- ✅ Easy to set up and maintain
- ✅ Great for component integration
- ✅ Runs in CI without browser dependencies
- ✅ Same test framework as unit tests

**Cons:**
- ❌ No real browser testing
- ❌ Can't test browser-specific bugs
- ❌ No visual regression testing
- ❌ Limited real user interaction simulation

### Playwright (If Needed)

**When to add Playwright:**
- Critical user flows require browser testing
- Need cross-browser compatibility testing
- Visual regression testing is important
- Testing complex animations/transitions
- Need to test with real backend

**Setup:**
```bash
npm install -D @playwright/test
npx playwright install
```

**Example Playwright test:**
```typescript
import { test, expect } from '@playwright/test';

test('dashboard loads and displays decisions', async ({ page }) => {
  await page.goto('http://localhost:5173');
  await expect(page.getByText('CryptoFunk Explainability Dashboard')).toBeVisible();
  await expect(page.getByText('Technical Agent')).toBeVisible();
});
```

## Test Results

All 32 E2E tests are passing successfully:

```
✓ src/e2e/dashboard.e2e.test.tsx  (32 tests) 755ms

Test Files  1 passed (1)
Tests  32 passed (32)
```

### Test Breakdown

- **Dashboard Loading** (3 tests) - ✅ All passing
- **Timeline Interactions** (3 tests) - ✅ All passing
- **Search Functionality** (5 tests) - ✅ All passing
- **Filters** (4 tests) - ✅ All passing
- **Stats View** (3 tests) - ✅ All passing
- **Pagination/Infinite Scroll** (4 tests) - ✅ All passing
- **Error Handling** (5 tests) - ✅ All passing
- **Navigation & State** (2 tests) - ✅ All passing
- **Accessibility** (3 tests) - ✅ All passing

## Conclusion

The current E2E test suite provides comprehensive coverage of the dashboard functionality using a fast, maintainable approach. The tests are integrated into the existing Vitest setup and run alongside unit tests.

**Key Benefits:**
- Fast execution (< 1 second)
- No browser dependencies
- Easy to debug and maintain
- Integrated with existing test infrastructure
- Comprehensive coverage of user flows

For most use cases, this approach is sufficient. Consider adding Playwright only if you need browser-specific testing or visual regression testing.

## Questions?

For issues or questions about the test suite, contact the development team or open an issue in the repository.
