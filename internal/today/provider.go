package today

import "context"

// Provider contributes named sections to the Today view. Implementations are
// domain-specific (todos, resources, finance, calendar). Each provider is
// independently queried; one failure does not block the others.
type Provider interface {
	Name() string                               // "todos", "resources", ...
	Sections(ctx context.Context) ([]Section, error)
}

// ProviderError captures a provider failure for graceful degradation.
type ProviderError struct {
	Name string
	Err  error
}

// Registry holds ordered providers. v3.0 registers TodosProvider and
// ResourcesProvider. v3.x adds finance/calendar by calling Register.
type Registry struct {
	providers []Provider
}

// NewRegistry builds an empty registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds p to the registry. Providers are queried in registration order.
func (r *Registry) Register(p Provider) {
	r.providers = append(r.providers, p)
}

// Collect iterates registered providers, calls Sections(ctx), and aggregates
// results. On provider error, the error is recorded in []ProviderError and
// execution continues with the next provider.
func (r *Registry) Collect(ctx context.Context) ([]Section, []ProviderError) {
	var sections []Section
	var errs []ProviderError

	for _, p := range r.providers {
		secs, err := p.Sections(ctx)
		if err != nil {
			errs = append(errs, ProviderError{Name: p.Name(), Err: err})
			continue
		}
		sections = append(sections, secs...)
	}
	return sections, errs
}
