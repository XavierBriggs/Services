package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/XavierBriggs/fortuna/services/opportunity-analytics/internal/writer"
	"github.com/XavierBriggs/fortuna/services/opportunity-analytics/pkg/models"
)

// Handler handles HTTP requests for analytics
type Handler struct {
	writer *writer.HolocronWriter

	// Prometheus metrics
	opportunitiesProcessed prometheus.Counter
	requestDuration        *prometheus.HistogramVec
}

// NewHandler creates a new handler
func NewHandler(w *writer.HolocronWriter) *Handler {
	h := &Handler{
		writer: w,
		opportunitiesProcessed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "opportunities_processed_total",
			Help: "Total number of opportunities processed",
		}),
		requestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"endpoint", "method"}),
	}

	// Register Prometheus metrics
	prometheus.MustRegister(h.opportunitiesProcessed)
	prometheus.MustRegister(h.requestDuration)

	return h
}

// SetupRouter sets up the HTTP router
func (h *Handler) SetupRouter() *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:3001", "*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	// Health check
	r.Get("/health", h.HealthCheck)

	// Stats endpoints (Phase 1 - existing)
	r.Get("/stats/summary", h.GetStatsSummary)
	r.Get("/stats/timeseries", h.GetTimeSeries)
	r.Get("/stats/profitability", h.GetProfitability)
	r.Get("/stats/books", h.GetBookStats)

	// Phase 2: Execution/Hold Time endpoints
	r.Get("/stats/execution", h.GetExecutionStats)
	r.Get("/stats/hold-time", h.GetHoldTimeStats)

	// Phase 3: Edge Distribution endpoints
	r.Get("/stats/edge-distribution", h.GetEdgeDistribution)

	// Book pair endpoints (for scalps/middles)
	r.Get("/stats/book-pairs", h.GetBestBookPairs)
	r.Get("/stats/scalp-pairs", h.GetScalpPairs)
	r.Get("/stats/middle-pairs", h.GetMiddlePairs)

	// Opportunity CLV endpoints (Edge Detector Validation)
	r.Get("/stats/opportunity-clv", h.GetOpportunityCLV)
	r.Get("/stats/edge-accuracy", h.GetEdgeAccuracy)

	// Pair Performance endpoints (Handle, Profit, ROI per book pair)
	r.Get("/stats/pair-performance", h.GetPairPerformance)
	r.Post("/stats/pair-performance/refresh", h.RefreshPairPerformance)

	// API Gateway compatible routes (prefixed with /api/v1)
	r.Route("/api/v1/analytics", func(r chi.Router) {
		r.Get("/stats/summary", h.GetStatsSummary)
		r.Get("/stats/timeseries", h.GetTimeSeries)
		r.Get("/stats/profitability", h.GetProfitability)
		r.Get("/stats/books", h.GetBookStats)
		r.Get("/stats/execution", h.GetExecutionStats)
		r.Get("/stats/hold-time", h.GetHoldTimeStats)
		r.Get("/edge-distribution", h.GetEdgeDistribution)
		r.Get("/stats/book-pairs", h.GetBestBookPairs)
		r.Get("/stats/scalp-pairs", h.GetScalpPairs)
		r.Get("/stats/middle-pairs", h.GetMiddlePairs)
		r.Get("/stats/opportunity-clv", h.GetOpportunityCLV)
		r.Get("/stats/edge-accuracy", h.GetEdgeAccuracy)
		r.Get("/stats/pair-performance", h.GetPairPerformance)
		r.Post("/stats/pair-performance/refresh", h.RefreshPairPerformance)
	})

	// Prometheus metrics
	r.Handle("/metrics", promhttp.Handler())

	return r
}

// HealthCheck handles health check requests
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "healthy",
		"service": "opportunity-analytics",
		"port":    "8091",
		"version": "2.0.0",
		"features": map[string]bool{
			"hold_time_tracking":   true,
			"edge_distribution":    true,
			"execution_rate":       true,
			"enhanced_aggregation": true,
		},
		"timestamp": time.Now(),
	})
}

