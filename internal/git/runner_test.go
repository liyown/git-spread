package git

import "testing"

func TestRunnerExecutesGitVersion(t *testing.T) {
	r := NewRunner("")
	out, err := r.Output("version")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("expected git version output")
	}
}
