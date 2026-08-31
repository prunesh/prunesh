// Package pluginsubprocess invokes external plugin binaries using subprocess/v1.
package pluginsubprocess

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/prunesh/prunesh/internal/registry"
)

const contractTimeout = 2 * time.Second

type request struct {
	Operation string   `json:"operation"`
	Args      []string `json:"args"`
	Output    string   `json:"output"`
	ExitCode  int      `json:"exit_code"`
}

type response struct {
	Args    []string `json:"args"`
	Changed bool     `json:"changed"`
	Output  string   `json:"output"`
}

// Module adapts an external filter binary to registry.Module.
type Module struct {
	binary string
	name   string
}

// NewModule returns a subprocess-backed module for argv0 name.
func NewModule(name, binaryPath string) *Module {
	return &Module{name: name, binary: binaryPath}
}

func (m *Module) Name() string { return m.name }

func (m *Module) Rewrite(args []string) ([]string, bool) {
	resp, err := m.call("rewrite", args, "", 0)
	if err != nil || !resp.Changed {
		return nil, false
	}
	return resp.Args, true
}

func (m *Module) FilterOutput(args []string, output string, exitCode int) string {
	resp, err := m.call("filter_output", args, output, exitCode)
	if err != nil {
		return output
	}
	return resp.Output
}

func (m *Module) TokensBefore(output string) int  { return registry.EstimateTokens(output) }
func (m *Module) TokensAfter(filtered string) int { return registry.EstimateTokens(filtered) }

func (m *Module) call(operation string, args []string, output string, exitCode int) (response, error) {
	req := request{Operation: operation, Args: args, Output: output, ExitCode: exitCode}
	payload, err := json.Marshal(req)
	if err != nil {
		return response{}, err
	}
	cmd := exec.Command(m.binary)
	cmd.Stdin = bytes.NewReader(payload)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return response{}, fmt.Errorf("filter binary: %w", err)
	}
	var resp response
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		return response{}, fmt.Errorf("decode filter response: %w", err)
	}
	return resp, nil
}

// ContractCheck verifies the binary implements subprocess/v1 by probing both
// rewrite and filter_output within the timeout. For filter_output it also
// checks that the response includes the required "output" field.
func ContractCheck(binaryPath string) error {
	if binaryPath == "" {
		return fmt.Errorf("binary path is empty")
	}
	if err := probeOperation(binaryPath, "rewrite", []string{"probe"}, "", 0, false); err != nil {
		return fmt.Errorf("rewrite probe: %w", err)
	}
	if err := probeOperation(binaryPath, "filter_output", []string{"probe"}, "prunesh-contract-probe", 0, true); err != nil {
		return fmt.Errorf("filter_output probe: %w", err)
	}
	return nil
}

func probeOperation(binaryPath, operation string, args []string, output string, exitCode int, requireOutputField bool) error {
	type result struct {
		raw []byte
		err error
	}
	done := make(chan result, 1)
	go func() {
		req := request{Operation: operation, Args: args, Output: output, ExitCode: exitCode}
		payload, err := json.Marshal(req)
		if err != nil {
			done <- result{err: err}
			return
		}
		cmd := exec.Command(binaryPath)
		cmd.Stdin = bytes.NewReader(payload)
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err != nil {
			done <- result{err: fmt.Errorf("binary exited with error: %w", err)}
			return
		}
		done <- result{raw: out.Bytes()}
	}()

	var res result
	select {
	case res = <-done:
	case <-time.After(contractTimeout):
		return fmt.Errorf("exceeded %s", contractTimeout)
	}
	if res.err != nil {
		return res.err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(res.raw, &fields); err != nil {
		return fmt.Errorf("invalid JSON response: %w", err)
	}
	if requireOutputField {
		if _, ok := fields["output"]; !ok {
			return fmt.Errorf("response missing required 'output' field")
		}
	}
	return nil
}

// BinaryNameFromID derives the installed binary filename from a filter id.
func BinaryNameFromID(id string) string {
	for i := len(id) - 1; i >= 0; i-- {
		if id[i] == '/' {
			return id[i+1:]
		}
	}
	return id
}
