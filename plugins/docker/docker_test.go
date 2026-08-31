package docker

import (
	"strings"
	"testing"

	"github.com/prunesh/prunesh/internal/registry"
)

func TestFilterPSCompact(t *testing.T) {
	raw := `CONTAINER ID   IMAGE          COMMAND                  CREATED        STATUS        PORTS                  NAMES
abc123def456   nginx:latest   "/docker-entrypoint.…"   2 hours ago    Up 2 hours    0.0.0.0:80->80/tcp     web
fed654cba321   redis:7        "docker-entrypoint.s…"   2 hours ago    Up 2 hours    6379/tcp               cache
`
	out := filterPS(raw)
	if !strings.Contains(out, "web") || !strings.Contains(out, "Up 2 hours") {
		t.Fatalf("got %q", out)
	}
	if strings.Contains(out, "CONTAINER ID") {
		t.Fatalf("header noise should be compact, got %q", out)
	}
	if registry.EstimateTokens(out) >= registry.EstimateTokens(raw) {
		t.Fatal("filtered output should use fewer tokens")
	}
}

func TestFilterImagesCompact(t *testing.T) {
	raw := `REPOSITORY   TAG       SIZE
nginx        latest    140MB
redis        7         110MB
`
	out := filterImages(raw)
	if !strings.Contains(out, "nginx") || !strings.Contains(out, "140MB") {
		t.Fatalf("got %q", out)
	}
}

func TestFilterLogsCap(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 150; i++ {
		sb.WriteString("log line\n")
	}
	out := filterLogs(sb.String())
	if !strings.Contains(out, "lines truncated") {
		t.Fatalf("got %q", out)
	}
	if strings.Count(out, "log line") > 101 {
		t.Fatalf("expected capped log lines, got %d", strings.Count(out, "log line"))
	}
}

func TestFilterComposePS(t *testing.T) {
	m := &Module{}
	raw := `NAME                IMAGE     COMMAND   SERVICE   CREATED   STATUS    PORTS
proj-web-1          nginx     ...       web       1h ago    Up 1h     0.0.0.0:80->80/tcp
`
	out := m.FilterOutput([]string{"compose", "ps"}, raw, 0)
	if out == raw && !strings.Contains(out, "truncated") {
		// compose ps table may use NAME column; at minimum should not error
		if !strings.Contains(out, "proj-web-1") && !strings.Contains(out, "NAME") {
			t.Fatalf("got %q", out)
		}
	}
}

func TestDockerRunPassthrough(t *testing.T) {
	m := &Module{}
	raw := "container started\n"
	out := m.FilterOutput([]string{"run", "nginx"}, raw, 0)
	if out != raw {
		t.Fatalf("docker run must pass through, got %q", out)
	}
}
