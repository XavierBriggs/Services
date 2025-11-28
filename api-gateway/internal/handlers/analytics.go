package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AnalyticsHandler handles proxying requests to Analytics service
type AnalyticsHandler struct {
	analyticsURL string
	httpClient   *http.Client
}

// NewAnalyticsHandler creates a new Analytics handler
func NewAnalyticsHandler(analyticsURL string) *AnalyticsHandler {
	return &AnalyticsHandler{
		analyticsURL: analyticsURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// proxyToAnalytics forwards requests to the Analytics service
func (h *AnalyticsHandler) proxyToAnalytics(w http.ResponseWriter, r *http.Request, path string) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s%s", h.analyticsURL, path)

	// Add query parameters
	if r.URL.RawQuery != "" {
		url = fmt.Sprintf("%s?%s", url, r.URL.RawQuery)
	}

	// Create new request
	proxyReq, err := http.NewRequestWithContext(ctx, r.Method, url, r.Body)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create proxy request", err)
		return
	}

	// Copy headers
	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	// Execute request
	resp, err := h.httpClient.Do(proxyReq)
	if err != nil {
		respondError(w, http.StatusBadGateway, "analytics service unavailable", err)
		return
	}
	defer resp.Body.Close()

	// Copy response headers (skip CORS headers - handled by middleware)
	for key, values := range resp.Header {
		// Skip CORS headers to avoid duplicates
		if key == "Access-Control-Allow-Origin" ||
			key == "Access-Control-Allow-Methods" ||
			key == "Access-Control-Allow-Headers" ||
			key == "Access-Control-Max-Age" ||
			key == "Access-Control-Allow-Credentials" {
			continue
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Copy status code
	w.WriteHeader(resp.StatusCode)

	// Copy body
	if _, err := io.Copy(w, resp.Body); err != nil {
		fmt.Printf("error copying response body: %v\n", err)
	}
}

// GetStatsSummary retrieves stats summary from Analytics
func (h *AnalyticsHandler) GetStatsSummary(w http.ResponseWriter, r *http.Request) {
	h.proxyToAnalytics(w, r, "/stats/summary")
}

// GetTimeSeries retrieves time series data from Analytics
func (h *AnalyticsHandler) GetTimeSeries(w http.ResponseWriter, r *http.Request) {
	h.proxyToAnalytics(w, r, "/stats/timeseries")
}

// GetProfitability retrieves profitability metrics from Analytics
func (h *AnalyticsHandler) GetProfitability(w http.ResponseWriter, r *http.Request) {
	h.proxyToAnalytics(w, r, "/stats/profitability")
}

// GetBookStats retrieves book-level statistics from Analytics
func (h *AnalyticsHandler) GetBookStats(w http.ResponseWriter, r *http.Request) {
	h.proxyToAnalytics(w, r, "/stats/books")
}

// GetScalpPairs retrieves best scalp book pairs from Analytics
func (h *AnalyticsHandler) GetScalpPairs(w http.ResponseWriter, r *http.Request) {
	h.proxyToAnalytics(w, r, "/stats/scalp-pairs")
}

// GetMiddlePairs retrieves best middle book pairs from Analytics
func (h *AnalyticsHandler) GetMiddlePairs(w http.ResponseWriter, r *http.Request) {
	h.proxyToAnalytics(w, r, "/stats/middle-pairs")
}

// GetExecutionStats retrieves execution/hold time statistics from Analytics
func (h *AnalyticsHandler) GetExecutionStats(w http.ResponseWriter, r *http.Request) {
	h.proxyToAnalytics(w, r, "/stats/execution")
}

// GetEdgeDistribution retrieves edge distribution data from Analytics
func (h *AnalyticsHandler) GetEdgeDistribution(w http.ResponseWriter, r *http.Request) {
	h.proxyToAnalytics(w, r, "/stats/edge-distribution")
}

// GetHoldTimeStats retrieves hold time statistics from Analytics
func (h *AnalyticsHandler) GetHoldTimeStats(w http.ResponseWriter, r *http.Request) {
	h.proxyToAnalytics(w, r, "/stats/hold-time")
}

// GetBestBookPairs retrieves best book pairs for scalps/middles
func (h *AnalyticsHandler) GetBestBookPairs(w http.ResponseWriter, r *http.Request) {
	h.proxyToAnalytics(w, r, "/stats/book-pairs")
}

// =========================================================================
// OPPORTUNITY CLV ENDPOINTS - Edge Detector Validation
// =========================================================================

// GetOpportunityCLV retrieves opportunity CLV analysis (validates edge detector)
func (h *AnalyticsHandler) GetOpportunityCLV(w http.ResponseWriter, r *http.Request) {
	h.proxyToAnalytics(w, r, "/stats/opportunity-clv")
}

// GetEdgeAccuracy retrieves edge accuracy over time
func (h *AnalyticsHandler) GetEdgeAccuracy(w http.ResponseWriter, r *http.Request) {
	h.proxyToAnalytics(w, r, "/stats/edge-accuracy")
}

// =========================================================================
// PAIR PERFORMANCE ENDPOINTS - Volume & ROI Decisions
// =========================================================================

// GetPairPerformance retrieves historical handle, profit, ROI per book pair
func (h *AnalyticsHandler) GetPairPerformance(w http.ResponseWriter, r *http.Request) {
	h.proxyToAnalytics(w, r, "/stats/pair-performance")
}

// RefreshPairPerformance recalculates pair performance from source data
func (h *AnalyticsHandler) RefreshPairPerformance(w http.ResponseWriter, r *http.Request) {
	h.proxyToAnalytics(w, r, "/stats/pair-performance/refresh")
}
