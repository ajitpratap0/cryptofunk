package backtest

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	btengine "github.com/ajitpratap0/cryptofunk/pkg/backtest"
)

// JobStatus represents the status of a backtest job
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// BacktestJob represents a backtest job configuration and results
type BacktestJob struct {
	ID             uuid.UUID              `json:"id"`
	Name           string                 `json:"name"`
	Status         JobStatus              `json:"status"`
	StartDate      time.Time              `json:"start_date"`
	EndDate        time.Time              `json:"end_date"`
	Symbols        []string               `json:"symbols"`
	InitialCapital float64                `json:"initial_capital"`
	StrategyConfig map[string]interface{} `json:"strategy_config"`
	ParameterGrid  map[string]interface{} `json:"parameter_grid,omitempty"`
	Results        *BacktestResults       `json:"results,omitempty"`
	ErrorMessage   string                 `json:"error_message,omitempty"`
	ErrorDetails   string                 `json:"error_details,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	StartedAt      *time.Time             `json:"started_at,omitempty"`
	CompletedAt    *time.Time             `json:"completed_at,omitempty"`
	UpdatedAt      time.Time              `json:"updated_at"`
	CreatedBy      string                 `json:"created_by,omitempty"`
}

// BacktestResults contains the complete results of a backtest
type BacktestResults struct {
	// Performance metrics
	TotalReturnPct float64 `json:"total_return_pct"`
	SharpeRatio    float64 `json:"sharpe_ratio"`
	MaxDrawdownPct float64 `json:"max_drawdown_pct"`
	WinRate        float64 `json:"win_rate"`
	TotalTrades    int     `json:"total_trades"`
	ProfitFactor   float64 `json:"profit_factor"`
	SortinoRatio   float64 `json:"sortino_ratio"`
	CalmarRatio    float64 `json:"calmar_ratio"`
	Expectancy     float64 `json:"expectancy"`

	// Additional metrics
	WinningTrades int     `json:"winning_trades"`
	LosingTrades  int     `json:"losing_trades"`
	AverageWin    float64 `json:"average_win"`
	AverageLoss   float64 `json:"average_loss"`
	LargestWin    float64 `json:"largest_win"`
	LargestLoss   float64 `json:"largest_loss"`

	// Equity curve (array of {date, value})
	EquityCurve []EquityPoint `json:"equity_curve"`

	// Trade log (entry/exit prices, P&L per trade)
	Trades []TradeResult `json:"trades"`
}

// EquityPoint represents a point in the equity curve
type EquityPoint struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
}

// TradeResult represents a single trade result
type TradeResult struct {
	Symbol      string    `json:"symbol"`
	Side        string    `json:"side"`
	EntryTime   time.Time `json:"entry_time"`
	ExitTime    time.Time `json:"exit_time"`
	EntryPrice  float64   `json:"entry_price"`
	ExitPrice   float64   `json:"exit_price"`
	Quantity    float64   `json:"quantity"`
	PnL         float64   `json:"pnl"`
	PnLPct      float64   `json:"pnl_pct"`
	Commission  float64   `json:"commission"`
	HoldingTime string    `json:"holding_time"`
}

// JobManager manages backtest jobs
type JobManager struct {
	db *pgxpool.Pool
	mu sync.RWMutex
}

// NewJobManager creates a new backtest job manager
func NewJobManager(db *pgxpool.Pool) *JobManager {
	return &JobManager{
		db: db,
	}
}

// CreateJob creates a new backtest job in the database
func (m *JobManager) CreateJob(ctx context.Context, job *BacktestJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate ID if not provided
	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}

	// Set timestamps
	now := time.Now()
	job.CreatedAt = now
	job.UpdatedAt = now
	job.Status = JobStatusPending

	// Validate input
	if err := m.validateJob(job); err != nil {
		return fmt.Errorf("invalid job configuration: %w", err)
	}

	// Convert strategy config to JSON
	strategyConfigJSON, err := json.Marshal(job.StrategyConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal strategy config: %w", err)
	}

	// Convert parameter grid to JSON (nullable)
	var parameterGridJSON []byte
	if job.ParameterGrid != nil {
		parameterGridJSON, err = json.Marshal(job.ParameterGrid)
		if err != nil {
			return fmt.Errorf("failed to marshal parameter grid: %w", err)
		}
	}

	// Insert into database
	query := `
		INSERT INTO backtest_jobs (
			id, name, status, start_date, end_date, symbols,
			initial_capital, strategy_config, parameter_grid,
			created_at, updated_at, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err = m.db.Exec(ctx, query,
		job.ID, job.Name, job.Status, job.StartDate, job.EndDate, job.Symbols,
		job.InitialCapital, strategyConfigJSON, parameterGridJSON,
		job.CreatedAt, job.UpdatedAt, job.CreatedBy,
	)

	if err != nil {
		return fmt.Errorf("failed to insert backtest job: %w", err)
	}

	log.Info().
		Str("job_id", job.ID.String()).
		Str("name", job.Name).
		Msg("Created backtest job")

	return nil
}

// validateJob validates a backtest job configuration
func (m *JobManager) validateJob(job *BacktestJob) error {
	if job.Name == "" {
		return fmt.Errorf("job name is required")
	}

	if job.EndDate.Before(job.StartDate) || job.EndDate.Equal(job.StartDate) {
		return fmt.Errorf("end_date must be after start_date")
	}

	if len(job.Symbols) == 0 {
		return fmt.Errorf("at least one symbol is required")
	}

	if job.InitialCapital <= 0 {
		return fmt.Errorf("initial_capital must be positive")
	}

	if len(job.StrategyConfig) == 0 {
		return fmt.Errorf("strategy_config is required")
	}

	return nil
}

// GetJob retrieves a backtest job by ID
func (m *JobManager) GetJob(ctx context.Context, jobID uuid.UUID) (*BacktestJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	query := `
		SELECT id, name, status, start_date, end_date, symbols,
		       initial_capital, strategy_config, parameter_grid, results,
		       error_message, error_details,
		       created_at, started_at, completed_at, updated_at, created_by
		FROM backtest_jobs
		WHERE id = $1
	`

	var job BacktestJob
	var strategyConfigJSON []byte
	var parameterGridJSON []byte
	var resultsJSON []byte

	err := m.db.QueryRow(ctx, query, jobID).Scan(
		&job.ID, &job.Name, &job.Status, &job.StartDate, &job.EndDate, &job.Symbols,
		&job.InitialCapital, &strategyConfigJSON, &parameterGridJSON, &resultsJSON,
		&job.ErrorMessage, &job.ErrorDetails,
		&job.CreatedAt, &job.StartedAt, &job.CompletedAt, &job.UpdatedAt, &job.CreatedBy,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve backtest job: %w", err)
	}

	// Unmarshal strategy config
	if err := json.Unmarshal(strategyConfigJSON, &job.StrategyConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal strategy config: %w", err)
	}

	// Unmarshal parameter grid (if present)
	if len(parameterGridJSON) > 0 {
		if err := json.Unmarshal(parameterGridJSON, &job.ParameterGrid); err != nil {
			return nil, fmt.Errorf("failed to unmarshal parameter grid: %w", err)
		}
	}

	// Unmarshal results (if present)
	if len(resultsJSON) > 0 {
		var results BacktestResults
		if err := json.Unmarshal(resultsJSON, &results); err != nil {
			return nil, fmt.Errorf("failed to unmarshal results: %w", err)
		}
		job.Results = &results
	}

	return &job, nil
}

// ListJobs retrieves a paginated list of backtest jobs
func (m *JobManager) ListJobs(ctx context.Context, createdBy string, limit, offset int) ([]*BacktestJob, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Build query with optional user filter
	whereClause := ""
	args := []interface{}{}
	argPos := 1

	if createdBy != "" {
		whereClause = "WHERE created_by = $" + fmt.Sprintf("%d", argPos)
		args = append(args, createdBy)
		argPos++
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM backtest_jobs %s", whereClause)
	var total int
	if err := m.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count backtest jobs: %w", err)
	}

	// Query jobs
	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT id, name, status, start_date, end_date, symbols,
		       initial_capital,
		       total_return_pct, sharpe_ratio, max_drawdown_pct, win_rate, total_trades,
		       error_message,
		       created_at, started_at, completed_at, updated_at, created_by
		FROM backtest_jobs
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argPos, argPos+1)

	rows, err := m.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query backtest jobs: %w", err)
	}
	defer rows.Close()

	jobs := make([]*BacktestJob, 0)
	for rows.Next() {
		var job BacktestJob
		var totalReturnPct, sharpeRatio, maxDrawdownPct, winRate *float64
		var totalTrades *int

		err := rows.Scan(
			&job.ID, &job.Name, &job.Status, &job.StartDate, &job.EndDate, &job.Symbols,
			&job.InitialCapital,
			&totalReturnPct, &sharpeRatio, &maxDrawdownPct, &winRate, &totalTrades,
			&job.ErrorMessage,
			&job.CreatedAt, &job.StartedAt, &job.CompletedAt, &job.UpdatedAt, &job.CreatedBy,
		)

		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan backtest job: %w", err)
		}

		// Create minimal results object with denormalized metrics
		if job.Status == JobStatusCompleted && totalReturnPct != nil {
			job.Results = &BacktestResults{
				TotalReturnPct: *totalReturnPct,
				SharpeRatio:    getValue(sharpeRatio),
				MaxDrawdownPct: getValue(maxDrawdownPct),
				WinRate:        getValue(winRate),
				TotalTrades:    getIntValue(totalTrades),
			}
		}

		jobs = append(jobs, &job)
	}

	return jobs, total, nil
}

