// Package proxy runs a registered module against a real command.
package proxy

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/prunesh/prunesh/internal/pluginregistry"
	"github.com/prunesh/prunesh/internal/pluginsubprocess"
	"github.com/prunesh/prunesh/internal/registry"
	"github.com/prunesh/prunesh/internal/text"
	"github.com/prunesh/prunesh/plugins/gain"
)

// Run executes name with args, filters stdout, records gain, and returns the child exit code.
func Run(name string, args []string) int {
	mod := resolveModule(name)
	if mod == nil {
		fmt.Fprintf(os.Stderr, "prunesh: unknown command %q\n", name)
		return 1
	}

	execArgs := args
	rewritten, didRewrite := mod.Rewrite(args)
	if didRewrite {
		execArgs = rewritten
	}

	env := extraEnv(mod, execArgs)

	start := time.Now()

	var rawOut, execOut, execErr string
	var execCode int
	var execFail error

	if didRewrite {
		rawOut, _, _, _ = runCmd(name, args, env)
		execOut, execErr, execCode, execFail = runCmd(name, execArgs, env)
	} else {
		execOut, execErr, execCode, execFail = runCmd(name, execArgs, env)
		rawOut = execOut
	}

	elapsed := time.Since(start)

	stripped := text.StripANSI(execOut)
	filtered := mod.FilterOutput(execArgs, stripped, execCode)
	shown := registry.NeverWorse(rawOut, filtered)

	if _, werr := os.Stdout.WriteString(shown); werr != nil {
		fmt.Fprintf(os.Stderr, "prunesh: write stdout: %v\n", werr)
	}
	if execErr != "" {
		_, _ = os.Stderr.WriteString(execErr)
	}

	label := name
	if len(args) > 0 {
		label = name + " " + strings.Join(args, " ")
	}
	recordGain(label, rawOut, shown, elapsed)

	if execFail != nil {
		fmt.Fprintf(os.Stderr, "prunesh: %v\n", execFail)
		return 1
	}
	return execCode
}

func resolveModule(name string) registry.Module {
	db, err := pluginregistry.Open()
	if err != nil {
		return registry.Get(name)
	}
	defer db.Close()
	rec, err := db.Active(name)
	if err != nil || rec == nil {
		return registry.Get(name)
	}
	return pluginsubprocess.NewModule(name, rec.BinaryPath)
}

func extraEnv(mod registry.Module, args []string) []string {
	inj, ok := mod.(registry.EnvInjector)
	if !ok {
		return nil
	}
	return inj.ExtraEnv(args)
}

func runCmd(name string, args []string, env []string) (stdout, stderr string, code int, err error) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	code = 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
			err = nil
		}
	}
	return outBuf.String(), errBuf.String(), code, err
}

func recordGain(cmd, raw, shown string, elapsed time.Duration) {
	t, err := gain.Open()
	if err != nil {
		return
	}
	defer t.Close()
	_ = t.Record(cmd, registry.EstimateTokens(raw), registry.EstimateTokens(shown), elapsed)
}
