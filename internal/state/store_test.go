package state

import "testing"

func TestStoreRoundTrip(t *testing.T) {
	store := NewStore(t.TempDir())
	run := Run{
		ID:   "run-1",
		Kind: "branch",
		Targets: []Target{
			{Branch: "release/1.0", Status: StatusDone},
			{Branch: "release/1.1", Status: StatusConflict, WorkspacePath: ".spread/release-1.1", ConflictedFiles: []string{"user.go"}},
		},
	}

	if err := store.Save(run); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "run-1" || got.Targets[1].ConflictedFiles[0] != "user.go" {
		t.Fatalf("loaded run = %#v", got)
	}
}
