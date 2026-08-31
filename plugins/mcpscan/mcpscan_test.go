package mcpscan

import (
	"testing"
)

func TestGroupByPrefix(t *testing.T) {
	tests := []struct {
		name  string
		tools map[string]bool
		want  map[string]bool
	}{
		{
			name:  "standard prefixes",
			tools: map[string]bool{"hc_file_context": true, "hc_search": true, "mem_save": true, "mem_context": true},
			want:  map[string]bool{"hc_": true, "mem_": true},
		},
		{
			name:  "no underscore tools are ignored",
			tools: map[string]bool{"search": true, "read": true},
			want:  map[string]bool{},
		},
		{
			name:  "single underscore at end is ignored",
			tools: map[string]bool{"tool_": true},
			want:  map[string]bool{},
		},
		{
			name:  "mixed",
			tools: map[string]bool{"hc_status": true, "standalone": true, "my_tool_a": true, "my_tool_b": true},
			want:  map[string]bool{"hc_": true, "my_": true},
		},
		{
			name:  "empty",
			tools: map[string]bool{},
			want:  map[string]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := groupByPrefix(tt.tools)
			if len(got) != len(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
				return
			}
			for k := range tt.want {
				if !got[k] {
					t.Errorf("missing prefix %q in result %v", k, got)
				}
			}
		})
	}
}

func TestPassthroughSet(t *testing.T) {
	tests := []struct {
		env  string
		want map[string]bool
	}{
		{"hc_,mem_", map[string]bool{"hc_": true, "mem_": true}},
		{"hc_", map[string]bool{"hc_": true}},
		{"", map[string]bool{}},
		{" hc_ , mem_ ", map[string]bool{"hc_": true, "mem_": true}},
	}

	for _, tt := range tests {
		t.Setenv("PRUNESH_MCP_PASSTHROUGH_PATTERNS", tt.env)
		got := passthroughSet()
		if len(got) != len(tt.want) {
			t.Errorf("env=%q: got %v, want %v", tt.env, got, tt.want)
			continue
		}
		for k := range tt.want {
			if !got[k] {
				t.Errorf("env=%q: missing %q in %v", tt.env, k, got)
			}
		}
	}
}

func TestSortedKeys(t *testing.T) {
	m := map[string]bool{"z": true, "a": true, "m": true}
	got := sortedKeys(m)
	want := []string{"a", "m", "z"}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("index %d: got %q, want %q", i, got[i], v)
		}
	}
}
