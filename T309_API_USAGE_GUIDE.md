# T309 Decision Feedback API - Usage Guide

## Quick Start

The Decision Feedback API allows users to rate trading decisions and provide feedback to improve prompt engineering.

---

## API Endpoints Summary

| Method | Endpoint | Purpose | Rate Limit |
|--------|----------|---------|------------|
| POST | `/api/v1/decisions/:id/feedback` | Submit feedback | Write |
| GET | `/api/v1/decisions/:id/feedback` | Get decision feedback | Read |
| GET | `/api/v1/feedback` | List all feedback | Read |
| GET | `/api/v1/feedback/stats` | Get statistics | Read |
| GET | `/api/v1/feedback/review` | Get low-rated decisions | Read |
| GET | `/api/v1/feedback/tags` | Get common tags | Read |
| GET | `/api/v1/feedback/:id` | Get single feedback | Read |
| PUT | `/api/v1/feedback/:id` | Update feedback | Write |
| DELETE | `/api/v1/feedback/:id` | Delete feedback | Write |
| POST | `/api/v1/feedback/refresh-stats` | Refresh stats view | Write |

---

## 1. Submit Feedback (Thumbs Up/Down)

### Endpoint
```
POST /api/v1/decisions/:id/feedback
```

### Request
```bash
curl -X POST http://localhost:8080/api/v1/decisions/550e8400-e29b-41d4-a716-446655440000/feedback \
  -H "Content-Type: application/json" \
  -d '{
    "rating": "positive",
    "comment": "Great prediction on BTC rally",
    "tags": ["accurate_prediction", "good_entry"]
  }'
```

### Request Body
```json
{
  "rating": "positive",           // Required: "positive" or "negative"
  "comment": "Optional comment",  // Optional: max 2000 characters
  "tags": ["tag1", "tag2"],       // Optional: max 20 tags
  "user_id": "uuid"               // Optional: for authenticated users
}
```

### Response (201 Created)
```json
{
  "id": "feedback-uuid",
  "decision_id": "decision-uuid",
  "user_id": "user-uuid",
  "rating": "positive",
  "comment": "Great prediction on BTC rally",
  "tags": ["accurate_prediction", "good_entry"],
  "created_at": "2024-11-28T10:00:00Z",
  "updated_at": "2024-11-28T10:00:00Z",
  "session_id": "session-uuid",
  "symbol": "BTC/USDT",
  "decision_type": "signal",
  "agent_name": "technical-agent"
}
```

### Common Tags
```
Negative:
- wrong_direction    - Predicted wrong market direction
- bad_timing         - Poor entry/exit timing
- risk_too_high      - Too aggressive
- risk_too_low       - Too conservative
- unclear_reasoning  - Unclear explanation

Positive:
- good_entry         - Excellent entry point
- good_exit          - Excellent exit point
- accurate_prediction - Correct market prediction
- helpful_explanation - Clear reasoning
```

### Error Responses
```json
// 400 Bad Request - Invalid rating
{
  "error": "Rating must be 'positive' or 'negative'"
}

// 400 Bad Request - Comment too long
{
  "error": "Comment too long, maximum 2000 characters allowed"
}

// 404 Not Found - Decision doesn't exist
{
  "error": "Decision not found"
}

// 409 Conflict - Duplicate feedback
{
  "error": "Feedback already submitted for this decision by this user"
}
```

---

## 2. Get Feedback for a Decision

### Endpoint
```
GET /api/v1/decisions/:id/feedback
```

### Request
```bash
curl http://localhost:8080/api/v1/decisions/550e8400-e29b-41d4-a716-446655440000/feedback
```

### Response (200 OK)
```json
{
  "decision_id": "550e8400-e29b-41d4-a716-446655440000",
  "feedback": [
    {
      "id": "fb-uuid-1",
      "rating": "positive",
      "comment": "Great call!",
      "tags": ["accurate_prediction"],
      "created_at": "2024-11-28T10:00:00Z"
    },
    {
      "id": "fb-uuid-2",
      "rating": "negative",
      "comment": "Wrong direction",
      "tags": ["wrong_direction"],
      "created_at": "2024-11-28T11:00:00Z"
    }
  ],
  "count": 2,
  "summary": {
    "positive": 1,
    "negative": 1
  }
}
```

---

## 3. Get Feedback Statistics

### Endpoint
```
GET /api/v1/feedback/stats
```

### Query Parameters
- `agent_name` - Filter by agent (e.g., "technical-agent")
- `symbol` - Filter by symbol (e.g., "BTC/USDT")
- `from_date` - Start date (RFC3339)
- `to_date` - End date (RFC3339)

### Request
```bash
# All stats
curl http://localhost:8080/api/v1/feedback/stats

# Stats for specific agent
curl http://localhost:8080/api/v1/feedback/stats?agent_name=technical-agent

# Stats for specific symbol
curl http://localhost:8080/api/v1/feedback/stats?symbol=BTC/USDT
```