// GetStatsSummary handles GET /stats/summary
func (h *Handler) GetStatsSummary(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		h.requestDuration.WithLabelValues("/stats/summary", "GET").Observe(time.Since(start).Seconds())
	}()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Parse query parameters
	startTime, endTime := parseTimeRange(r)
	bookKey := r.URL.Query().Get("book")
	oppType := r.URL.Query().Get("type")

	summary, err := h.writer.GetStatsSummary(ctx, startTime, endTime, bookKey, oppType)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get summary", err)
		return
	}

	respondJSON(w, http.StatusOK, summary)
}

// GetTimeSeries handles GET /stats/timeseries
func (h *Handler) GetTimeSeries(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		h.requestDuration.WithLabelValues("/stats/timeseries", "GET").Observe(time.Since(start).Seconds())
	}()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Parse query parameters
	startTime, endTime := parseTimeRange(r)
	bookKey := r.URL.Query().Get("book")
	oppType := r.URL.Query().Get("type")

	points, err := h.writer.GetTimeSeries(ctx, startTime, endTime, bookKey, oppType)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get time series", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"points": points,
		"count":  len(points),
	})
}

// GetProfitability handles GET /stats/profitability
func (h *Handler) GetProfitability(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		h.requestDuration.WithLabelValues("/stats/profitability", "GET").Observe(time.Since(start).Seconds())
	}()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Parse query parameters
	startTime, endTime := parseTimeRange(r)
	bookKey := r.URL.Query().Get("book")
	oppType := r.URL.Query().Get("type")

	summary, err := h.writer.GetStatsSummary(ctx, startTime, endTime, bookKey, oppType)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get profitability", err)
		return
	}

	// Return focused profitability metrics
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"net_profit":            summary.NetProfit,
		"roi":                   summary.ROI,
		"avg_clv":               summary.AvgCLV,
		"win_rate":              summary.WinRate,
		"total_bets":            summary.TotalBets,
		"total_opportunities":   summary.TotalOpportunities,
		"execution_rate":        summary.ExecutionRate,
		"avg_hold_time_seconds": summary.AvgHoldTimeSeconds,
		"avg_edge_pct":          summary.AvgEdgePct,
		"by_book":               summary.ByBook,
		"by_type":               summary.ByType,
		"start_time":            startTime,
		"end_time":              endTime,
	})
}

// GetBookStats handles GET /stats/books
func (h *Handler) GetBookStats(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		h.requestDuration.WithLabelValues("/stats/books", "GET").Observe(time.Since(start).Seconds())
	}()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Parse query parameters
	startTime, endTime := parseTimeRange(r)
	oppType := r.URL.Query().Get("type")

	summary, err := h.writer.GetStatsSummary(ctx, startTime, endTime, "", oppType)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get book stats", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"books":      summary.ByBook,
		"start_time": startTime,
		"end_time":   endTime,
	})
}

// GetExecutionStats handles GET /stats/execution (Phase 2)
func (h *Handler) GetExecutionStats(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		h.requestDuration.WithLabelValues("/stats/execution", "GET").Observe(time.Since(start).Seconds())
	}()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Parse query parameters
	startTime, endTime := parseTimeRange(r)
	bookKey := r.URL.Query().Get("book")

	stats, err := h.writer.GetExecutionStats(ctx, startTime, endTime, bookKey)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get execution stats", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"execution_stats": stats,
		"start_time":      startTime,
		"end_time":        endTime,
		"book_filter":     bookKey,
	})
}

// GetHoldTimeStats handles GET /stats/hold-time (Phase 2)
// This is an alias/focused view of execution stats specifically for hold time analysis
func (h *Handler) GetHoldTimeStats(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		h.requestDuration.WithLabelValues("/stats/hold-time", "GET").Observe(time.Since(start).Seconds())
	}()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Parse query parameters
	startTime, endTime := parseTimeRange(r)
	bookKey := r.URL.Query().Get("book")

	stats, err := h.writer.GetExecutionStats(ctx, startTime, endTime, bookKey)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get hold time stats", err)
		return
	}

	// Return focused hold time metrics
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"avg_hold_time_seconds": stats.AvgHoldTimeSeconds,
		"min_hold_time_seconds": stats.MinHoldTimeSeconds,
		"max_hold_time_seconds": stats.MaxHoldTimeSeconds,
		"total_opportunities":   stats.TotalOpportunities,
		"execution_rate":        stats.ExecutionRate,
		"conversion_by_book":    stats.ConversionByBook,
		"interpretation": map[string]interface{}{
			"avg_window_description": fmt.Sprintf("Average opportunity window is %d seconds", stats.AvgHoldTimeSeconds),
			"execution_description":  fmt.Sprintf("%.1f%% of opportunities are being converted to bets", stats.ExecutionRate),
		},
		"start_time":  startTime,
		"end_time":    endTime,
		"book_filter": bookKey,
	})
}

