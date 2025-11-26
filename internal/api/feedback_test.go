package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Struct Tests
// =============================================================================

// TestFeedback validates Feedback struct
func TestFeedback(t *testing.T) {
	id := uuid.New()
	decisionID := uuid.New()
	userID := uuid.New()
	sessionID := uuid.New()
	comment := "Great prediction!"
	symbol := "BTC/USDT"
	decisionType := "signal"
	agentName := "technical-agent"

	feedback := Feedback{
		ID:           id,
		DecisionID:   decisionID,
		UserID:       &userID,
		Rating:       FeedbackPositive,
		Comment:      &comment,
		Tags:         []string{"accurate_prediction", "good_timing"},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		SessionID:    &sessionID,
		Symbol:       &symbol,
		DecisionType: &decisionType,
		AgentName:    &agentName,
	}

	assert.Equal(t, id, feedback.ID)
	assert.Equal(t, decisionID, feedback.DecisionID)
	assert.Equal(t, &userID, feedback.UserID)
	assert.Equal(t, FeedbackPositive, feedback.Rating)
	assert.Equal(t, "Great prediction!", *feedback.Comment)
	assert.Len(t, feedback.Tags, 2)
	assert.Equal(t, "BTC/USDT", *feedback.Symbol)
	assert.Equal(t, "signal", *feedback.DecisionType)
	assert.Equal(t, "technical-agent", *feedback.AgentName)
}

// TestFeedbackRating validates FeedbackRating constants
func TestFeedbackRating(t *testing.T) {
	assert.Equal(t, FeedbackRating("positive"), FeedbackPositive)
	assert.Equal(t, FeedbackRating("negative"), FeedbackNegative)
}

// TestFeedbackFilter validates FeedbackFilter struct
func TestFeedbackFilter(t *testing.T) {
	now := time.Now()
	decisionID := uuid.New()
	userID := uuid.New()
	rating := FeedbackPositive

	filter := FeedbackFilter{
		DecisionID:   &decisionID,
		UserID:       &userID,
		Rating:       &rating,
		AgentName:    "technical-agent",
		Symbol:       "BTC/USDT",
		DecisionType: "signal",
		FromDate:     &now,
		ToDate:       &now,
		Limit:        50,
		Offset:       10,
	}

	assert.Equal(t, &decisionID, filter.DecisionID)
	assert.Equal(t, &userID, filter.UserID)
	assert.Equal(t, &rating, filter.Rating)
	assert.Equal(t, "technical-agent", filter.AgentName)
	assert.Equal(t, "BTC/USDT", filter.Symbol)
	assert.Equal(t, "signal", filter.DecisionType)
	assert.Equal(t, 50, filter.Limit)
	assert.Equal(t, 10, filter.Offset)
}

// TestFeedbackStats validates FeedbackStats struct
func TestFeedbackStats(t *testing.T) {
	stats := FeedbackStats{
		TotalFeedback: 100,
		PositiveCount: 70,
		NegativeCount: 30,
		PositiveRate:  70.0,
		ByAgent: map[string]AgentFeedbackStats{
			"technical-agent": {
				PositiveCount: 50,
				NegativeCount: 10,
				Total:         60,
				PositiveRate:  83.33,
			},
		},
		ByDecisionType: map[string]int{
			"signal":        60,
			"risk_approval": 40,
		},
		BySymbol: map[string]int{
			"BTC/USDT": 50,
			"ETH/USDT": 50,
		},
		TopTags: []TagCount{
			{Tag: "accurate_prediction", Count: 30},
			{Tag: "good_timing", Count: 25},
		},
		RecentTrend: []DailyFeedback{
			{Date: "2024-01-15", PositiveCount: 10, NegativeCount: 5},
		},
	}

	assert.Equal(t, 100, stats.TotalFeedback)
	assert.Equal(t, 70, stats.PositiveCount)
	assert.Equal(t, 30, stats.NegativeCount)
	assert.Equal(t, 70.0, stats.PositiveRate)
	assert.Equal(t, 60, stats.ByAgent["technical-agent"].Total)
	assert.Equal(t, 60, stats.ByDecisionType["signal"])
	assert.Equal(t, 50, stats.BySymbol["BTC/USDT"])
	assert.Len(t, stats.TopTags, 2)
	assert.Equal(t, "accurate_prediction", stats.TopTags[0].Tag)
}

