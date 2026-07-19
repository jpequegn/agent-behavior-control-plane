package flags

import (
	"context"
	"sync"

	of "github.com/open-feature/go-sdk/openfeature"
)

// LocalProvider is a deterministic OpenFeature provider for the synthetic lab.
// It accepts a restricted, flagd-style static configuration rather than relying on a running daemon.
type LocalProvider struct {
	mu     sync.RWMutex
	config Config
}

func NewLocalProvider(config Config) (*LocalProvider, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &LocalProvider{config: copyConfig(config)}, nil
}

func (p *LocalProvider) Metadata() of.Metadata {
	return of.Metadata{Name: "abcp-local-flagd-compatible"}
}

func (p *LocalProvider) Hooks() []of.Hook {
	return nil
}

func (p *LocalProvider) Update(config Config) error {
	if err := config.Validate(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config = copyConfig(config)
	return nil
}

func (p *LocalProvider) Config() Config {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return copyConfig(p.config)
}

func (p *LocalProvider) BooleanEvaluation(_ context.Context, flag string, defaultValue bool, _ of.FlattenedContext) of.BoolResolutionDetail {
	value, detail := p.resolve(flag)
	if detail.Error() != nil {
		return of.BoolResolutionDetail{Value: defaultValue, ProviderResolutionDetail: detail}
	}
	boolean, ok := value.(bool)
	if !ok {
		detail.ResolutionError = of.NewTypeMismatchResolutionError("flag is not boolean")
		return of.BoolResolutionDetail{Value: defaultValue, ProviderResolutionDetail: detail}
	}
	return of.BoolResolutionDetail{Value: boolean, ProviderResolutionDetail: detail}
}

func (p *LocalProvider) StringEvaluation(_ context.Context, flag string, defaultValue string, _ of.FlattenedContext) of.StringResolutionDetail {
	value, detail := p.resolve(flag)
	if detail.Error() != nil {
		return of.StringResolutionDetail{Value: defaultValue, ProviderResolutionDetail: detail}
	}
	text, ok := value.(string)
	if !ok {
		detail.ResolutionError = of.NewTypeMismatchResolutionError("flag is not string")
		return of.StringResolutionDetail{Value: defaultValue, ProviderResolutionDetail: detail}
	}
	return of.StringResolutionDetail{Value: text, ProviderResolutionDetail: detail}
}

func (p *LocalProvider) FloatEvaluation(_ context.Context, _ string, defaultValue float64, _ of.FlattenedContext) of.FloatResolutionDetail {
	return of.FloatResolutionDetail{Value: defaultValue, ProviderResolutionDetail: unsupportedTypeDetail()}
}

func (p *LocalProvider) IntEvaluation(_ context.Context, _ string, defaultValue int64, _ of.FlattenedContext) of.IntResolutionDetail {
	return of.IntResolutionDetail{Value: defaultValue, ProviderResolutionDetail: unsupportedTypeDetail()}
}

func (p *LocalProvider) ObjectEvaluation(_ context.Context, _ string, defaultValue any, _ of.FlattenedContext) of.InterfaceResolutionDetail {
	return of.InterfaceResolutionDetail{Value: defaultValue, ProviderResolutionDetail: unsupportedTypeDetail()}
}

func (p *LocalProvider) resolve(flag string) (any, of.ProviderResolutionDetail) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	value, ok := p.config.Flags[flag]
	if !ok {
		return nil, of.ProviderResolutionDetail{ResolutionError: of.NewFlagNotFoundResolutionError("flag is not configured")}
	}
	return value, of.ProviderResolutionDetail{
		FlagMetadata: of.FlagMetadata{"snapshot": p.config.Snapshot()},
		Reason:       of.StaticReason,
		Variant:      "default",
	}
}

func unsupportedTypeDetail() of.ProviderResolutionDetail {
	return of.ProviderResolutionDetail{ResolutionError: of.NewTypeMismatchResolutionError("unsupported flag type")}
}

func copyConfig(config Config) Config {
	flags := make(map[string]any, len(config.Flags))
	for key, value := range config.Flags {
		flags[key] = value
	}
	return Config{Flags: flags}
}