// GetEdgeDistribution handles GET /stats/edge-distribution (Phase 3)
func (h *Handler) GetEdgeDistribution(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		h.requestDuration.WithLabelValues("/stats/edge-distribution", "GET").Observe(time.Since(start).Seconds())
	}()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Parse query parameters
	startTime, endTime := parseTimeRange(r)
	bookKey := r.URL.Query().Get("book")
	oppType := r.URL.Query().Get("type")

	distribution, err := h.writer.GetEdgeDistribution(ctx, startTime, endTime, bookKey, oppType)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get edge distribution", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"distribution": distribution,
		"start_time":   startTime,
		"end_time":     endTime,
		"book_filter":  bookKey,
		"type_filter":  oppType,
	})
}

// parseTimeRange parses start and end time from query parameters
func parseTimeRange(r *http.Request) (time.Time, time.Time) {
	// Default to last 24 hours
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	// Parse start_time if provided
	if startStr := r.URL.Query().Get("start_time"); startStr != "" {
		if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			startTime = t
		}
	}

	// Parse end_time if provided
	if endStr := r.URL.Query().Get("end_time"); endStr != "" {
		if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			endTime = t
		}
	}

	// Parse hours parameter (e.g., hours=12 for last 12 hours)
	if hoursStr := r.URL.Query().Get("hours"); hoursStr != "" {
		var hours float64
		if _, err := fmt.Sscanf(hoursStr, "%f", &hours); err == nil {
			endTime = time.Now()
			startTime = endTime.Add(-time.Duration(hours) * time.Hour)
		}
	}

	// Parse days parameter (e.g., days=7 for last week)
	if daysStr := r.URL.Query().Get("days"); daysStr != "" {
		var days int
		if _, err := fmt.Sscanf(daysStr, "%d", &days); err == nil {
			endTime = time.Now()
			startTime = endTime.Add(-time.Duration(days) * 24 * time.Hour)
		}
	}

	return startTime, endTime
}

// respondJSON sends a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError sends an error response
func respondError(w http.ResponseWriter, status int, message string, err error) {
	response := map[string]interface{}{
		"error":  message,
		"status": status,
	}
	if err != nil {
		response["detail"] = err.Error()
	}
	respondJSON(w, status, response)
}

// GetBestBookPairs handles GET /stats/book-pairs
// Returns best performing book pairs for scalps and middles
func (h *Handler) GetBestBookPairs(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		h.requestDuration.WithLabelValues("/stats/book-pairs", "GET").Observe(time.Since(start).Seconds())
	}()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Parse query parameters
	startTime, endTime := parseTimeRange(r)
	oppType := r.URL.Query().Get("type") // "scalp", "middle", or empty for both

	limit := 10 // Default
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}

	pairs, err := h.writer.GetBestBookPairs(ctx, startTime, endTime, oppType, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get book pairs", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"book_pairs":  pairs,
		"count":       len(pairs),
		"type_filter": oppType,
		"start_time":  startTime,
		"end_time":    endTime,
		"description": "Best performing book combinations for scalps/middles",
	})
}

// GetScalpPairs handles GET /stats/scalp-pairs
// Returns best performing book pairs specifically for scalps (arbitrage)
func (h *Handler) GetScalpPairs(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		h.requestDuration.WithLabelValues("/stats/scalp-pairs", "GET").Observe(time.Since(start).Seconds())
	}()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	startTime, endTime := parseTimeRange(r)

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}

	pairs, err := h.writer.GetBestBookPairs(ctx, startTime, endTime, "scalp", limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get scalp pairs", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"scalp_pairs": pairs,
		"count":       len(pairs),
		"start_time":  startTime,
		"end_time":    endTime,
		"description": "Best book combinations for arbitrage opportunities",
		"insight":     generateScalpInsight(pairs),
	})
}

