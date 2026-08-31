package pluginsubprocess_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/prunesh/prunesh/internal/pluginsubprocess"
)

// compliantSrc is a minimal subprocess/v1 binary: handles both rewrite and filter_output.
const compliantSrc = `package main

import (
	"encoding/json"
	"os"
)

func main() {
	var req struct {
		Operation string   ` + "`" + `json:"operation"` + "`" + `
		Args      []string ` + "`" + `json:"args"` + "`" + `
		Output    string   ` + "`" + `json:"output"` + "`" + `
		ExitCode  int      ` + "`" + `json:"exit_code"` + "`" + `
	}
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		os.Exit(1)
	}
	resp := map[string]interface{}{
		"args":    req.Args,
		"changed": false,
		"output":  req.Output,
	}
	json.NewEncoder(os.Stdout).Encode(resp)
}
`

// noOutputFieldSrc handles both operations but omits the "output" field in filter_output.
const noOutputFieldSrc = `package main

import (
	"encoding/json"
	"os"
)

func main() {
	var req struct {
		Operation string ` + "`" + `json:"operation"` + "`" + `
	}
	json.NewDecoder(os.Stdin).Decode(&req)
	if req.Operation == "filter_output" {
		os.Stdout.WriteString(` + "`" + `{"args":[],"changed":false}` + "`" + ` + "\n")
	} else {
		os.Stdout.WriteString(` + "`" + `{"args":[],"changed":false,"output":""}` + "`" + ` + "\n")
	}
}
`

// crashOnFilterSrc responds to rewrite but exits non-zero on filter_output.
const crashOnFilterSrc = `package main

import (
	"encoding/json"
	"os"
)

func main() {
	var req struct {
		Operation string ` + "`" + `json:"operation"` + "`" + `
	}
	json.NewDecoder(os.Stdin).Decode(&req)
	if req.Operation == "filter_output" {
		os.Exit(1)
	}
	os.Stdout.WriteString(` + "`" + `{"args":[],"changed":false,"output":""}` + "`" + ` + "\n")
}
`

func buildBinary(t *testing.T, src string) string {
	t.Helper()
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(srcFile, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "stub")
	out, err := exec.Command("go", "build", "-o", bin, srcFile).CombinedOutput()
	if err != nil {
		t.Fatalf("build stub: %v\n%s", err, out)
	}
	return bin
}

func TestContractCheck_compliant(t *testing.T) {
	bin := buildBinary(t, compliantSrc)
	if err := pluginsubprocess.ContractCheck(bin); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestContractCheck_missingOutputField(t *testing.T) {
	bin := buildBinary(t, noOutputFieldSrc)
	err := pluginsubprocess.ContractCheck(bin)
	if err == nil {
		t.Fatal("expected error for missing 'output' field, got nil")
	}
}

func TestContractCheck_crashOnFilterOutput(t *testing.T) {
	bin := buildBinary(t, crashOnFilterSrc)
	err := pluginsubprocess.ContractCheck(bin)
	if err == nil {
		t.Fatal("expected error when binary crashes on filter_output, got nil")
	}
}

func TestContractCheck_emptyPath(t *testing.T) {
	err := pluginsubprocess.ContractCheck("")
	if err == nil {
		t.Fatal("expected error for empty binary path")
	}
}
