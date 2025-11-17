package basketball_nba

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fortuna/services/game-stats-service/pkg/models"
)

// ESPN stat indices for NBA
// Based on: MIN, PTS, OREB, DREB, REB, AST, STL, BLK, TO, FG, FG%, 3PT, 3PT%, FT, FT%, PF, +/-
const (
	idxMinutes = 0
	idxPoints = 1
	idxOffReb = 2
	idxDefReb = 3
	idxReb = 4
	idxAst = 5
	idxStl = 6
	idxBlk = 7
	idxTO = 8
	idxFG = 9
	idxFGPct = 10
	idx3PT = 11
	idx3PTPct = 12
	idxFT = 13
	idxFTPct = 14
	idxPF = 15
	idxPlusMinus = 16
)

// parseFloat parses a float from interface{}
func parseFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case int:
		return float64(val)
	default:
		return 0.0
	}
}

// parseInt parses an int from interface{}
func parseInt(v interface{}) int {
	switch val := v.(type) {
	case float64:
		return int(val)
	case string:
		i, _ := strconv.Atoi(val)
		return i
	case int:
		return val
	default:
		return 0
	}
}

// parseMinutes converts ESPN minutes format to float
func parseMinutes(minutesStr string) float64 {
	if minutesStr == "" || minutesStr == "0" {
		return 0.0
	}

	// Handle "33" or "33:15" format
	if strings.Contains(minutesStr, ":") {
		parts := strings.Split(minutesStr, ":")
		mins, _ := strconv.Atoi(parts[0])
		secs := 0
		if len(parts) > 1 {
			secs, _ = strconv.Atoi(parts[1])
		}
		return float64(mins) + (float64(secs) / 60.0)
	}

	f, _ := strconv.ParseFloat(minutesStr, 64)
	return f
}

// parsePlusMinus parses +/- stat
func parsePlusMinus(pmStr string) int {
	if pmStr == "" || pmStr == "0" {
		return 0
	}
	pmStr = strings.Replace(pmStr, "+", "", -1)
	i, _ := strconv.Atoi(pmStr)
	return i
}

// parseGameStatus converts ESPN status to our GameStatus
func parseGameStatus(statusType map[string]interface{}) models.GameStatus {
	if completed, ok := statusType["completed"].(bool); ok && completed {
		return models.StatusFinal
	}

	if state, ok := statusType["state"].(string); ok {
		switch state {
		case "in":
			return models.StatusLive
		case "pre":
			return models.StatusUpcoming
		case "post":
			return models.StatusFinal
		}
	}

	return models.StatusUpcoming
}

// parseCommenceTime parses ESPN date format to time.Time
func parseCommenceTime(dateStr string) time.Time {
	if dateStr == "" {
		return time.Now()
	}

	// ESPN format: "2025-11-11T23:30Z"
	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		// Try without Z
		t, err = time.Parse("2006-01-02T15:04:05", strings.TrimSuffix(dateStr, "Z"))
		if err != nil {
			return time.Now()
		}
	}

	return t
}

// getPeriodLabel returns NBA-specific period label
func getPeriodLabel(period int) string {
	switch period {
	case 1:
		return "Q1"
	case 2:
		return "Q2"
	case 3:
		return "Q3"
	case 4:
		return "Q4"
	default:
		if period > 4 {
			return fmt.Sprintf("OT%d", period-4)
		}
		return fmt.Sprintf("Q%d", period)
	}
}

// extractString safely extracts a string from a map
func extractString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if str, ok := v.(string); ok {
			return str
		}
	}
	return ""
}

// extractInt safely extracts an int from a map
func extractInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		return parseInt(v)
	}
	return 0
}

// extractMap safely extracts a map from a map
func extractMap(m map[string]interface{}, key string) map[string]interface{} {
	if v, ok := m[key]; ok {
		if mapVal, ok := v.(map[string]interface{}); ok {
			return mapVal
		}
	}
	return map[string]interface{}{}
}

// extractArray safely extracts an array from a map
func extractArray(m map[string]interface{}, key string) []interface{} {
	if v, ok := m[key]; ok {
		if arrVal, ok := v.([]interface{}); ok {
			return arrVal
		}
	}
	return []interface{}{}
}




