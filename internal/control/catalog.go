package control

type ToolDefinition struct {
	Name      string
	Operation Operation
	Risk      Risk
}

type ToolCatalog struct {
	Tools []ToolDefinition
}

func DefaultIncidentToolCatalog() ToolCatalog {
	return ToolCatalog{Tools: []ToolDefinition{
		{Name: "metrics.read", Operation: OperationRead, Risk: RiskLow},
		{Name: "ticket.update", Operation: OperationWrite, Risk: RiskMedium},
		{Name: "service.restart", Operation: OperationWrite, Risk: RiskHigh},
		{Name: "subagent.spawn", Operation: OperationSpawn, Risk: RiskMedium},
	}}
}

func (c ToolCatalog) Find(name string) (ToolDefinition, bool) {
	for _, tool := range c.Tools {
		if tool.Name == name {
			return tool, true
		}
	}
	return ToolDefinition{}, false
}

func (c ToolCatalog) FailModeFor(name string, environment Environment) FailMode {
	tool, ok := c.Find(name)
	if !ok {
		return FailClosed
	}
	return DefaultFailModeMatrix().ModeFor(environment, tool.Risk)
}

type FailModeMatrix struct {
	Modes map[Environment]map[Risk]FailMode
}

func DefaultFailModeMatrix() FailModeMatrix {
	return FailModeMatrix{Modes: map[Environment]map[Risk]FailMode{
		EnvironmentDevelopment: {
			RiskLow: FailOpen, RiskMedium: FailClosed, RiskHigh: FailClosed,
		},
		EnvironmentProduction: {
			RiskLow: FailOpen, RiskMedium: FailClosed, RiskHigh: FailClosed,
		},
	}}
}

func (m FailModeMatrix) ModeFor(environment Environment, risk Risk) FailMode {
	if modes, ok := m.Modes[environment]; ok {
		if mode, ok := modes[risk]; ok {
			return mode
		}
	}
	return FailClosed
}
