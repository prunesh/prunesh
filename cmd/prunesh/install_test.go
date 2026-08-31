// Tests de instalación.
// Ejercitan install.sh con HOME y PRUNESH_INSTALL_DIR apuntando a directorios
// temporales, verificando que la detección de agentes, el flujo dry-run,
// y la configuración de cada agente produzcan los archivos y entradas correctas.
package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// installScript devuelve la ruta absoluta a install.sh.
func installScript(t *testing.T) string {
	t.Helper()
	return filepath.Join(moduleRoot(t), "install.sh")
}

// runInstall ejecuta install.sh con el entorno dado y devuelve stdout+stderr y el exit code.
func runInstall(t *testing.T, env []string, args ...string) (string, int) {
	t.Helper()
	sh := installScript(t)
	cmd := exec.Command("sh", append([]string{sh}, args...)...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("exec install.sh: %v", err)
		}
	}
	return string(out), code
}

// baseEnv construye un entorno mínimo para install.sh que:
//   - apunta HOME a un directorio temporal
//   - apunta PRUNESH_INSTALL_DIR a un directorio temporal con el binario ya compilado
//   - usa PRUNESH_SKIP_BINARY=1 para no intentar descargar nada de la red
//   - usa PRUNESH_SCRIPTS_DIR apuntando a la raíz del repo local
func baseEnv(t *testing.T, home, installDir string) []string {
	t.Helper()
	bin := buildBinary(t)
	// copiar el binario compilado al directorio de instalación simulado
	dst := filepath.Join(installDir, "prunesh")
	data, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, data, 0o755); err != nil {
		t.Fatal(err)
	}
	return []string{
		"HOME=" + home,
		"PRUNESH_INSTALL_DIR=" + installDir,
		"PRUNESH_SKIP_BINARY=1",
		"PRUNESH_SKIP_FILTERS=1",
		"PRUNESH_SCRIPTS_DIR=" + moduleRoot(t),
		"PATH=" + installDir + ":" + os.Getenv("PATH"),
		"SHELL=/bin/sh",
	}
}

func TestInstallDryRun(t *testing.T) {
	home := t.TempDir()
	installDir := t.TempDir()
	env := append(baseEnv(t, home, installDir), "PRUNESH_DRY_RUN=true", "PRUNESH_AGENT=claudecode")

	out, code := runInstall(t, env)
	if code != 0 {
		t.Fatalf("dry-run exit %d:\n%s", code, out)
	}
	if !strings.Contains(out, "dry-run") {
		t.Fatalf("expected dry-run message in output:\n%s", out)
	}
	// en dry-run no deben crearse archivos de configuración del agente
	if _, err := os.Stat(filepath.Join(home, ".claude", "settings.json")); err == nil {
		t.Fatal("dry-run must not write agent config files")
	}
}

func TestInstallClaudeCode(t *testing.T) {
	home := t.TempDir()
	installDir := t.TempDir()
	env := baseEnv(t, home, installDir)

	out, code := runInstall(t, env, "--agent=claudecode")
	if code != 0 {
		t.Fatalf("claudecode install exit %d:\n%s", code, out)
	}
	// skill must be installed in the global store
	skillPath := filepath.Join(home, ".agents", "skills", "prunesh", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("SKILL.md not in global store: %v", err)
	}
	// ~/.claude/skills/prunesh must be a symlink pointing to the global store
	linkPath := filepath.Join(home, ".claude", "skills", "prunesh")
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("claude skill symlink not created: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("~/.claude/skills/prunesh is not a symlink")
	}
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	expected := filepath.Join(home, ".agents", "skills", "prunesh")
	if target != expected {
		t.Fatalf("symlink target: got %q, want %q", target, expected)
	}
}

