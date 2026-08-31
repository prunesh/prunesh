package updatecheck

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"
)

type NotifyOptions struct {
	CurrentVersion string
	Args           []string
	HomeDir        string
	Endpoint       string
	Stderr         *os.File
	Now            func() time.Time
}

func MaybeNotify(ctx context.Context, opts NotifyOptions) {
	if !ShouldCheck(opts.CurrentVersion, opts.Args, opts.Stderr, os.Getenv) {
		return
	}
	result, err := Check(ctx, Options{CurrentVersion: opts.CurrentVersion, HomeDir: opts.HomeDir, Endpoint: opts.Endpoint, Now: opts.Now})
	if err != nil || !result.UpdateAvailable {
		return
	}
	WriteNotice(opts.Stderr, opts.CurrentVersion, result)
}

func WriteNotice(w io.Writer, currentVersion string, result Result) {
	url := result.URL
	if url == "" {
		url = "https://github.com/prunesh/prunesh/releases/latest"
	}
	_, _ = fmt.Fprintf(w, "[prunesh] update available: v%s is installed, v%s is available.\n", Normalize(currentVersion), result.LatestVersion)
	_, _ = fmt.Fprintln(w, "[prunesh] run 'prunesh update' to review, confirm, download, and install the update.")
	_, _ = fmt.Fprintf(w, "[prunesh] release notes: %s\n", url)
}

// ShouldPrompt returns true when the binary is running interactively and an
// update check + prompt is appropriate.
func ShouldPrompt(currentVersion string, args []string, stdin *os.File, stderr *os.File, getenv func(string) string) bool {
	if !ShouldCheck(currentVersion, args, stderr, getenv) {
		return false
	}
	if stdin == nil || !isTerminal(stdin) {
		return false
	}
	return true
}

// ShouldCheck returns true when a background update check is appropriate.
// It returns false when called from a hook, MCP, pipe, or version/update commands.
func ShouldCheck(currentVersion string, args []string, stderr *os.File, getenv func(string) string) bool {
	if Normalize(currentVersion) == "" || Normalize(currentVersion) == "dev" {
		return false
	}
	if getenv("PRUNESH_NO_UPDATE_CHECK") != "" || getenv("PRUNESH_DISABLE_UPDATE_CHECK") != "" {
		return false
	}
	// Skip when invoked as a hook by an agent — these are never interactive.
	if getenv("PRUNESH_SOURCE") == "hook" {
		return false
	}
	if len(args) == 0 {
		return false
	}
	if commandSkipsUpdateCheck(args) {
		return false
	}
	if stderr == nil || !isTerminal(stderr) {
		return false
	}
	return true
}

func commandSkipsUpdateCheck(args []string) bool {
	switch args[0] {
	case "hook-pre", "hook-post", "json-merge", "update", "version", "--version", "-v", "help", "--help", "-h":
		return true
	}
	for _, arg := range args[1:] {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

func isTerminal(file *os.File) bool {
	stat, err := file.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}

var _ io.Writer = (*os.File)(nil)