### Response (200 OK)
```json
{
  "total_feedback": 100,
  "positive_count": 70,
  "negative_count": 30,
  "positive_rate": 70.0,
  "by_agent": {
    "technical-agent": {
      "positive_count": 50,
      "negative_count": 10,
      "total": 60,
      "positive_rate": 83.33
    },
    "trend-agent": {
      "positive_count": 20,
      "negative_count": 20,
      "total": 40,
      "positive_rate": 50.0
    }
  },
  "by_decision_type": {
    "signal": 60,
    "risk_approval": 40
  },
  "by_symbol": {
    "BTC/USDT": 50,
    "ETH/USDT": 50
  },
  "top_tags": [
    {
      "tag": "accurate_prediction",
      "count": 30
    },
    {
      "tag": "good_entry",
      "count": 25
    },
    {
      "tag": "wrong_direction",
      "count": 15
    }
  ],
  "recent_trend": [
    {
      "date": "2024-11-28",
      "positive_count": 10,
      "negative_count": 5
    },
    {
      "date": "2024-11-27",
      "positive_count": 8,
      "negative_count": 6
    }
  ]
}
```

---

## 4. Get Decisions Needing Review (Low-Rated)

### Endpoint
```
GET /api/v1/feedback/review
```

### Query Parameters
- `limit` - Number of results (default: 20, max: 100)

### Request
```bash
curl http://localhost:8080/api/v1/feedback/review?limit=10
```

### Response (200 OK)
```json
{
  "decisions": [
    {
      "decision_id": "decision-uuid",
      "agent_name": "technical-agent",
      "decision_type": "signal",
      "symbol": "BTC/USDT",
      "prompt": "Full prompt text for review...",
      "response": "Full LLM response...",
      "confidence": 0.75,
      "outcome": "FAILURE",
      "outcome_pnl": -50.0,
      "decision_created_at": "2024-11-28T09:00:00Z",
      "feedback_count": 5,
      "negative_count": 4,
      "positive_count": 1,
      "comments": [
        "Predicted uptrend but market crashed",
        "Wrong timing on entry",
        "Risk was too high for this setup"
      ],
      "all_tags": [
        "wrong_direction",
        "bad_timing",
        "risk_too_high"
      ]
    }
  ],
  "count": 10
}
```

**Use Case:** Identify decisions with 2+ negative ratings for prompt engineering improvements

---

## 5. List All Feedback (with Filters)

### Endpoint
```
GET /api/v1/feedback
```

### Query Parameters
- `rating` - Filter by 'positive' or 'negative'
- `agent_name` - Filter by agent
- `symbol` - Filter by symbol
- `decision_type` - Filter by decision type
- `from_date` - Start date (RFC3339)
- `to_date` - End date (RFC3339)
- `limit` - Results per page (default: 50, max: 200)
- `offset` - Pagination offset

### Request
```bash
# Get all negative feedback
curl "http://localhost:8080/api/v1/feedback?rating=negative&limit=20"

# Get feedback for specific agent
curl "http://localhost:8080/api/v1/feedback?agent_name=technical-agent"

# Get feedback with pagination
curl "http://localhost:8080/api/v1/feedback?limit=50&offset=100"

# Get recent negative feedback for BTC
curl "http://localhost:8080/api/v1/feedback?rating=negative&symbol=BTC/USDT&limit=10"
```

### Response (200 OK)
```json
{
  "feedback": [
    {
      "id": "fb-uuid",
      "decision_id": "decision-uuid",
      "rating": "negative",
      "comment": "Wrong prediction",
      "tags": ["wrong_direction"],
      "created_at": "2024-11-28T10:00:00Z",
      "symbol": "BTC/USDT",
      "agent_name": "technical-agent"
    }
  ],
  "count": 1,
  "filter": {
    "Rating": "negative",
    "Symbol": "BTC/USDT",
    "Limit": 10,
    "Offset": 0
  }
}
```

---

## 6. Update Feedback

### Endpoint
```
PUT /api/v1/feedback/:id
```

### Request
```bash
curl -X PUT http://localhost:8080/api/v1/feedback/fb-uuid \
  -H "Content-Type: application/json" \
  -d '{
    "rating": "negative",
    "comment": "Updated: Actually this was wrong",
    "tags": ["wrong_direction"]
  }'
```

### Request Body (all fields optional)
```json
{
  "rating": "negative",
  "comment": "Updated comment",
  "tags": ["new_tag"]
}
```

### Response (200 OK)
Returns updated feedback object

---

## 7. Delete Feedback

### Endpoint
```
DELETE /api/v1/feedback/:id
```

### Request
```bash
curl -X DELETE http://localhost:8080/api/v1/feedback/fb-uuid
```

### Response (204 No Content)
No body returned on success

---

## 8. Get Common Tags

### Endpoint
```
GET /api/v1/feedback/tags
```

### Request
```bash
curl http://localhost:8080/api/v1/feedback/tags
```

