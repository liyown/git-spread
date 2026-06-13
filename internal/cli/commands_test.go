package cli

import (
	"bytes"
	"testing"
)

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"--version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if stdout.String() != "git-spread dev\n" {
		t.Fatalf("stdout = %q, want version line", stdout.String())
	}
}
