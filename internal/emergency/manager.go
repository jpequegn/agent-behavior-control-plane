package emergency

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jpequegn/agent-behavior-control-plane/internal/flags"
)

const TargetPropagationBudget = 500 * time.Millisecond

type Request struct {
	ExpiresAt time.Time `json:"expires_at"`
	Owner     string    `json:"owner"`
	Reason    string    `json:"reason"`
	Target    string    `json:"target"`
	Value     any       `json:"value"`
}

type Mutation struct {
	Request
	AppliedAt  time.Time  `json:"applied_at"`
	ID         string     `json:"id"`
	ObservedAt *time.Time `json:"observed_at,omitempty"`
}

type Manager struct {
	base      flags.Config
	history   []Mutation
	mutations map[string]Mutation
	provider  *flags.LocalProvider
	mu        sync.Mutex
}

func NewManager(provider *flags.LocalProvider) (*Manager, error) {
	if provider == nil {
		return nil, errors.New("flag provider is required")
	}
	return &Manager{base: provider.Config(), mutations: map[string]Mutation{}, provider: provider}, nil
}

func (m *Manager) Provider() *flags.LocalProvider { return m.provider }

func (m *Manager) Apply(request Request, now time.Time) (Mutation, error) {
	if err := validateRequest(request, now); err != nil {
		return Mutation{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked(now)
	id := mutationID(request, now)
	mutation := Mutation{Request: request, AppliedAt: now.UTC(), ID: id}
	m.mutations[id] = mutation
	m.history = append(m.history, mutation)
	if err := m.rebuildLocked(); err != nil {
		delete(m.mutations, id)
		return Mutation{}, err
	}
	return mutation, nil
}

func (m *Manager) Clear(id string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked(now)
	if _, ok := m.mutations[id]; !ok {
		return fmt.Errorf("emergency control %q not found", id)
	}
	delete(m.mutations, id)
	return m.rebuildLocked()
}

// BeforeBoundary expires stale controls and captures the first boundary that observed each one.
func (m *Manager) BeforeBoundary(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked(now)
	changed := false
	for id, mutation := range m.mutations {
		if mutation.ObservedAt == nil {
			observed := now.UTC()
			mutation.ObservedAt = &observed
			m.mutations[id] = mutation
			m.history = append(m.history, mutation)
			changed = true
		}
	}
	if changed {
		_ = m.rebuildLocked()
	}
}

func (m *Manager) Active(now time.Time) []Mutation {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked(now)
	result := make([]Mutation, 0, len(m.mutations))
	for _, mutation := range m.mutations {
		result = append(result, mutation)
	}
	return result
}

func (m *Manager) History() []Mutation {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Mutation(nil), m.history...)
}

func (m *Manager) expireLocked(now time.Time) {
	changed := false
	for id, mutation := range m.mutations {
		if !now.Before(mutation.ExpiresAt) {
			delete(m.mutations, id)
			changed = true
		}
	}
	if changed {
		_ = m.rebuildLocked()
	}
}

func (m *Manager) rebuildLocked() error {
	config := copyConfig(m.base)
	for _, mutation := range m.mutations {
		if err := applyMutation(&config, mutation); err != nil {
			return err
		}
	}
	return m.provider.Update(config)
}

func validateRequest(request Request, now time.Time) error {
	if request.Owner == "" || request.Reason == "" {
		return errors.New("owner and reason are required")
	}
	if !request.ExpiresAt.After(now) {
		return errors.New("expiry must be in the future")
	}
	if request.Target == "agent.global_halt" || strings.HasPrefix(request.Target, "tool.") && strings.HasSuffix(request.Target, ".halt") {
		if value, ok := request.Value.(bool); !ok || !value {
			return errors.New("halt controls require value true")
		}
		return nil
	}
	if request.Target == flags.FlagMaxAutonomy {
		if value, ok := request.Value.(string); ok && (value == "observe" || value == "recommend" || value == "draft" || value == "act") {
			return nil
		}
		return errors.New("maximum autonomy must be observe, recommend, draft, or act")
	}
	if request.Target == flags.FlagModelRoute {
		if value, ok := request.Value.(string); ok && value != "" {
			return nil
		}
		return errors.New("model route must be a non-empty string")
	}
	return fmt.Errorf("unsupported emergency control target %q", request.Target)
}

func applyMutation(config *flags.Config, mutation Mutation) error {
	if _, ok := config.Flags[mutation.Target]; !ok {
		return fmt.Errorf("target %q is not configured", mutation.Target)
	}
	config.Flags[mutation.Target] = mutation.Value
	return nil
}

func copyConfig(config flags.Config) flags.Config {
	values := make(map[string]any, len(config.Flags))
	for key, value := range config.Flags {
		values[key] = value
	}
	return flags.Config{Flags: values}
}

func mutationID(request Request, now time.Time) string {
	digest := sha256.Sum256([]byte(request.Target + "\x00" + request.Owner + "\x00" + request.Reason + "\x00" + now.UTC().Format(time.RFC3339Nano)))
	return "control-" + hex.EncodeToString(digest[:8])
}
