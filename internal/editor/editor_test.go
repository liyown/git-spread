package editor

import "testing"

func TestCommandForExplicitEditor(t *testing.T) {
	cmd, args, err := CommandWithLookup("code", "/tmp/workspace", func(name string) (string, error) {
		return "/bin/" + name, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "/bin/code" || len(args) != 1 || args[0] != "/tmp/workspace" {
		t.Fatalf("cmd=%q args=%v", cmd, args)
	}
}

func TestCommandRejectsUnknownExplicitEditor(t *testing.T) {
	_, _, err := CommandWithLookup("unknown-editor", "/tmp/workspace", func(name string) (string, error) {
		return "/bin/" + name, nil
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAutoUsesFirstAvailableEditor(t *testing.T) {
	cmd, args, err := CommandWithLookup("auto", "/tmp/workspace", func(name string) (string, error) {
		if name == "idea" {
			return "/bin/idea", nil
		}
		return "", ErrNotFound
	})
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "/bin/idea" || args[0] != "/tmp/workspace" {
		t.Fatalf("cmd=%q args=%v", cmd, args)
	}
}