// getValue safely dereferences a float64 pointer
func getValue(ptr *float64) float64 {
	if ptr != nil {
		return *ptr
	}
	return 0
}

// getIntValue safely dereferences an int pointer
func getIntValue(ptr *int) int {
	if ptr != nil {
		return *ptr
	}
	return 0
}

// UpdateJobStatus updates the status of a backtest job
func (m *JobManager) UpdateJobStatus(ctx context.Context, jobID uuid.UUID, status JobStatus, errorMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	var startedAt, completedAt *time.Time

	switch status {
	case JobStatusRunning:
		startedAt = &now
	case JobStatusCompleted, JobStatusFailed, JobStatusCancelled:
		completedAt = &now
	}

	query := `
		UPDATE backtest_jobs
		SET status = $1,
		    started_at = COALESCE($2, started_at),
		    completed_at = COALESCE($3, completed_at),
		    error_message = $4,
		    updated_at = $5
		WHERE id = $6
	`

	_, err := m.db.Exec(ctx, query, status, startedAt, completedAt, errorMsg, now, jobID)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	return nil
}

// SaveResults saves the backtest results to the database
func (m *JobManager) SaveResults(ctx context.Context, jobID uuid.UUID, results *BacktestResults) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Marshal results to JSON
	resultsJSON, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	now := time.Now()

	query := `
		UPDATE backtest_jobs
		SET results = $1,
		    total_return_pct = $2,
		    sharpe_ratio = $3,
		    max_drawdown_pct = $4,
		    win_rate = $5,
		    total_trades = $6,
		    status = $7,
		    completed_at = $8,
		    updated_at = $9
		WHERE id = $10
	`

	_, err = m.db.Exec(ctx, query,
		resultsJSON,
		results.TotalReturnPct,
		results.SharpeRatio,
		results.MaxDrawdownPct,
		results.WinRate,
		results.TotalTrades,
		JobStatusCompleted,
		now,
		now,
		jobID,
	)

	if err != nil {
		return fmt.Errorf("failed to save results: %w", err)
	}

	log.Info().
		Str("job_id", jobID.String()).
		Float64("total_return_pct", results.TotalReturnPct).
		Float64("sharpe_ratio", results.SharpeRatio).
		Msg("Saved backtest results")

	return nil
}

