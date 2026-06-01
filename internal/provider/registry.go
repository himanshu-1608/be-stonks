package provider

import "fmt"

// Constructor builds a Provider from neutral config.
type Constructor func(cfg Config) (Provider, error)

var registry = map[string]Constructor{}

// Register adds a provider constructor under a name. Called from provider
// packages' init() functions.
func Register(name string, c Constructor) {
	registry[name] = c
}

// New constructs the named provider.
func New(name string, cfg Config) (Provider, error) {
	c, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", name)
	}
	return c(cfg)
}