// GetMiddlePairs handles GET /stats/middle-pairs
// Returns best performing book pairs specifically for middles
func (h *Handler) GetMiddlePairs(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		h.requestDuration.WithLabelValues("/stats/middle-pairs", "GET").Observe(time.Since(start).Seconds())
	}()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	startTime, endTime := parseTimeRange(r)

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}

	pairs, err := h.writer.GetBestBookPairs(ctx, startTime, endTime, "middle", limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get middle pairs", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"middle_pairs": pairs,
		"count":        len(pairs),
		"start_time":   startTime,
		"end_time":     endTime,
		"description":  "Best book combinations for middle opportunities (both sides can win)",
		"insight":      generateMiddleInsight(pairs),
	})
}

// generateScalpInsight generates actionable insight for scalp pairs
func generateScalpInsight(pairs []models.BookPairSummary) string {
	if len(pairs) == 0 {
		return "No scalp opportunities detected in this time period."
	}
	return "Focus on the top book pairs for highest arbitrage frequency. Pairs with higher execution rates indicate faster line execution."
}

// generateMiddleInsight generates actionable insight for middle pairs
func generateMiddleInsight(pairs []models.BookPairSummary) string {
	if len(pairs) == 0 {
		return "No middle opportunities detected in this time period."
	}
	return "Middle opportunities between these book pairs have the highest chance of both sides winning. Consider higher stakes when edge > 3%."
}

// =========================================================================
// OPPORTUNITY CLV ENDPOINTS - Edge Detector Validation
// =========================================================================

// GetOpportunityCLV handles GET /stats/opportunity-clv
// Returns CLV analysis for all detected opportunities (validates edge detector accuracy)
func (h *Handler) GetOpportunityCLV(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		h.requestDuration.WithLabelValues("/stats/opportunity-clv", "GET").Observe(time.Since(start).Seconds())
	}()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Parse query parameters
	startTime, endTime := parseTimeRange(r)
	bookKey := r.URL.Query().Get("book")
	oppType := r.URL.Query().Get("type")
	includeTimeSeries := r.URL.Query().Get("timeseries") == "true"

	// Get summary
	summary, err := h.writer.GetOpportunityCLVSummary(ctx, startTime, endTime, bookKey, oppType)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get opportunity CLV summary", err)
		return
	}

	// Get breakdowns
	if bookKey == "" {
		summary.ByBook, _ = h.writer.GetOpportunityCLVByBook(ctx, startTime, endTime, oppType)
	}
	if oppType == "" {
		summary.ByType, _ = h.writer.GetOpportunityCLVByType(ctx, startTime, endTime, bookKey)
	}

	response := models.OpportunityCLVResponse{
		Summary: summary,
	}

	// Include time series if requested
	if includeTimeSeries {
		interval := r.URL.Query().Get("interval")
		if interval == "" {
			interval = "hour"
		}
		response.TimeSeries, _ = h.writer.GetEdgeAccuracyTimeSeries(ctx, startTime, endTime, interval)
	}

	// Add interpretation message
	if summary.TotalOpportunities == 0 {
		response.Message = "No opportunity CLV data available. Ensure CLV Calculator is running and games have gone live."
	} else if summary.EdgeAccuracy >= 70 {
		response.Message = fmt.Sprintf("Edge detector is highly accurate (%.1f%% of detected edges hold value at close)", summary.EdgeAccuracy)
	} else if summary.EdgeAccuracy >= 50 {
		response.Message = fmt.Sprintf("Edge detector accuracy is moderate (%.1f%%). Consider increasing minimum edge threshold.", summary.EdgeAccuracy)
	} else {
		response.Message = fmt.Sprintf("Edge detector accuracy is low (%.1f%%). Review detection parameters.", summary.EdgeAccuracy)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"opportunity_clv": response,
		"start_time":      startTime,
		"end_time":        endTime,
		"book_filter":     bookKey,
		"type_filter":     oppType,
		"description":     "Measures how often detected edges are real by comparing opportunity prices to closing lines",
		"metrics_explained": map[string]string{
			"avg_clv":             "Average CLV in cents. Positive = detected prices beat closing line on average",
			"edge_accuracy":       "Percentage of opportunities where CLV > 0 (edge was real)",
			"false_positive_rate": "Percentage of opportunities where edge evaporated (CLV <= 0)",
			"avg_edge_decay":      "How much the detected edge decreases by close time",
		},
	})
}

