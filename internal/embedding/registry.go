package embedding

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sync"
)

// ProviderConfig holds common configuration for embedding providers.
type ProviderConfig struct {
	Model     string
	BatchSize int
}

// Factory creates an Embedder from a ProviderConfig.
// It returns the embedder, a human-readable info string, and any error.
type Factory func(ctx context.Context, cfg ProviderConfig) (Embedder, string, error)

var (
	mu        sync.Mutex
	providers = make(map[string]Factory)
)

// Register adds a named provider factory. It panics on duplicate names.
func Register(name string, factory Factory) {
	mu.Lock()
	defer mu.Unlock()
	if _, exists := providers[name]; exists {
		panic(fmt.Sprintf("embedding: provider %q already registered", name))
	}
	providers[name] = factory
}

// Create instantiates a named provider.
func Create(ctx context.Context, name string, cfg ProviderConfig) (Embedder, string, error) {
	mu.Lock()
	factory, ok := providers[name]
	mu.Unlock()
	if !ok {
		return nil, "", fmt.Errorf("embedding: unknown provider %q", name)
	}
	return factory(ctx, cfg)
}

// List returns sorted provider names.
func List() []string {
	mu.Lock()
	defer mu.Unlock()
	return slices.Sorted(maps.Keys(providers))
}
