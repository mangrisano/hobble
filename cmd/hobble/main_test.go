package main

import "testing"

func TestRootCmdFlagShorthands(t *testing.T) {
	cmd := newRootCmd()
	want := map[string]string{
		"target": "t",
		"listen": "l",
	}
	for name, short := range want {
		fl := cmd.Flags().Lookup(name)
		if fl == nil {
			t.Errorf("flag --%s not registered", name)
			continue
		}
		if fl.Shorthand != short {
			t.Errorf("--%s shorthand = %q, want %q", name, fl.Shorthand, short)
		}
	}
	for _, name := range []string{"latency", "status", "drop"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not registered", name)
		}
	}
}

func TestRootCmdRequiresTarget(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute() with no --target error = nil, want error")
	}
}

func TestRootCmdInvalidTarget(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"-t", "not-a-target"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute() with invalid --target error = nil, want error")
	}
}

func TestRootCmdInvalidLatency(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"-t", "http://example.com", "--latency", "not-a-duration"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute() with invalid --latency error = nil, want error")
	}
}

func TestRootCmdInvalidStatus(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"-t", "http://example.com", "--status", "not-a-rule"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute() with invalid --status error = nil, want error")
	}
}