### Response (200 OK)
```json
{
  "tags": [
    "wrong_direction",
    "bad_timing",
    "risk_too_high",
    "risk_too_low",
    "missed_opportunity",
    "good_entry",
    "good_exit",
    "accurate_prediction",
    "unclear_reasoning",
    "helpful_explanation"
  ]
}
```

---

## 9. Refresh Statistics (Admin Only)

### Endpoint
```
POST /api/v1/feedback/refresh-stats
```

### Request
```bash
curl -X POST http://localhost:8080/api/v1/feedback/refresh-stats
```

### Response (200 OK)
```json
{
  "message": "Feedback statistics refreshed successfully",
  "refreshed_at": "2024-11-28T12:00:00Z"
}
```

**Note:** This refreshes the materialized view for up-to-date statistics. Use sparingly (e.g., once per hour).

---

## Integration Examples

### JavaScript/TypeScript (Fetch API)
```javascript
// Submit positive feedback
async function submitFeedback(decisionId, rating, comment, tags) {
  const response = await fetch(
    `http://localhost:8080/api/v1/decisions/${decisionId}/feedback`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ rating, comment, tags })
    }
  );

  if (!response.ok) {
    const error = await response.json();
    throw new Error(error.error);
  }

  return response.json();
}

// Usage
try {
  const feedback = await submitFeedback(
    'decision-uuid',
    'positive',
    'Great call!',
    ['accurate_prediction']
  );
  console.log('Feedback submitted:', feedback);
} catch (error) {
  console.error('Failed to submit feedback:', error.message);
}
```

### Python (Requests)
```python
import requests

# Submit negative feedback
def submit_feedback(decision_id, rating, comment=None, tags=None):
    url = f"http://localhost:8080/api/v1/decisions/{decision_id}/feedback"
    payload = {"rating": rating}

    if comment:
        payload["comment"] = comment
    if tags:
        payload["tags"] = tags

    response = requests.post(url, json=payload)
    response.raise_for_status()
    return response.json()

# Usage
try:
    feedback = submit_feedback(
        decision_id='decision-uuid',
        rating='negative',
        comment='Wrong direction',
        tags=['wrong_direction', 'bad_timing']
    )
    print(f"Feedback submitted: {feedback['id']}")
except requests.HTTPError as e:
    print(f"Error: {e.response.json()['error']}")
```

### Go
```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

type FeedbackRequest struct {
    Rating  string   `json:"rating"`
    Comment *string  `json:"comment,omitempty"`
    Tags    []string `json:"tags,omitempty"`
}

func submitFeedback(decisionID, rating string, comment *string, tags []string) error {
    url := fmt.Sprintf("http://localhost:8080/api/v1/decisions/%s/feedback", decisionID)

    req := FeedbackRequest{
        Rating:  rating,
        Comment: comment,
        Tags:    tags,
    }

    body, _ := json.Marshal(req)
    resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusCreated {
        var errResp map[string]string
        json.NewDecoder(resp.Body).Decode(&errResp)
        return fmt.Errorf("API error: %s", errResp["error"])
    }

    return nil
}
```

---

## Rate Limiting

All endpoints are rate-limited to prevent abuse:

- **Read endpoints** (GET): Standard read rate limit
- **Write endpoints** (POST, PUT, DELETE): Stricter write rate limit
- **Stats/Search endpoints**: Moderate rate limit for expensive operations

If you exceed the rate limit, you'll receive a `429 Too Many Requests` response.

---

## Best Practices

1. **Always provide feedback context:**
   - Use tags to categorize issues
   - Add comments explaining the rating
   - Be specific about what went wrong or right

2. **Use appropriate tags:**
   - Refer to the common tags list
   - Be consistent with tag naming
   - Max 20 tags per feedback

3. **Review flagged decisions:**
   - Query `/api/v1/feedback/review` regularly
   - Focus on decisions with multiple negative ratings
   - Use feedback to improve prompts

4. **Monitor statistics:**
   - Track positive rate by agent
   - Identify problematic symbols or decision types
   - Watch trends over time

5. **Handle errors gracefully:**
   - Check for 409 (duplicate) before retrying
   - Validate decision ID exists before submitting
   - Respect rate limits

---

## Troubleshooting

### 400 Bad Request
- Check that rating is 'positive' or 'negative'
- Verify comment length is ≤ 2000 characters
- Ensure ≤ 20 tags, each ≤ 100 characters
- Make sure decision ID is a valid UUID

### 404 Not Found
- Decision ID doesn't exist in database
- Verify the UUID is correct
- Check that the decision hasn't been deleted

### 409 Conflict
- User has already submitted feedback for this decision
- Use PUT endpoint to update existing feedback instead
- Or delete old feedback first with DELETE endpoint

### 429 Too Many Requests
- You've exceeded the rate limit
- Wait before retrying
- Implement exponential backoff in your client

---

## Support

For issues or questions:
- Check the implementation summary: `T309_IMPLEMENTATION_SUMMARY.md`
- Review the test file: `internal/api/feedback_test.go`
- Consult the migration: `migrations/007_decision_feedback.sql`
