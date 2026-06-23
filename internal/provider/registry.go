package provider

import "sort"

// registry holds the configured providers, keyed by Name(). It is populated at
// startup (main wires claude in) so the rest of fleet can look a provider up by
// the name stored on a Session.
var registry = map[string]Provider{}

// Register adds a provider. Re-registering the same name overwrites — harmless
// and convenient for tests.
func Register(p Provider) { registry[p.Name()] = p }

// Get returns the provider with name and whether it is configured.
func Get(name string) (Provider, bool) {
	p, ok := registry[name]
	return p, ok
}

// All returns the configured providers, name-sorted for stable display.
func All() []Provider {
	out := make([]Provider, 0, len(registry))
	for _, p := range registry {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// Count is how many providers are configured. The /new wizard hides its provider
// step while this is < 2 (a one-option step is pointless — §4.4).
func Count() int { return len(registry) }