// TestDecisionNeedingReview validates DecisionNeedingReview struct
func TestDecisionNeedingReview(t *testing.T) {
	decisionID := uuid.New()
	agentName := "technical-agent"
	confidence := 0.75
	outcome := "FAILURE"
	pnl := -50.0

	decision := DecisionNeedingReview{
		DecisionID:        decisionID,
		AgentName:         &agentName,
		DecisionType:      "signal",
		Symbol:            "BTC/USDT",
		Prompt:            "Test prompt",
		Response:          "Test response",
		Confidence:        &confidence,
		Outcome:           &outcome,
		OutcomePnL:        &pnl,
		DecisionCreatedAt: time.Now(),
		FeedbackCount:     5,
		NegativeCount:     4,
		PositiveCount:     1,
		Comments:          []string{"Wrong direction", "Bad timing"},
		AllTags:           []string{"wrong_direction", "bad_timing"},
	}

	assert.Equal(t, decisionID, decision.DecisionID)
	assert.Equal(t, "technical-agent", *decision.AgentName)
	assert.Equal(t, "signal", decision.DecisionType)
	assert.Equal(t, 5, decision.FeedbackCount)
	assert.Equal(t, 4, decision.NegativeCount)
	assert.Len(t, decision.Comments, 2)
	assert.Len(t, decision.AllTags, 2)
}

// TestCommonFeedbackTags validates common tags list
func TestCommonFeedbackTags(t *testing.T) {
	assert.NotEmpty(t, CommonFeedbackTags)
	assert.Contains(t, CommonFeedbackTags, "wrong_direction")
	assert.Contains(t, CommonFeedbackTags, "good_entry")
	assert.Contains(t, CommonFeedbackTags, "accurate_prediction")
}

// TestFeedbackConstants validates constants
func TestFeedbackConstants(t *testing.T) {
	assert.Equal(t, 50, DefaultFeedbackLimit)
	assert.Equal(t, 200, MaxFeedbackLimit)
}

// =============================================================================
// Mock Repository
// =============================================================================

// MockFeedbackRepository for unit testing handlers without database
type MockFeedbackRepository struct {
	CreateFeedbackFunc            func(ctx context.Context, req CreateFeedbackRequest) (*Feedback, error)
	GetFeedbackFunc               func(ctx context.Context, id uuid.UUID) (*Feedback, error)
	GetFeedbackByDecisionFunc     func(ctx context.Context, decisionID uuid.UUID) ([]Feedback, error)
	ListFeedbackFunc              func(ctx context.Context, filter FeedbackFilter) ([]Feedback, error)
	UpdateFeedbackFunc            func(ctx context.Context, id uuid.UUID, req UpdateFeedbackRequest) (*Feedback, error)
	DeleteFeedbackFunc            func(ctx context.Context, id uuid.UUID) error
	GetFeedbackStatsFunc          func(ctx context.Context, filter FeedbackFilter) (*FeedbackStats, error)
	GetDecisionsNeedingReviewFunc func(ctx context.Context, limit int) ([]DecisionNeedingReview, error)
	RefreshStatsViewFunc          func(ctx context.Context) error
}

func (m *MockFeedbackRepository) CreateFeedback(ctx context.Context, req CreateFeedbackRequest) (*Feedback, error) {
	if m.CreateFeedbackFunc != nil {
		return m.CreateFeedbackFunc(ctx, req)
	}
	return &Feedback{}, nil
}

