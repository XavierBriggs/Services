package registry

import (
	"fmt"
	"sync"

	"github.com/XavierBriggs/fortuna/services/normalizer/pkg/contracts"
)

// NormalizerRegistry manages registered sport normalizers
type NormalizerRegistry struct {
	normalizers map[string]contracts.SportNormalizer
	mu          sync.RWMutex
}

// NewNormalizerRegistry creates a new normalizer registry
func NewNormalizerRegistry() *NormalizerRegistry {
	return &NormalizerRegistry{
		normalizers: make(map[string]contracts.SportNormalizer),
	}
}

// Register adds a sport normalizer to the registry
func (r *NormalizerRegistry) Register(normalizer contracts.SportNormalizer) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	sportKey := normalizer.GetSportKey()
	if _, exists := r.normalizers[sportKey]; exists {
		return fmt.Errorf("normalizer for sport %s is already registered", sportKey)
	}

	r.normalizers[sportKey] = normalizer
	return nil
}

// Get retrieves a normalizer by sport key
func (r *NormalizerRegistry) Get(sportKey string) (contracts.SportNormalizer, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	normalizer, exists := r.normalizers[sportKey]
	return normalizer, exists
}

// GetAll returns all registered normalizers
func (r *NormalizerRegistry) GetAll() []contracts.SportNormalizer {
	r.mu.RLock()
	defer r.mu.RUnlock()

	normalizers := make([]contracts.SportNormalizer, 0, len(r.normalizers))
	for _, normalizer := range r.normalizers {
		normalizers = append(normalizers, normalizer)
	}
	return normalizers
}

// Count returns the number of registered normalizers
func (r *NormalizerRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.normalizers)
}