// DeleteJob deletes a backtest job
func (m *JobManager) DeleteJob(ctx context.Context, jobID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	query := `DELETE FROM backtest_jobs WHERE id = $1`

	result, err := m.db.Exec(ctx, query, jobID)
	if err != nil {
		return fmt.Errorf("failed to delete backtest job: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("backtest job not found")
	}

	log.Info().
		Str("job_id", jobID.String()).
		Msg("Deleted backtest job")

	return nil
}

// ConvertEngineResultsToBacktestResults converts backtest engine results to API format
func ConvertEngineResultsToBacktestResults(engine *btengine.Engine, metrics *btengine.Metrics) *BacktestResults {
	// Convert equity curve
	equityCurve := make([]EquityPoint, len(engine.EquityCurve))
	for i, point := range engine.EquityCurve {
		equityCurve[i] = EquityPoint{
			Date:  point.Timestamp.Format("2006-01-02"),
			Value: point.Equity,
		}
	}

	// Convert trades
	trades := make([]TradeResult, len(engine.ClosedPositions))
	for i, pos := range engine.ClosedPositions {
		trades[i] = TradeResult{
			Symbol:      pos.Symbol,
			Side:        pos.Side,
			EntryTime:   pos.EntryTime,
			ExitTime:    pos.ExitTime,
			EntryPrice:  pos.EntryPrice,
			ExitPrice:   pos.ExitPrice,
			Quantity:    pos.Quantity,
			PnL:         pos.RealizedPL,
			PnLPct:      pos.ReturnPct,
			Commission:  pos.Commission,
			HoldingTime: pos.HoldingTime.String(),
		}
	}

	return &BacktestResults{
		TotalReturnPct: metrics.TotalReturnPct,
		SharpeRatio:    metrics.SharpeRatio,
		MaxDrawdownPct: metrics.MaxDrawdownPct,
		WinRate:        metrics.WinRate,
		TotalTrades:    metrics.TotalTrades,
		ProfitFactor:   metrics.ProfitFactor,
		SortinoRatio:   metrics.SortinoRatio,
		CalmarRatio:    metrics.CalmarRatio,
		Expectancy:     metrics.Expectancy,
		WinningTrades:  metrics.WinningTrades,
		LosingTrades:   metrics.LosingTrades,
		AverageWin:     metrics.AverageWin,
		AverageLoss:    metrics.AverageLoss,
		LargestWin:     metrics.LargestWin,
		LargestLoss:    metrics.LargestLoss,
		EquityCurve:    equityCurve,
		Trades:         trades,
	}
}
