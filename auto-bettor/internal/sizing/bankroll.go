package sizing

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/XavierBriggs/fortuna/services/auto-bettor/internal/models"
)

// BankrollManager handles dynamic bankroll fetching from bot service
type BankrollManager struct {
	botServiceURL string
	httpClient    *http.Client
	balanceCache  *BalanceCache
}

// NewBankrollManager creates a new bankroll manager
func NewBankrollManager(botServiceURL string, cacheTTL time.Duration) *BankrollManager {
	return &BankrollManager{
		botServiceURL: botServiceURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		balanceCache: NewBalanceCache(cacheTTL),
	}
}

// GetBankroll gets the live balance for a specific book
func (bm *BankrollManager) GetBankroll(bookKey string) (*models.BotBalance, error) {
	// Check cache first
	if cached := bm.balanceCache.Get(bookKey); cached != nil {
		return cached, nil
	}

	// Fetch from bot service
	resp, err := bm.httpClient.Get(bm.botServiceURL + "/api/v1/bots/status")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch bot status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bot service returned status %d", resp.StatusCode)
	}

	var botStatus struct {
		Bots []struct {
			Name    string  `json:"name"`
			Balance *string `json:"balance"`
		} `json:"bots"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&botStatus); err != nil {
		return nil, fmt.Errorf("failed to parse bot status: %w", err)
	}

	// Find the requested bot
	for _, bot := range botStatus.Bots {
		if bot.Name == bookKey && bot.Balance != nil {
			balance, err := parseBalance(*bot.Balance)
			if err != nil {
				return nil, fmt.Errorf("failed to parse balance for %s: %w", bookKey, err)
			}

			botBalance := &models.BotBalance{
				BookKey:     bookKey,
				Balance:     balance,
				FetchedAt:   time.Now(),
				IsAvailable: true,
			}

			// Cache the balance
			bm.balanceCache.Set(bookKey, botBalance)

			return botBalance, nil
		}
	}

	return nil, fmt.Errorf("bot not found for book: %s", bookKey)
}

// InvalidateCache invalidates the balance cache for a specific book
func (bm *BankrollManager) InvalidateCache(bookKey string) {
	bm.balanceCache.Invalidate(bookKey)
}

// parseBalance parses balance strings like "$1,234.56" -> 1234.56
func parseBalance(balanceStr string) (float64, error) {
	// Remove common formatting characters
	cleaned := strings.TrimSpace(balanceStr)
	cleaned = strings.ReplaceAll(cleaned, "$", "")
	cleaned = strings.ReplaceAll(cleaned, ",", "")
	cleaned = strings.TrimSpace(cleaned)

	var balance float64
	_, err := fmt.Sscanf(cleaned, "%f", &balance)
	if err != nil {
		return 0, fmt.Errorf("failed to parse balance '%s': %w", balanceStr, err)
	}

	if balance < 0 {
		return 0, fmt.Errorf("balance cannot be negative: %f", balance)
	}

	return balance, nil
}

// BalanceCache caches bot balances to reduce API calls
type BalanceCache struct {
	cache map[string]*cachedBalance
	ttl   time.Duration
}

type cachedBalance struct {
	balance   *models.BotBalance
	expiresAt time.Time
}

// NewBalanceCache creates a new balance cache
func NewBalanceCache(ttl time.Duration) *BalanceCache {
	return &BalanceCache{
		cache: make(map[string]*cachedBalance),
		ttl:   ttl,
	}
}

// Get retrieves a cached balance if not expired
func (bc *BalanceCache) Get(bookKey string) *models.BotBalance {
	cached, exists := bc.cache[bookKey]
	if !exists {
		return nil
	}

	if time.Now().After(cached.expiresAt) {
		// Expired
		delete(bc.cache, bookKey)
		return nil
	}

	return cached.balance
}

// Set caches a balance
func (bc *BalanceCache) Set(bookKey string, balance *models.BotBalance) {
	bc.cache[bookKey] = &cachedBalance{
		balance:   balance,
		expiresAt: time.Now().Add(bc.ttl),
	}
}

// Invalidate removes a cached balance
func (bc *BalanceCache) Invalidate(bookKey string) {
	delete(bc.cache, bookKey)
}
