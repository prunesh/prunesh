package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/prunesh/prunesh/internal/updatecheck"
)

const installScriptURL = "https://raw.githubusercontent.com/prunesh/prunesh/main/install.sh"

type updateRuntime struct {
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
	install func(context.Context, string, io.Writer, io.Writer) error
	check   func(context.Context, updatecheck.Options) (updatecheck.Result, error)
}

func defaultUpdateRuntime() updateRuntime {
	return updateRuntime{
		stdin:   os.Stdin,
		stdout:  os.Stdout,
		stderr:  os.Stderr,
		install: installPruneshRelease,
		check:   updatecheck.Check,
	}
}

func runUpdate() {
	opts, err := parseUpdateArgs(os.Args[2:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "prunesh update: %v\n", err)
		os.Exit(1)
	}
	rt := defaultUpdateRuntime()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := rt.check(ctx, updatecheck.Options{CurrentVersion: version, Force: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "prunesh update: %v\n", err)
		os.Exit(1)
	}
	if !result.UpdateAvailable {
		fmt.Fprintf(rt.stdout, "prunesh update: v%s is already current.\n", updatecheck.Normalize(version))
		return
	}
	updatecheck.WriteNotice(rt.stderr, version, result)
	if opts.checkOnly {
		return
	}
	if !opts.assumeYes {
		approved, err := promptForUpdate(rt.stdin, rt.stderr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "prunesh update: %v\n", err)
			os.Exit(1)
		}
		if !approved {
			fmt.Fprintln(rt.stderr, "[prunesh] update skipped.")
			return
		}
	}
	if err := rt.install(ctx, result.LatestVersion, rt.stdout, rt.stderr); err != nil {
		fmt.Fprintf(os.Stderr, "prunesh update: install failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(rt.stderr, "[prunesh] update installed: v%s\n", result.LatestVersion)
}

type updateOpts struct {
	assumeYes bool
	checkOnly bool
}

func parseUpdateArgs(args []string) (updateOpts, error) {
	var opts updateOpts
	for _, arg := range args {
		switch arg {
		case "--yes", "-y":
			opts.assumeYes = true
		case "--check":
			opts.checkOnly = true
		default:
			return opts, fmt.Errorf("unknown argument %q", arg)
		}
	}
	return opts, nil
}

func maybeOfferUpdate(ctx context.Context, rt updateRuntime) {
	result, err := rt.check(ctx, updatecheck.Options{CurrentVersion: version})
	if err != nil || !result.UpdateAvailable {
		return
	}
	updatecheck.WriteNotice(rt.stderr, version, result)
	approved, err := promptForUpdate(rt.stdin, rt.stderr)
	if err != nil || !approved {
		if err == nil {
			_, _ = fmt.Fprintln(rt.stderr, "[prunesh] update skipped.")
		}
		return
	}
	installCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := rt.install(installCtx, result.LatestVersion, rt.stdout, rt.stderr); err != nil {
		_, _ = fmt.Fprintf(rt.stderr, "[prunesh] update failed: %v\n", err)
		return
	}
	_, _ = fmt.Fprintf(rt.stderr, "[prunesh] update installed: v%s\n", result.LatestVersion)
}

func promptForUpdate(stdin io.Reader, stderr io.Writer) (bool, error) {
	_, _ = fmt.Fprint(stderr, "[prunesh] Update now? [y/N] ")
	answer, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}

func installPruneshRelease(ctx context.Context, latestVersion string, stdout io.Writer, stderr io.Writer) error {
	tag := "v" + updatecheck.Normalize(latestVersion)
	if tag == "v" {
		return errors.New("latest version is empty")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, installScriptURL, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download installer: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download installer returned %s", resp.Status)
	}
	tmpDir, err := os.MkdirTemp("", "prunesh-update-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	scriptPath := filepath.Join(tmpDir, "install.sh")
	script, err := os.OpenFile(scriptPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		return err
	}
	if _, err := io.Copy(script, io.LimitReader(resp.Body, 1024*1024)); err != nil {
		_ = script.Close()
		return err
	}
	if err := script.Close(); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "bash", scriptPath)
	cmd.Env = append(os.Environ(), "PRUNESH_VERSION="+tag)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
