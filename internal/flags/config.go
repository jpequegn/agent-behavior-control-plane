package flags

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

const (
	FlagGlobalHalt         = "agent.global_halt"
	FlagDisableMemoryWrite = "agent.disable_memory_write"
	FlagDisableSubagents   = "agent.disable_subagents"
	FlagMaxAutonomy        = "agent.max_autonomy"
	FlagModelRoute         = "agent.model_route"
)

func ToolHaltFlag(tool string) string {
	return "tool." + tool + ".halt"
}

type Config struct {
	Flags map[string]any
}

func (c Config) Validate() error {
	if len(c.Flags) == 0 {
		return errors.New("flag configuration cannot be empty")
	}
	for key, value := range c.Flags {
		if key == "" {
			return errors.New("flag key cannot be empty")
		}
		switch value.(type) {
		case bool, string:
		default:
			return fmt.Errorf("flag %q has unsupported value type %T", key, value)
		}
	}
	return nil
}

func (c Config) Snapshot() string {
	keys := make([]string, 0, len(c.Flags))
	for key := range c.Flags {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make([][2]any, 0, len(keys))
	for _, key := range keys {
		ordered = append(ordered, [2]any{key, c.Flags[key]})
	}
	encoded, _ := json.Marshal(ordered)
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:])
}

type flagdDocument struct {
	Flags  map[string]flagdFlag `json:"flags"`
	Schema string               `json:"$schema"`
}

type flagdFlag struct {
	DefaultVariant string                     `json:"defaultVariant"`
	State          string                     `json:"state"`
	Variants       map[string]json.RawMessage `json:"variants"`
}

func ParseFlagdJSON(input []byte) (Config, error) {
	var document flagdDocument
	if err := json.Unmarshal(input, &document); err != nil {
		return Config{}, fmt.Errorf("parse flagd configuration: %w", err)
	}
	if document.Schema == "" || len(document.Flags) == 0 {
		return Config{}, errors.New("flagd schema and flags are required")
	}
	config := Config{Flags: make(map[string]any, len(document.Flags))}
	for key, definition := range document.Flags {
		if definition.State != "ENABLED" {
			return Config{}, fmt.Errorf("flag %q is not enabled", key)
		}
		raw, ok := definition.Variants[definition.DefaultVariant]
		if !ok {
			return Config{}, fmt.Errorf("flag %q has no default variant", key)
		}
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return Config{}, fmt.Errorf("decode flag %q: %w", key, err)
		}
		config.Flags[key] = value
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}
