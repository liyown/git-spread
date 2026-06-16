package state

import "testing"

func TestHistoryAppendsAndListsNewestFirst(t *testing.T) {
	store := NewStore(t.TempDir())
	first := Run{ID: "run-1", Task: "release", Kind: "branch", Mode: "direct", Targets: []Target{{Branch: "main", Status: StatusDone}}}
	second := Run{ID: "run-2", Task: "backport", Kind: "commit", Mode: "pr", Targets: []Target{{Branch: "release/1.0", Status: StatusConflict}, {Branch: "release/1.1", Status: StatusPending}}}

	if err := store.AppendHistory(first); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendHistory(second); err != nil {
		t.Fatal(err)
	}

	entries, err := store.History(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	if entries[0].Run.ID != "run-2" || entries[1].Run.ID != "run-1" {
		t.Fatalf("history order = %#v", entries)
	}
	if entries[0].Summary[StatusConflict] != 1 || entries[0].Summary[StatusPending] != 1 {
		t.Fatalf("summary = %#v", entries[0].Summary)
	}
}

func TestHistoryLimit(t *testing.T) {
	store := NewStore(t.TempDir())
	for _, id := range []string{"run-1", "run-2", "run-3"} {
		if err := store.AppendHistory(Run{ID: id, Targets: []Target{{Branch: "main", Status: StatusDone}}}); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := store.History(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Run.ID != "run-3" || entries[1].Run.ID != "run-2" {
		t.Fatalf("entries = %#v", entries)
	}
}