func TestInstallCursor(t *testing.T) {
	home := t.TempDir()
	installDir := t.TempDir()
	env := baseEnv(t, home, installDir)

	out, code := runInstall(t, env, "--agent=cursor")
	if code != 0 {
		t.Fatalf("cursor install exit %d:\n%s", code, out)
	}
	// scripts de hooks deben estar instalados
	hooksDir := filepath.Join(home, ".cursor", "hooks")
	for _, script := range []string{"prunesh-pre-tool-use.sh", "prunesh-post-tool-use.sh"} {
		p := filepath.Join(hooksDir, script)
		info, err := os.Stat(p)
		if err != nil {
			t.Fatalf("cursor hook %s not installed: %v", script, err)
		}
		if info.Mode()&0o111 == 0 {
			t.Fatalf("cursor hook %s is not executable", script)
		}
	}
	// hooks.json debe contener la entrada preToolUse
	hooksJSON, err := os.ReadFile(filepath.Join(home, ".cursor", "hooks.json"))
	if err != nil {
		t.Fatalf("hooks.json not created: %v", err)
	}
	if !strings.Contains(string(hooksJSON), "prunesh-pre-tool-use.sh") {
		t.Fatalf("hooks.json does not register preToolUse hook:\n%s", hooksJSON)
	}
	// regla de contexto
	if _, err := os.Stat(filepath.Join(home, ".cursor", "rules", "prunesh.mdc")); err != nil {
		t.Fatalf("prunesh.mdc rule not installed: %v", err)
	}
	// skill symlink
	linkPath := filepath.Join(home, ".cursor", "skills", "prunesh")
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("cursor skill symlink not created: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("~/.cursor/skills/prunesh is not a symlink")
	}
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("readlink cursor skill: %v", err)
	}
	if target != filepath.Join(home, ".agents", "skills", "prunesh") {
		t.Fatalf("cursor skill symlink target: got %q", target)
	}
}

func TestInstallCodex(t *testing.T) {
	home := t.TempDir()
	installDir := t.TempDir()
	env := baseEnv(t, home, installDir)

	out, code := runInstall(t, env, "--agent=codex")
	if code != 0 {
		t.Fatalf("codex install exit %d:\n%s", code, out)
	}
	// hook pre-tool-use debe estar instalado
	hookPath := filepath.Join(home, ".codex", "hooks", "prunesh-pre-tool-use.sh")
	info, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("codex hook not installed: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatal("codex hook is not executable")
	}
	// hooks.json debe contener PreToolUse
	hooksJSON, err := os.ReadFile(filepath.Join(home, ".codex", "hooks.json"))
	if err != nil {
		t.Fatalf("hooks.json not created: %v", err)
	}
	if !strings.Contains(string(hooksJSON), "PreToolUse") {
		t.Fatalf("hooks.json does not register PreToolUse:\n%s", hooksJSON)
	}
	// config.toml must NOT contain the legacy codex_hooks entry (Codex 0.149.1+)
	configTOML, _ := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if strings.Contains(string(configTOML), "codex_hooks") {
		t.Fatalf("config.toml must not contain legacy codex_hooks:\n%s", configTOML)
	}
	// skill symlink
	linkPath := filepath.Join(home, ".codex", "skills", "prunesh")
	info, err = os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("codex skill symlink not created: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("~/.codex/skills/prunesh is not a symlink")
	}
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("readlink codex skill: %v", err)
	}
	if target != filepath.Join(home, ".agents", "skills", "prunesh") {
		t.Fatalf("codex skill symlink target: got %q", target)
	}
}

func TestInstallOpenCode(t *testing.T) {
	home := t.TempDir()
	installDir := t.TempDir()
	env := baseEnv(t, home, installDir)

	out, code := runInstall(t, env, "--agent=opencode")
	if code != 0 {
		t.Fatalf("opencode install exit %d:\n%s", code, out)
	}
	// plugin ts debe estar instalado
	pluginPath := filepath.Join(home, ".config", "opencode", "plugins", "prunesh.ts")
	if _, err := os.Stat(pluginPath); err != nil {
		t.Fatalf("opencode plugin not installed: %v", err)
	}
}

func TestInstallUnknownAgentFails(t *testing.T) {
	home := t.TempDir()
	installDir := t.TempDir()
	env := baseEnv(t, home, installDir)

	_, code := runInstall(t, env, "--agent=unknown")
	if code == 0 {
		t.Fatal("install with unknown agent must exit non-zero")
	}
}

// TestInstallIdempotentClaudeCode verifica que ejecutar install.sh dos veces
// no falla y el symlink queda apuntando al target correcto.
func TestInstallIdempotentClaudeCode(t *testing.T) {
	home := t.TempDir()
	installDir := t.TempDir()
	env := baseEnv(t, home, installDir)

	for i := 0; i < 2; i++ {
		out, code := runInstall(t, env, "--agent=claudecode")
		if code != 0 {
			t.Fatalf("run %d exit %d:\n%s", i+1, code, out)
		}
	}
	linkPath := filepath.Join(home, ".claude", "skills", "prunesh")
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("readlink after 2 runs: %v", err)
	}
	expected := filepath.Join(home, ".agents", "skills", "prunesh")
	if target != expected {
		t.Fatalf("symlink target after 2 runs: got %q, want %q", target, expected)
	}
}
