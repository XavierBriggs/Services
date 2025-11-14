package registry

import (
	"fmt"

	"github.com/fortuna/services/game-stats-service/internal/sports/basketball_nba"
	"github.com/fortuna/services/game-stats-service/pkg/contracts"
)

// Registry manages available sport modules
type Registry struct {
	modules map[string]contracts.SportModule
}

// New creates a new sport registry with all available sports
func New() *Registry {
	r := &Registry{
		modules: make(map[string]contracts.SportModule),
	}

	// Register NBA (active in v0)
	r.Register(basketball_nba.New())

	// Future sports (uncomment when ready):
	// r.Register(american_football_nfl.New())
	// r.Register(baseball_mlb.New())

	return r
}

// Register adds a sport module to the registry
func (r *Registry) Register(module contracts.SportModule) {
	r.modules[module.GetSportKey()] = module
}

// GetModule retrieves a sport module by key
func (r *Registry) GetModule(sportKey string) (contracts.SportModule, error) {
	module, ok := r.modules[sportKey]
	if !ok {
		return nil, fmt.Errorf("sport module not found: %s", sportKey)
	}
	return module, nil
}

// EnabledSports returns all enabled sport modules
func (r *Registry) EnabledSports() []contracts.SportModule {
	var enabled []contracts.SportModule
	for _, m := range r.modules {
		if m.IsEnabled() {
			enabled = append(enabled, m)
		}
	}
	return enabled
}

// AllSportKeys returns all registered sport keys
func (r *Registry) AllSportKeys() []string {
	keys := make([]string, 0, len(r.modules))
	for key := range r.modules {
		keys = append(keys, key)
	}
	return keys
}



