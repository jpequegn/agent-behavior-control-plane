package control

import "fmt"

type ToolControl struct {
	Enabled   bool `json:"enabled"`
	WriteMode bool `json:"write_mode"`
}

type MemoryPolicy struct {
	ReadEnabled  bool `json:"read_enabled"`
	WriteEnabled bool `json:"write_enabled"`
}

type SubagentPolicy struct {
	Enabled  bool `json:"enabled"`
	MaxCount int  `json:"max_count"`
}

type KillSwitches struct {
	GlobalHalt          bool     `json:"global_halt"`
	HaltedTools         []string `json:"halted_tools"`
	MemoryWriteDisabled bool     `json:"memory_write_disabled"`
	SubagentsDisabled   bool     `json:"subagents_disabled"`
}

type BehaviorConfig struct {
	Autonomy      AutonomyTier           `json:"autonomy"`
	KillSwitches  KillSwitches           `json:"kill_switches"`
	Memory        MemoryPolicy           `json:"memory"`
	ModelRoute    string                 `json:"model_route"`
	PromptVersion string                 `json:"prompt_version"`
	Subagents     SubagentPolicy         `json:"subagents"`
	Tools         map[string]ToolControl `json:"tools"`
	Version       string                 `json:"version"`
}

func DefaultBehaviorConfig() BehaviorConfig {
	return BehaviorConfig{
		Autonomy:      AutonomyRecommend,
		Memory:        MemoryPolicy{ReadEnabled: true, WriteEnabled: true},
		ModelRoute:    "incident-model-v1",
		PromptVersion: "incident-prompt-v1",
		Subagents:     SubagentPolicy{Enabled: true, MaxCount: 2},
		Tools: map[string]ToolControl{
			"metrics.read":    {Enabled: true, WriteMode: false},
			"service.restart": {Enabled: true, WriteMode: true},
			"ticket.update":   {Enabled: true, WriteMode: true},
		},
		Version: "behavior-v1",
	}
}

func (c BehaviorConfig) Validate() error {
	if c.Version == "" || c.PromptVersion == "" || c.ModelRoute == "" {
		return fmt.Errorf("behavior version, prompt version, and model route are required")
	}
	if !c.Autonomy.Valid() {
		return fmt.Errorf("invalid autonomy tier %q", c.Autonomy)
	}
	if c.Subagents.MaxCount < 0 {
		return fmt.Errorf("subagent count cannot be negative")
	}
	return nil
}
