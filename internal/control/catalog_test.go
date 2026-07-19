package control

import "testing"

func TestFailModeMatrix(t *testing.T) {
	t.Parallel()
	catalog := DefaultIncidentToolCatalog()
	tests := []struct {
		name        string
		tool        string
		environment Environment
		want        FailMode
	}{
		{name: "low risk read remains available", tool: "metrics.read", environment: EnvironmentProduction, want: FailOpen},
		{name: "reversible write fails closed", tool: "ticket.update", environment: EnvironmentDevelopment, want: FailClosed},
		{name: "consequential write fails closed", tool: "service.restart", environment: EnvironmentProduction, want: FailClosed},
		{name: "undefined tool fails closed", tool: "unknown.write", environment: EnvironmentProduction, want: FailClosed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := catalog.FailModeFor(test.tool, test.environment); got != test.want {
				t.Fatalf("mode = %q, want %q", got, test.want)
			}
		})
	}
}
