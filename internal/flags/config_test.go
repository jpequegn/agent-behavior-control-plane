package flags

import "testing"

func TestSnapshotIsDeterministic(t *testing.T) {
	first := Config{Flags: map[string]any{
		FlagGlobalHalt:  false,
		FlagMaxAutonomy: "act",
	}}
	second := Config{Flags: map[string]any{
		FlagMaxAutonomy: "act",
		FlagGlobalHalt:  false,
	}}
	if got, want := first.Snapshot(), second.Snapshot(); got != want {
		t.Fatalf("snapshot = %q, want %q", got, want)
	}
}