func (m *MockFeedbackRepository) GetFeedback(ctx context.Context, id uuid.UUID) (*Feedback, error) {
	if m.GetFeedbackFunc != nil {
		return m.GetFeedbackFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockFeedbackRepository) GetFeedbackByDecision(ctx context.Context, decisionID uuid.UUID) ([]Feedback, error) {
	if m.GetFeedbackByDecisionFunc != nil {
		return m.GetFeedbackByDecisionFunc(ctx, decisionID)
	}
	return []Feedback{}, nil
}

func (m *MockFeedbackRepository) ListFeedback(ctx context.Context, filter FeedbackFilter) ([]Feedback, error) {
	if m.ListFeedbackFunc != nil {
		return m.ListFeedbackFunc(ctx, filter)
	}
	return []Feedback{}, nil
}

func (m *MockFeedbackRepository) UpdateFeedback(ctx context.Context, id uuid.UUID, req UpdateFeedbackRequest) (*Feedback, error) {
	if m.UpdateFeedbackFunc != nil {
		return m.UpdateFeedbackFunc(ctx, id, req)
	}
	return &Feedback{}, nil
}

func (m *MockFeedbackRepository) DeleteFeedback(ctx context.Context, id uuid.UUID) error {
	if m.DeleteFeedbackFunc != nil {
		return m.DeleteFeedbackFunc(ctx, id)
	}
	return nil
}

func (m *MockFeedbackRepository) GetFeedbackStats(ctx context.Context, filter FeedbackFilter) (*FeedbackStats, error) {
	if m.GetFeedbackStatsFunc != nil {
		return m.GetFeedbackStatsFunc(ctx, filter)
	}
	return &FeedbackStats{}, nil
}

func (m *MockFeedbackRepository) GetDecisionsNeedingReview(ctx context.Context, limit int) ([]DecisionNeedingReview, error) {
	if m.GetDecisionsNeedingReviewFunc != nil {
		return m.GetDecisionsNeedingReviewFunc(ctx, limit)
	}
	return []DecisionNeedingReview{}, nil
}

func (m *MockFeedbackRepository) RefreshStatsView(ctx context.Context) error {
	if m.RefreshStatsViewFunc != nil {
		return m.RefreshStatsViewFunc(ctx)
	}
	return nil
}

// =============================================================================
// Handler Tests
// =============================================================================

// setupFeedbackTestRouter creates a router with the feedback handler using a mock repository
func setupFeedbackTestRouter(mock *MockFeedbackRepository) *gin.Engine {
	router := gin.New()
	handler := &FeedbackHandler{repo: mock}
	v1 := router.Group("/api/v1")
	handler.RegisterRoutes(v1)
	return router
}

// TestHandlerCreateFeedback tests the create feedback endpoint
func TestHandlerCreateFeedback(t *testing.T) {
	testDecisionID := uuid.New()
	testFeedbackID := uuid.New()

	mock := &MockFeedbackRepository{
		CreateFeedbackFunc: func(ctx context.Context, req CreateFeedbackRequest) (*Feedback, error) {
			if req.DecisionID == testDecisionID {
				return &Feedback{
					ID:         testFeedbackID,
					DecisionID: req.DecisionID,
					Rating:     req.Rating,
					Comment:    req.Comment,
					Tags:       req.Tags,
					CreatedAt:  time.Now(),
					UpdatedAt:  time.Now(),
				}, nil
			}
			return nil, errors.New("decision not found: " + req.DecisionID.String())
		},
	}

	router := setupFeedbackTestRouter(mock)

	t.Run("create positive feedback", func(t *testing.T) {
		comment := "Great prediction!"
		body := map[string]interface{}{
			"rating":  "positive",
			"comment": comment,
			"tags":    []string{"accurate_prediction"},
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/decisions/"+testDecisionID.String()+"/feedback", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var feedback Feedback
		err := json.Unmarshal(w.Body.Bytes(), &feedback)
		require.NoError(t, err)
		assert.Equal(t, FeedbackPositive, feedback.Rating)
	})

	t.Run("create negative feedback", func(t *testing.T) {
		body := map[string]interface{}{
			"rating":  "negative",
			"comment": "Wrong direction",
			"tags":    []string{"wrong_direction", "bad_timing"},
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/decisions/"+testDecisionID.String()+"/feedback", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("create feedback with invalid decision ID", func(t *testing.T) {
		body := map[string]interface{}{
			"rating": "positive",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/decisions/invalid-uuid/feedback", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("create feedback with invalid rating", func(t *testing.T) {
		body := map[string]interface{}{
			"rating": "neutral",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/decisions/"+testDecisionID.String()+"/feedback", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("create feedback for non-existent decision", func(t *testing.T) {
		nonExistentID := uuid.New()
		body := map[string]interface{}{
			"rating": "positive",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/decisions/"+nonExistentID.String()+"/feedback", bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// TestHandlerGetDecisionFeedback tests getting feedback for a decision
func TestHandlerGetDecisionFeedback(t *testing.T) {
	testDecisionID := uuid.New()

	testFeedbacks := []Feedback{
		{
			ID:         uuid.New(),
			DecisionID: testDecisionID,
			Rating:     FeedbackPositive,
			CreatedAt:  time.Now(),
		},
		{
			ID:         uuid.New(),
			DecisionID: testDecisionID,
			Rating:     FeedbackNegative,
			CreatedAt:  time.Now(),
		},
	}

	mock := &MockFeedbackRepository{
		GetFeedbackByDecisionFunc: func(ctx context.Context, decisionID uuid.UUID) ([]Feedback, error) {
			if decisionID == testDecisionID {
				return testFeedbacks, nil
			}
			return []Feedback{}, nil
		},
	}

	router := setupFeedbackTestRouter(mock)

	t.Run("get decision feedback", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/decisions/"+testDecisionID.String()+"/feedback", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		feedback := response["feedback"].([]interface{})
		assert.Len(t, feedback, 2)
		assert.Equal(t, float64(2), response["count"])

		summary := response["summary"].(map[string]interface{})
		assert.Equal(t, float64(1), summary["positive"])
		assert.Equal(t, float64(1), summary["negative"])
	})

	t.Run("get feedback for decision with no feedback", func(t *testing.T) {
		emptyID := uuid.New()
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/decisions/"+emptyID.String()+"/feedback", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, float64(0), response["count"])
	})
}

// TestHandlerListFeedback tests listing feedback with filters
func TestHandlerListFeedback(t *testing.T) {
	testFeedbacks := []Feedback{
		{
			ID:         uuid.New(),
			DecisionID: uuid.New(),
			Rating:     FeedbackPositive,
			CreatedAt:  time.Now(),
		},
		{
			ID:         uuid.New(),
			DecisionID: uuid.New(),
			Rating:     FeedbackNegative,
			CreatedAt:  time.Now(),
		},
	}

	mock := &MockFeedbackRepository{
		ListFeedbackFunc: func(ctx context.Context, filter FeedbackFilter) ([]Feedback, error) {
			// Filter by rating if provided
			if filter.Rating != nil {
				filtered := make([]Feedback, 0)
				for _, f := range testFeedbacks {
					if f.Rating == *filter.Rating {
						filtered = append(filtered, f)
					}
				}
				return filtered, nil
			}
			return testFeedbacks, nil
		},
	}

	router := setupFeedbackTestRouter(mock)

	t.Run("list all feedback", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/feedback", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		feedback := response["feedback"].([]interface{})
		assert.Len(t, feedback, 2)
	})

	t.Run("list feedback with rating filter", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/feedback?rating=positive", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		feedback := response["feedback"].([]interface{})
		assert.Len(t, feedback, 1)
	})

	t.Run("list feedback with invalid rating", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/feedback?rating=invalid", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("list feedback with pagination", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/feedback?limit=1&offset=0", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		filter := response["filter"].(map[string]interface{})
		assert.Equal(t, float64(1), filter["Limit"])
	})
}

// TestHandlerGetFeedback tests getting a single feedback by ID
func TestHandlerGetFeedback(t *testing.T) {
	testID := uuid.New()
	testFeedback := &Feedback{
		ID:         testID,
		DecisionID: uuid.New(),
		Rating:     FeedbackPositive,
		CreatedAt:  time.Now(),
	}

	mock := &MockFeedbackRepository{
		GetFeedbackFunc: func(ctx context.Context, id uuid.UUID) (*Feedback, error) {
			if id == testID {
				return testFeedback, nil
			}
			return nil, nil
		},
	}

	router := setupFeedbackTestRouter(mock)

	t.Run("get existing feedback", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/feedback/"+testID.String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var feedback Feedback
		err := json.Unmarshal(w.Body.Bytes(), &feedback)
		require.NoError(t, err)
		assert.Equal(t, testID, feedback.ID)
	})

	t.Run("get non-existent feedback", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/feedback/"+uuid.New().String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("get feedback with invalid ID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/feedback/invalid-uuid", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestHandlerUpdateFeedback tests updating feedback
func TestHandlerUpdateFeedback(t *testing.T) {
	testID := uuid.New()

	mock := &MockFeedbackRepository{
		UpdateFeedbackFunc: func(ctx context.Context, id uuid.UUID, req UpdateFeedbackRequest) (*Feedback, error) {
			if id == testID {
				rating := FeedbackNegative
				if req.Rating != nil {
					rating = *req.Rating
				}
				return &Feedback{
					ID:        testID,
					Rating:    rating,
					Comment:   req.Comment,
					Tags:      req.Tags,
					UpdatedAt: time.Now(),
				}, nil
			}
			return nil, nil
		},
	}

	router := setupFeedbackTestRouter(mock)

	t.Run("update feedback rating", func(t *testing.T) {
		body := map[string]interface{}{
			"rating": "negative",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/api/v1/feedback/"+testID.String(), bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var feedback Feedback
		err := json.Unmarshal(w.Body.Bytes(), &feedback)
		require.NoError(t, err)
		assert.Equal(t, FeedbackNegative, feedback.Rating)
	})

	t.Run("update feedback with invalid rating", func(t *testing.T) {
		body := map[string]interface{}{
			"rating": "invalid",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/api/v1/feedback/"+testID.String(), bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("update non-existent feedback", func(t *testing.T) {
		body := map[string]interface{}{
			"rating": "positive",
		}
		jsonBody, _ := json.Marshal(body)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "PUT", "/api/v1/feedback/"+uuid.New().String(), bytes.NewBuffer(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// TestHandlerDeleteFeedback tests deleting feedback
func TestHandlerDeleteFeedback(t *testing.T) {
	testID := uuid.New()
	testFeedback := &Feedback{
		ID:     testID,
		Rating: FeedbackPositive,
	}

	mock := &MockFeedbackRepository{
		GetFeedbackFunc: func(ctx context.Context, id uuid.UUID) (*Feedback, error) {
			if id == testID {
				return testFeedback, nil
			}
			return nil, nil
		},
		DeleteFeedbackFunc: func(ctx context.Context, id uuid.UUID) error {
			return nil
		},
	}

	router := setupFeedbackTestRouter(mock)

	t.Run("delete existing feedback", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "DELETE", "/api/v1/feedback/"+testID.String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("delete non-existent feedback", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "DELETE", "/api/v1/feedback/"+uuid.New().String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// TestHandlerGetFeedbackStats tests the stats endpoint
func TestHandlerGetFeedbackStats(t *testing.T) {
	mock := &MockFeedbackRepository{
		GetFeedbackStatsFunc: func(ctx context.Context, filter FeedbackFilter) (*FeedbackStats, error) {
			return &FeedbackStats{
				TotalFeedback: 100,
				PositiveCount: 70,
				NegativeCount: 30,
				PositiveRate:  70.0,
				ByAgent: map[string]AgentFeedbackStats{
					"technical-agent": {
						PositiveCount: 50,
						NegativeCount: 10,
						Total:         60,
						PositiveRate:  83.33,
					},
				},
				TopTags: []TagCount{
					{Tag: "accurate_prediction", Count: 30},
				},
			}, nil
		},
	}

	router := setupFeedbackTestRouter(mock)

	t.Run("get stats without filters", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/feedback/stats", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var stats FeedbackStats
		err := json.Unmarshal(w.Body.Bytes(), &stats)
		require.NoError(t, err)

		assert.Equal(t, 100, stats.TotalFeedback)
		assert.Equal(t, 70.0, stats.PositiveRate)
	})

	t.Run("get stats with agent filter", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/feedback/stats?agent_name=technical-agent", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestHandlerGetDecisionsNeedingReview tests the review endpoint
func TestHandlerGetDecisionsNeedingReview(t *testing.T) {
	mock := &MockFeedbackRepository{
		GetDecisionsNeedingReviewFunc: func(ctx context.Context, limit int) ([]DecisionNeedingReview, error) {
			return []DecisionNeedingReview{
				{
					DecisionID:    uuid.New(),
					DecisionType:  "signal",
					Symbol:        "BTC/USDT",
					FeedbackCount: 5,
					NegativeCount: 4,
					PositiveCount: 1,
				},
			}, nil
		},
	}

	router := setupFeedbackTestRouter(mock)

	t.Run("get decisions needing review", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/feedback/review", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		decisions := response["decisions"].([]interface{})
		assert.Len(t, decisions, 1)
	})

	t.Run("get decisions with limit", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/feedback/review?limit=5", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

// TestHandlerGetCommonTags tests the common tags endpoint
func TestHandlerGetCommonTags(t *testing.T) {
	mock := &MockFeedbackRepository{}
	router := setupFeedbackTestRouter(mock)

	w := httptest.NewRecorder()
	req, _ := http.NewRequestWithContext(context.Background(), "GET", "/api/v1/feedback/tags", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	tags := response["tags"].([]interface{})
	assert.NotEmpty(t, tags)
}

// TestHandlerRefreshStats tests the refresh stats endpoint
func TestHandlerRefreshStats(t *testing.T) {
	mock := &MockFeedbackRepository{
		RefreshStatsViewFunc: func(ctx context.Context) error {
			return nil
		},
	}

	router := setupFeedbackTestRouter(mock)

	t.Run("refresh stats success", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/feedback/refresh-stats", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response["message"], "refreshed successfully")
	})

	t.Run("refresh stats failure", func(t *testing.T) {
		failingMock := &MockFeedbackRepository{
			RefreshStatsViewFunc: func(ctx context.Context) error {
				return errors.New("database error")
			},
		}

		failingRouter := setupFeedbackTestRouter(failingMock)

		w := httptest.NewRecorder()
		req, _ := http.NewRequestWithContext(context.Background(), "POST", "/api/v1/feedback/refresh-stats", nil)
		failingRouter.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// TestIsDuplicateKeyError tests the duplicate key error detection
func TestIsDuplicateKeyError(t *testing.T) {
	assert.True(t, isDuplicateKeyError(errors.New("duplicate key value violates unique constraint")))
	assert.True(t, isDuplicateKeyError(errors.New("unique_user_decision_feedback constraint violated")))
	assert.False(t, isDuplicateKeyError(errors.New("some other error")))
	assert.False(t, isDuplicateKeyError(nil))
}

// TestContains tests the contains helper function
func TestContains(t *testing.T) {
	assert.True(t, contains("hello world", "world"))
	assert.True(t, contains("hello world", "hello"))
	assert.False(t, contains("hello world", "foo"))
	assert.True(t, contains("hello", "hello"))
	assert.False(t, contains("", "hello"))
}
