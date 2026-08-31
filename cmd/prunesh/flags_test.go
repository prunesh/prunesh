package main

import "testing"

func TestParseAgentFlag(t *testing.T) {
	got, err := parseAgentFlag([]string{"--agent=cursor"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "cursor" {
		t.Fatalf("got %q", got)
	}

	got, err = parseAgentFlag([]string{"--agent", "codex"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "codex" {
		t.Fatalf("got %q", got)
	}
}

func TestParseAgentFlagRequired(t *testing.T) {
	if _, err := parseAgentFlag(nil); err == nil {
		t.Fatal("missing --agent must fail")
	}
	if _, err := parseAgentFlag([]string{"--agent"}); err == nil {
		t.Fatal("bare --agent must fail")
	}
	if _, err := parseAgentFlag([]string{"--agent="}); err == nil {
		t.Fatal("empty --agent value must fail")
	}
	if _, err := parseAgentFlag([]string{"--agent=windsurf"}); err == nil {
		t.Fatal("unknown agent must fail")
	}
	if _, err := parseAgentFlag([]string{"--agent=cursor", "--extra"}); err == nil {
		t.Fatal("unknown extra argument must fail")
	}
}
