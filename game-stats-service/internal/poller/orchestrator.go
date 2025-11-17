package poller

import (
	"context"
	"log"
	"sync"

	"github.com/fortuna/services/game-stats-service/internal/cache"
	"github.com/fortuna/services/game-stats-service/internal/providers/espn"
	"github.com/fortuna/services/game-stats-service/internal/publisher"
	"github.com/fortuna/services/game-stats-service/internal/registry"
)

// Orchestrator manages pollers for all enabled sports
type Orchestrator struct {
	registry   *registry.Registry
	espnClient *espn.Client
	cache      *cache.RedisWriter
	publisher  *publisher.StreamPublisher
	pollers    map[string]*SportPoller
}

// NewOrchestrator creates a new polling orchestrator
func NewOrchestrator(
	reg *registry.Registry,
	espnClient *espn.Client,
	cache *cache.RedisWriter,
	publisher *publisher.StreamPublisher,
) *Orchestrator {
	return &Orchestrator{
		registry:   reg,
		espnClient: espnClient,
		cache:      cache,
		publisher:  publisher,
		pollers:    make(map[string]*SportPoller),
	}
}

// Start launches pollers for all enabled sports
func (o *Orchestrator) Start(ctx context.Context) {
	var wg sync.WaitGroup

	enabledSports := o.registry.EnabledSports()
	log.Printf("Starting pollers for %d enabled sports", len(enabledSports))

	for _, module := range enabledSports {
		sportKey := module.GetSportKey()
		
		poller := NewSportPoller(module, o.espnClient, o.cache, o.publisher)
		o.pollers[sportKey] = poller

		wg.Add(1)
		go func(p *SportPoller, key string) {
			defer wg.Done()
			p.Run(ctx)
		}(poller, sportKey)

		log.Printf("Started poller for %s", sportKey)
	}

	wg.Wait()
	log.Println("All pollers stopped")
}




