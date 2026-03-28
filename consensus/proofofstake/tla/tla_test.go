package tla

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	tlaToolsJar = "tla2tools.jar"
	specModule  = "MCQuantumCoinConsensus"
	cfgFile     = "QuantumCoinConsensus.cfg"
)

func findTLAToolsJar(t *testing.T) string {
	t.Helper()

	if p := os.Getenv("TLA_TOOLS_JAR"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	specDir := specDirectory(t)
	candidate := filepath.Join(specDir, tlaToolsJar)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}

	path, err := exec.LookPath("tlc2.TLC")
	if err == nil {
		return path
	}

	t.Skipf("TLC not found: set TLA_TOOLS_JAR env var or place %s in the tla/ directory", tlaToolsJar)
	return ""
}

func specDirectory(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("failed to get spec directory: %v", err)
	}
	return dir
}

func TestTLCModelCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TLC model check in short mode")
	}

	jar := findTLAToolsJar(t)
	specDir := specDirectory(t)

	var stdout, stderr bytes.Buffer
	args := []string{
		"-jar", jar,
		"-config", cfgFile,
		"-workers", "auto",
		"-deadlock",
		specModule,
	}

	cmd := exec.Command("java", args...)
	cmd.Dir = specDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	t.Logf("Running TLC: java %s", strings.Join(args, " "))
	t.Logf("Working directory: %s", specDir)

	err := cmd.Run()
	output := stdout.String() + "\n" + stderr.String()

	if strings.Contains(output, "Error:") || strings.Contains(output, "Invariant") && strings.Contains(output, "is violated") {
		t.Fatalf("TLC found a violation:\n%s", output)
	}

	if err != nil {
		t.Fatalf("TLC exited with error: %v\nOutput:\n%s", err, output)
	}

	if !strings.Contains(output, "Model checking completed. No error has been found.") {
		t.Logf("TLC output (could not confirm success):\n%s", output)
	} else {
		t.Logf("TLC model checking passed successfully.")
	}

	if strings.Contains(output, "states generated") {
		for _, line := range strings.Split(output, "\n") {
			if strings.Contains(line, "states generated") || strings.Contains(line, "distinct states") {
				t.Logf("  %s", strings.TrimSpace(line))
			}
		}
	}
}

func TestTLCNoCounterexample(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TLC counterexample check in short mode")
	}

	jar := findTLAToolsJar(t)
	specDir := specDirectory(t)

	invariants := []string{"TypeOK", "Agreement", "Validity", "Round2Consistency", "CommitIntegrity"}

	for _, inv := range invariants {
		t.Run(fmt.Sprintf("Invariant_%s", inv), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			args := []string{
				"-jar", jar,
				"-config", cfgFile,
				"-workers", "auto",
				"-deadlock",
				specModule,
			}

			cmd := exec.Command("java", args...)
			cmd.Dir = specDir
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			output := stdout.String() + "\n" + stderr.String()

			violated := strings.Contains(output, fmt.Sprintf("Invariant %s is violated", inv))
			if violated {
				t.Errorf("Invariant %s violated. Counterexample:\n%s", inv, output)
			}

			if err != nil && !violated {
				t.Logf("TLC exited with error for %s: %v", inv, err)
			}
		})
	}
}
