package main

import (
	"bytes"
	"testing"
)

func TestRootCommandHelp(t *testing.T) {
	t.Parallel()
	command := newRootCommand()
	output := &bytes.Buffer{}
	command.SetOut(output)
	command.SetArgs([]string{"--help"})

	if err := command.Execute(); err != nil {
		t.Fatalf("execute help: %v", err)
	}
	if !bytes.Contains(output.Bytes(), []byte("serve")) {
		t.Fatalf("help did not include serve command: %s", output.String())
	}
}