// GetEdgeAccuracy handles GET /stats/edge-accuracy
// Returns focused edge accuracy metrics over time
func (h *Handler) GetEdgeAccuracy(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		h.requestDuration.WithLabelValues("/stats/edge-accuracy", "GET").Observe(time.Since(start).Seconds())
	}()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Parse query parameters
	startTime, endTime := parseTimeRange(r)
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "hour"
	}

	// Get time series
	timeSeries, err := h.writer.GetEdgeAccuracyTimeSeries(ctx, startTime, endTime, interval)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get edge accuracy time series", err)
		return
	}

	// Get overall summary
	summary, err := h.writer.GetOpportunityCLVSummary(ctx, startTime, endTime, "", "")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get edge accuracy summary", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"time_series": timeSeries,
		"summary": map[string]interface{}{
			"overall_edge_accuracy": summary.EdgeAccuracy,
			"avg_clv_cents":         summary.AvgCLV,
			"total_opportunities":   summary.TotalOpportunities,
			"avg_edge_at_detection": summary.AvgEdgeAtDetection,
			"avg_edge_decay":        summary.AvgEdgeDecay,
			"positive_clv_count":    summary.PositiveCLVCount,
			"negative_clv_count":    summary.NegativeCLVCount,
		},
		"start_time":  startTime,
		"end_time":    endTime,
		"interval":    interval,
		"description": "Tracks how edge detection accuracy changes over time",
		"interpretation": map[string]interface{}{
			"edge_accuracy_meaning": "Percentage of detected opportunities that had positive CLV at close",
			"good_accuracy":         ">70% indicates reliable edge detection",
			"concerning_accuracy":   "<50% indicates potential issues with detection parameters",
		},
	})
}

// =========================================================================
// PAIR PERFORMANCE ENDPOINTS - Volume & ROI Decisions
// =========================================================================

// GetPairPerformance handles GET /stats/pair-performance
// Returns historical handle, profit, and ROI per book pair
func (h *Handler) GetPairPerformance(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		h.requestDuration.WithLabelValues("/stats/pair-performance", "GET").Observe(time.Since(start).Seconds())
	}()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Parse query parameters
	oppType := r.URL.Query().Get("type") // "scalp", "middle", or empty for both
	limit := 20                          // Default
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		fmt.Sscanf(limitStr, "%d", &limit)
	}

	pairs, err := h.writer.GetPairPerformance(ctx, oppType, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get pair performance", err)
		return
	}

	// Generate insights
	var recommendations []string
	for _, p := range pairs {
		if p.TotalBetsPlaced >= 5 { // Need sample size
			if p.ROIPct >= 3 {
				recommendations = append(recommendations, fmt.Sprintf("🟢 %s: Strong ROI (%.1f%%). Consider increasing volume.", p.PairName, p.ROIPct))
			} else if p.ROIPct <= -3 {
				recommendations = append(recommendations, fmt.Sprintf("🔴 %s: Negative ROI (%.1f%%). Consider reducing volume or reviewing execution.", p.PairName, p.ROIPct))
			}
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"pairs":           pairs,
		"count":           len(pairs),
		"type_filter":     oppType,
		"recommendations": recommendations,
		"description":     "Historical performance by book pair. Use this to decide where to push volume.",
		"metrics_explained": map[string]string{
			"total_handle":    "Total amount wagered on this pair",
			"realized_profit": "Net profit from settled bets",
			"roi_pct":         "(Profit / Handle) × 100 - Your return on investment",
			"execution_rate":  "% of detected opportunities you actually bet on",
		},
	})
}

// RefreshPairPerformance handles POST /stats/pair-performance/refresh
// Recalculates pair performance from source data
func (h *Handler) RefreshPairPerformance(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		h.requestDuration.WithLabelValues("/stats/pair-performance/refresh", "POST").Observe(time.Since(start).Seconds())
	}()

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	err := h.writer.RefreshPairPerformance(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to refresh pair performance", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "success",
		"message":     "Pair performance metrics refreshed",
		"duration_ms": time.Since(start).Milliseconds(),
	})
}
