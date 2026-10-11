package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Only the browser box gets the state key, so the agent's shell box always meets this refusal.
// On 2026-10-11 its old text blamed AURA_AUTHULA_SECRET, and the agent reported the browser as
// broken: the refusal has to send it to the browser tools instead.
func TestSandboxAgentBrowserRefusalPointsAtTheBrowserTools(t *testing.T) {
	if _, err := os.Stat("/run/aura/agent-browser.key"); err == nil {
		t.Skip("this host holds a browser state key, so the refusal cannot run here")
	}
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Fatalf("the entry point is a POSIX sh script: %v", err)
	}
	script := filepath.Join(repoRootForTest(t), "docker", "aura-sandbox", "agent-browser.sh")
	var stderr bytes.Buffer
	cmd := exec.Command(sh, script, "--version")
	cmd.Stderr = &stderr

	var exit *exec.ExitError
	if err := cmd.Run(); !errors.As(err, &exit) || exit.ExitCode() != 78 {
		t.Fatalf("agent-browser without a key = %v, want exit 78; stderr: %s", err, stderr.String())
	}
	for _, want := range []string{"browser__agent_browser_", "box of its own", "AURA_AUTHULA_SECRET"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("refusal lacks %q:\n%s", want, stderr.String())
		}
	}
}
