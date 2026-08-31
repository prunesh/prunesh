// Package registry provides the central module registry.
// Each module registers itself via Register() at init time.
// The core binary never imports modules directly — modules import registry.
package registry

import (
	"fmt"
	"strings"
)

// Module is the interface every prunesh module must implement.
type Module interface {
	// Name returns the command name this module handles (e.g. "find", "ls").
	Name() string

	// Rewrite optionally rewrites the raw command before execution.
	// Returns (rewritten, true) if rewritten, ("", false) if no change needed.
	Rewrite(args []string) ([]string, bool)

	// FilterOutput applies heuristic pruning to command output before it reaches the agent.
	// args are the words after the module name (e.g. ["status", "-sb"] for git).
	// exitCode is the child exit status; pass -1 when unknown (PostToolUse).
	FilterOutput(args []string, output string, exitCode int) string

	// TokensBefore estimates tokens in the original output.
	TokensBefore(output string) int

	// TokensAfter estimates tokens in the filtered output.
	TokensAfter(filtered string) int
}

// EnvInjector is implemented by modules that need extra environment variables
// when the proxy executes the command.
type EnvInjector interface {
	ExtraEnv(args []string) []string
}

var modules = map[string]Module{}

// Register adds a module to the registry. Called from module init().
func Register(m Module) {
	name := strings.ToLower(m.Name())
	if _, exists := modules[name]; exists {
		panic(fmt.Sprintf("prunesh: module %q already registered", name))
	}
	modules[name] = m
}

// Get returns the module for a given command name, or nil if not registered.
func Get(name string) Module {
	return modules[strings.ToLower(name)]
}

// All returns all registered modules.
func All() map[string]Module {
	result := make(map[string]Module, len(modules))
	for k, v := range modules {
		result[k] = v
	}
	return result
}

// EstimateTokens estimates token count from byte length (~4 chars/token).
func EstimateTokens(s string) int {
	n := len(s) / 4
	if n == 0 && len(s) > 0 {
		return 1
	}
	return n
}

// NeverWorse returns filtered, or raw when filtered would use more estimated tokens.
func NeverWorse(raw, filtered string) string {
	if EstimateTokens(filtered) > EstimateTokens(raw) {
		return raw
	}
	return filtered
}
