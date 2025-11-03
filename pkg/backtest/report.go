// HTML report generation for backtest results
package backtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"time"
)

// ============================================================================
// REPORT GENERATOR
// ============================================================================

// ReportGenerator generates HTML reports for backtest results
type ReportGenerator struct {
	engine  *Engine
	metrics *Metrics
	summary *OptimizationSummary // Optional, for optimization reports
}

// NewReportGenerator creates a new report generator
func NewReportGenerator(engine *Engine) (*ReportGenerator, error) {
	metrics, err := CalculateMetrics(engine)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate metrics: %w", err)
	}
	return &ReportGenerator{
		engine:  engine,
		metrics: metrics,
	}, nil
}

// NewOptimizationReportGenerator creates a report generator for optimization results
func NewOptimizationReportGenerator(engine *Engine, summary *OptimizationSummary) (*ReportGenerator, error) {
	metrics, err := CalculateMetrics(engine)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate metrics: %w", err)
	}
	return &ReportGenerator{
		engine:  engine,
		metrics: metrics,
		summary: summary,
	}, nil
}

// GenerateHTML generates a complete HTML report
func (r *ReportGenerator) GenerateHTML() (string, error) {
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"formatFloat":   formatFloat,
		"formatPercent": formatPercent,
		"formatTime":    formatTime,
		"mul":           func(a, b float64) float64 { return a * b },
		"add":           func(a, b int) int { return a + b },
		"last":          func(items []*ClosedPosition, n int) []*ClosedPosition {
			if len(items) <= n {
				return items
			}
			return items[len(items)-n:]
		},
		"formatParams":  func(params ParameterSet) string {
			result := ""
			for k, v := range params {
				if result != "" {
					result += ", "
				}
				result += fmt.Sprintf("%s=%v", k, v)
			}
			return result
		},
	}).Parse(reportTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	data := r.prepareTemplateData()

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// SaveToFile saves the HTML report to a file
func (r *ReportGenerator) SaveToFile(filepath string) error {
	html, err := r.GenerateHTML()
	if err != nil {
		return err
	}

	return os.WriteFile(filepath, []byte(html), 0644)
}

// prepareTemplateData prepares all data needed for the HTML template
func (r *ReportGenerator) prepareTemplateData() map[string]interface{} {
	// Reconstruct config from engine fields
	config := BacktestConfig{
		InitialCapital: r.engine.InitialCapital,
		CommissionRate: r.engine.CommissionRate,
		PositionSizing: r.engine.PositionSizing,
		PositionSize:   r.engine.PositionSize,
		MaxPositions:   r.engine.MaxPositions,
	}

	data := map[string]interface{}{
		"Title":       "Backtest Report",
		"GeneratedAt": time.Now(),
		"Config":      config,
		"Metrics":     r.metrics,
		"Summary":     r.summary,

		// Chart data
		"EquityCurveData":     r.prepareEquityCurveData(),
		"DrawdownData":        r.prepareDrawdownData(),
		"MonthlyReturnsData":  r.prepareMonthlyReturnsData(),
		"TradeDistribution":   r.prepareTradeDistributionData(),
		"WinLossData":         r.prepareWinLossData(),

		// Trade details
		"ClosedPositions":     r.engine.ClosedPositions,
		"Trades":             r.engine.Trades,

		// Optimization data (if available)
		"HasOptimization":    r.summary != nil,
		"OptimizationRuns":   r.getTopOptimizationRuns(10),
	}

	return data
}

// ============================================================================
// CHART DATA PREPARATION
// ============================================================================

// prepareEquityCurveData prepares equity curve data for Chart.js
func (r *ReportGenerator) prepareEquityCurveData() string {
	if len(r.engine.EquityCurve) == 0 {
		return "{labels: [], datasets: []}"
	}

	labels := make([]string, len(r.engine.EquityCurve))
	values := make([]float64, len(r.engine.EquityCurve))

	for i, point := range r.engine.EquityCurve {
		labels[i] = point.Timestamp.Format("2006-01-02 15:04")
		values[i] = point.Equity
	}

	labelsJSON, _ := json.Marshal(labels)
	valuesJSON, _ := json.Marshal(values)

	return fmt.Sprintf(`{
		labels: %s,
		datasets: [{
			label: 'Equity',
			data: %s,
			borderColor: 'rgb(75, 192, 192)',
			backgroundColor: 'rgba(75, 192, 192, 0.1)',
			tension: 0.1,
			fill: true
		}]
	}`, labelsJSON, valuesJSON)
}

// prepareDrawdownData prepares drawdown chart data
func (r *ReportGenerator) prepareDrawdownData() string {
	if len(r.engine.EquityCurve) == 0 {
		return "{labels: [], datasets: []}"
	}

	labels := make([]string, len(r.engine.EquityCurve))
	drawdowns := make([]float64, len(r.engine.EquityCurve))

	peakEquity := r.engine.EquityCurve[0].Equity
	for i, point := range r.engine.EquityCurve {
		labels[i] = point.Timestamp.Format("2006-01-02 15:04")

		if point.Equity > peakEquity {
			peakEquity = point.Equity
		}

		drawdown := ((point.Equity - peakEquity) / peakEquity) * 100
		drawdowns[i] = drawdown
	}

	labelsJSON, _ := json.Marshal(labels)
	drawdownsJSON, _ := json.Marshal(drawdowns)

	return fmt.Sprintf(`{
		labels: %s,
		datasets: [{
			label: 'Drawdown (%%)',
			data: %s,
			borderColor: 'rgb(255, 99, 132)',
			backgroundColor: 'rgba(255, 99, 132, 0.1)',
			tension: 0.1,
			fill: true
		}]
	}`, labelsJSON, drawdownsJSON)
}

// prepareMonthlyReturnsData prepares monthly returns bar chart data
func (r *ReportGenerator) prepareMonthlyReturnsData() string {
	if len(r.engine.ClosedPositions) == 0 {
		return "{labels: [], datasets: []}"
	}

	// Group P&L by month
	monthlyPL := make(map[string]float64)
	for _, pos := range r.engine.ClosedPositions {
		monthKey := pos.ExitTime.Format("2006-01")
		monthlyPL[monthKey] += pos.RealizedPL
	}

	// Sort months
	var months []string
	for month := range monthlyPL {
		months = append(months, month)
	}

	// Simple bubble sort (small dataset)
	for i := 0; i < len(months); i++ {
		for j := i + 1; j < len(months); j++ {
			if months[i] > months[j] {
				months[i], months[j] = months[j], months[i]
			}
		}
	}

	values := make([]float64, len(months))
	for i, month := range months {
		values[i] = monthlyPL[month]
	}

	labelsJSON, _ := json.Marshal(months)
	valuesJSON, _ := json.Marshal(values)

	return fmt.Sprintf(`{
		labels: %s,
		datasets: [{
			label: 'Monthly P&L ($)',
			data: %s,
			backgroundColor: %s.map(v => v >= 0 ? 'rgba(75, 192, 192, 0.8)' : 'rgba(255, 99, 132, 0.8)'),
			borderColor: %s.map(v => v >= 0 ? 'rgb(75, 192, 192)' : 'rgb(255, 99, 132)'),
			borderWidth: 1
		}]
	}`, labelsJSON, valuesJSON, valuesJSON, valuesJSON)
}

// prepareTradeDistributionData prepares trade P&L distribution histogram
func (r *ReportGenerator) prepareTradeDistributionData() string {
	if len(r.engine.ClosedPositions) == 0 {
		return "{labels: [], datasets: []}"
	}

	// Create bins for P&L distribution
	bins := []float64{-1000, -500, -250, -100, -50, 0, 50, 100, 250, 500, 1000}
	binLabels := []string{"< -$1000", "-$1000 to -$500", "-$500 to -$250", "-$250 to -$100",
		"-$100 to -$50", "-$50 to $0", "$0 to $50", "$50 to $100", "$100 to $250", "$250 to $500", "> $500"}
	counts := make([]int, len(bins)+1)

	for _, pos := range r.engine.ClosedPositions {
		binned := false
		for i, bin := range bins {
			if pos.RealizedPL < bin {
				counts[i]++
				binned = true
				break
			}
		}
		if !binned {
			counts[len(bins)]++
		}
	}

	labelsJSON, _ := json.Marshal(binLabels)
	countsJSON, _ := json.Marshal(counts)

	return fmt.Sprintf(`{
		labels: %s,
		datasets: [{
			label: 'Number of Trades',
			data: %s,
			backgroundColor: 'rgba(54, 162, 235, 0.8)',
			borderColor: 'rgb(54, 162, 235)',
			borderWidth: 1
		}]
	}`, labelsJSON, countsJSON)
}

// prepareWinLossData prepares pie chart for win/loss ratio
func (r *ReportGenerator) prepareWinLossData() string {
	data := []int{r.metrics.WinningTrades, r.metrics.LosingTrades}
	dataJSON, _ := json.Marshal(data)

	return fmt.Sprintf(`{
		labels: ['Winning Trades', 'Losing Trades'],
		datasets: [{
			data: %s,
			backgroundColor: [
				'rgba(75, 192, 192, 0.8)',
				'rgba(255, 99, 132, 0.8)'
			],
			borderColor: [
				'rgb(75, 192, 192)',
				'rgb(255, 99, 132)'
			],
			borderWidth: 1
		}]
	}`, dataJSON)
}

// getTopOptimizationRuns returns top N optimization runs by score
func (r *ReportGenerator) getTopOptimizationRuns(n int) []*OptimizationResult {
	if r.summary == nil || len(r.summary.TopResults) == 0 {
		return nil
	}

	if n > len(r.summary.TopResults) {
		n = len(r.summary.TopResults)
	}

	return r.summary.TopResults[:n]
}

// ============================================================================
// TEMPLATE HELPER FUNCTIONS
// ============================================================================

func formatFloat(f float64) string {
	return fmt.Sprintf("%.2f", f)
}

func formatPercent(f float64) string {
	return fmt.Sprintf("%.2f%%", f)
}

func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// ============================================================================
// HTML TEMPLATE
// ============================================================================

const reportTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{ .Title }}</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js@4.4.0/dist/chart.umd.min.js"></script>
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            background: #f5f5f5;
            color: #333;
            line-height: 1.6;
        }

        .container {
            max-width: 1400px;
            margin: 0 auto;
            padding: 20px;
        }

        header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 30px;
            border-radius: 10px;
            margin-bottom: 30px;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
        }

        header h1 {
            font-size: 2.5em;
            margin-bottom: 10px;
        }

        header p {
            opacity: 0.9;
            font-size: 1.1em;
        }

        .section {
            background: white;
            padding: 25px;
            margin-bottom: 25px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
        }

        .section h2 {
            color: #667eea;
            margin-bottom: 20px;
            padding-bottom: 10px;
            border-bottom: 2px solid #f0f0f0;
        }

        .metrics-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 20px;
            margin-top: 20px;
        }

        .metric-card {
            background: linear-gradient(135deg, #f5f7fa 0%, #c3cfe2 100%);
            padding: 20px;
            border-radius: 8px;
            border-left: 4px solid #667eea;
        }

        .metric-label {
            font-size: 0.9em;
            color: #666;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            margin-bottom: 8px;
        }

        .metric-value {
            font-size: 1.8em;
            font-weight: bold;
            color: #333;
        }

        .metric-value.positive {
            color: #10b981;
        }

        .metric-value.negative {
            color: #ef4444;
        }

        .chart-container {
            position: relative;
            height: 400px;
            margin: 20px 0;
        }

        .chart-row {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 25px;
            margin: 20px 0;
        }

        table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 20px;
        }

        table th {
            background: #667eea;
            color: white;
            padding: 12px;
            text-align: left;
            font-weight: 600;
        }

        table td {
            padding: 12px;
            border-bottom: 1px solid #f0f0f0;
        }

        table tr:hover {
            background: #f9f9f9;
        }

        .positive {
            color: #10b981;
            font-weight: 600;
        }

        .negative {
            color: #ef4444;
            font-weight: 600;
        }

        .config-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 15px;
            margin-top: 15px;
        }

        .config-item {
            display: flex;
            flex-direction: column;
        }

        .config-label {
            font-size: 0.85em;
            color: #666;
            margin-bottom: 5px;
        }

        .config-value {
            font-weight: 600;
            color: #333;
        }

        footer {
            text-align: center;
            padding: 20px;
            color: #666;
            font-size: 0.9em;
        }

        @media print {
            .chart-container {
                height: 300px;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>{{ .Title }}</h1>
            <p>Generated: {{ formatTime .GeneratedAt }}</p>
        </header>

        <!-- Performance Metrics -->
        <div class="section">
            <h2>📊 Performance Summary</h2>
            <div class="metrics-grid">
                <div class="metric-card">
                    <div class="metric-label">Total Return</div>
                    <div class="metric-value {{ if ge .Metrics.TotalReturn 0.0 }}positive{{ else }}negative{{ end }}">
                        {{ formatPercent .Metrics.TotalReturn }}
                    </div>
                </div>
                <div class="metric-card">
                    <div class="metric-label">Sharpe Ratio</div>
                    <div class="metric-value">{{ formatFloat .Metrics.SharpeRatio }}</div>
                </div>
                <div class="metric-card">
                    <div class="metric-label">Max Drawdown</div>
                    <div class="metric-value negative">{{ formatPercent .Metrics.MaxDrawdownPct }}</div>
                </div>
                <div class="metric-card">
                    <div class="metric-label">Win Rate</div>
                    <div class="metric-value">{{ formatPercent .Metrics.WinRate }}</div>
                </div>
                <div class="metric-card">
                    <div class="metric-label">Profit Factor</div>
                    <div class="metric-value">{{ formatFloat .Metrics.ProfitFactor }}</div>
                </div>
                <div class="metric-card">
                    <div class="metric-label">Total Trades</div>
                    <div class="metric-value">{{ .Metrics.TotalTrades }}</div>
                </div>
                <div class="metric-card">
                    <div class="metric-label">CAGR</div>
                    <div class="metric-value {{ if ge .Metrics.CAGR 0.0 }}positive{{ else }}negative{{ end }}">
                        {{ formatPercent .Metrics.CAGR }}
                    </div>
                </div>
                <div class="metric-card">
                    <div class="metric-label">Sortino Ratio</div>
                    <div class="metric-value">{{ formatFloat .Metrics.SortinoRatio }}</div>
                </div>
            </div>
        </div>

        <!-- Equity Curve -->
        <div class="section">
            <h2>📈 Equity Curve</h2>
            <div class="chart-container">
                <canvas id="equityChart"></canvas>
            </div>
        </div>

        <!-- Drawdown Chart -->
        <div class="section">
            <h2>📉 Drawdown</h2>
            <div class="chart-container">
                <canvas id="drawdownChart"></canvas>
            </div>
        </div>

        <!-- Monthly Returns & Distribution -->
        <div class="section">
            <h2>📊 Returns Analysis</h2>
            <div class="chart-row">
                <div class="chart-container">
                    <canvas id="monthlyReturnsChart"></canvas>
                </div>
                <div class="chart-container">
                    <canvas id="tradeDistributionChart"></canvas>
                </div>
            </div>
        </div>

        <!-- Win/Loss Breakdown -->
        <div class="section">
            <h2>🎯 Trade Breakdown</h2>
            <div class="chart-row">
                <div class="chart-container">
                    <canvas id="winLossChart"></canvas>
                </div>
                <div>
                    <div class="metrics-grid">
                        <div class="metric-card">
                            <div class="metric-label">Winning Trades</div>
                            <div class="metric-value positive">{{ .Metrics.WinningTrades }}</div>
                        </div>
                        <div class="metric-card">
                            <div class="metric-label">Losing Trades</div>
                            <div class="metric-value negative">{{ .Metrics.LosingTrades }}</div>
                        </div>
                        <div class="metric-card">
                            <div class="metric-label">Average Win</div>
                            <div class="metric-value positive">${{ formatFloat .Metrics.AverageWin }}</div>
                        </div>
                        <div class="metric-card">
                            <div class="metric-label">Average Loss</div>
                            <div class="metric-value negative">${{ formatFloat .Metrics.AverageLoss }}</div>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Configuration -->
        <div class="section">
            <h2>⚙️ Backtest Configuration</h2>
            <div class="config-grid">
                <div class="config-item">
                    <div class="config-label">Initial Capital</div>
                    <div class="config-value">${{ formatFloat .Config.InitialCapital }}</div>
                </div>
                <div class="config-item">
                    <div class="config-label">Commission Rate</div>
                    <div class="config-value">{{ formatPercent (mul .Config.CommissionRate 100) }}</div>
                </div>
                <div class="config-item">
                    <div class="config-label">Position Sizing</div>
                    <div class="config-value">{{ .Config.PositionSizing }}</div>
                </div>
                <div class="config-item">
                    <div class="config-label">Max Positions</div>
                    <div class="config-value">{{ .Config.MaxPositions }}</div>
                </div>
            </div>
        </div>

        {{ if .HasOptimization }}
        <!-- Optimization Results -->
        <div class="section">
            <h2>🔬 Optimization Results</h2>
            <p><strong>Method:</strong> {{ .Summary.Method }}</p>
            <p><strong>Total Runs:</strong> {{ .Summary.TotalRuns }}</p>
            <p><strong>Duration:</strong> {{ .Summary.Duration }}</p>
            <p><strong>Best Score:</strong> {{ formatFloat .Summary.BestResult.Score }}</p>

            <h3 style="margin-top: 20px;">Top 10 Parameter Sets</h3>
            <table>
                <thead>
                    <tr>
                        <th>Rank</th>
                        <th>Score</th>
                        <th>Parameters</th>
                        <th>Sharpe</th>
                        <th>Return</th>
                        <th>Drawdown</th>
                    </tr>
                </thead>
                <tbody>
                    {{ range $i, $result := .OptimizationRuns }}
                    <tr>
                        <td>{{ add $i 1 }}</td>
                        <td>{{ formatFloat $result.Score }}</td>
                        <td>{{ formatParams $result.Parameters }}</td>
                        <td>{{ formatFloat $result.Metrics.SharpeRatio }}</td>
                        <td class="{{ if ge $result.Metrics.TotalReturn 0.0 }}positive{{ else }}negative{{ end }}">
                            {{ formatPercent $result.Metrics.TotalReturn }}
                        </td>
                        <td class="negative">{{ formatPercent $result.Metrics.MaxDrawdownPct }}</td>
                    </tr>
                    {{ end }}
                </tbody>
            </table>
        </div>
        {{ end }}

        <!-- Recent Trades -->
        <div class="section">
            <h2>📝 Recent Trades (Last 20)</h2>
            <table>
                <thead>
                    <tr>
                        <th>Symbol</th>
                        <th>Entry Time</th>
                        <th>Exit Time</th>
                        <th>Side</th>
                        <th>Quantity</th>
                        <th>Entry Price</th>
                        <th>Exit Price</th>
                        <th>P&L</th>
                        <th>P&L %</th>
                    </tr>
                </thead>
                <tbody>
                    {{ range last .ClosedPositions 20 }}
                    <tr>
                        <td>{{ .Symbol }}</td>
                        <td>{{ formatTime .EntryTime }}</td>
                        <td>{{ formatTime .ExitTime }}</td>
                        <td>{{ .Side }}</td>
                        <td>{{ formatFloat .Quantity }}</td>
                        <td>${{ formatFloat .EntryPrice }}</td>
                        <td>${{ formatFloat .ExitPrice }}</td>
                        <td class="{{ if ge .RealizedPL 0.0 }}positive{{ else }}negative{{ end }}">
                            ${{ formatFloat .RealizedPL }}
                        </td>
                        <td class="{{ if ge .ReturnPct 0.0 }}positive{{ else }}negative{{ end }}">
                            {{ formatPercent .ReturnPct }}
                        </td>
                    </tr>
                    {{ end }}
                </tbody>
            </table>
        </div>

        <footer>
            <p>🤖 Generated with CryptoFunk Backtest Engine</p>
            <p>Powered by Claude Code</p>
        </footer>
    </div>

    <script>
        // Chart.js configuration
        Chart.defaults.font.family = '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif';
        Chart.defaults.color = '#666';

        // Equity Curve Chart
        new Chart(document.getElementById('equityChart'), {
            type: 'line',
            data: {{ .EquityCurveData }},
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: true },
                    title: { display: false }
                },
                scales: {
                    y: {
                        beginAtZero: false,
                        ticks: {
                            callback: function(value) {
                                return '$' + value.toLocaleString();
                            }
                        }
                    }
                }
            }
        });

        // Drawdown Chart
        new Chart(document.getElementById('drawdownChart'), {
            type: 'line',
            data: {{ .DrawdownData }},
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: true }
                },
                scales: {
                    y: {
                        ticks: {
                            callback: function(value) {
                                return value.toFixed(2) + '%';
                            }
                        }
                    }
                }
            }
        });

        // Monthly Returns Chart
        new Chart(document.getElementById('monthlyReturnsChart'), {
            type: 'bar',
            data: {{ .MonthlyReturnsData }},
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: true },
                    title: { display: true, text: 'Monthly P&L' }
                },
                scales: {
                    y: {
                        ticks: {
                            callback: function(value) {
                                return '$' + value.toLocaleString();
                            }
                        }
                    }
                }
            }
        });

        // Trade Distribution Chart
        new Chart(document.getElementById('tradeDistributionChart'), {
            type: 'bar',
            data: {{ .TradeDistribution }},
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { display: true },
                    title: { display: true, text: 'P&L Distribution' }
                }
            }
        });

        // Win/Loss Pie Chart
        new Chart(document.getElementById('winLossChart'), {
            type: 'pie',
            data: {{ .WinLossData }},
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: {
                        display: true,
                        position: 'bottom'
                    }
                }
            }
        });
    </script>
</body>
</html>
`
