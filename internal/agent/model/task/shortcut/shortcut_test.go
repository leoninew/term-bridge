package shortcut

import "testing"

func TestNormalizePreservesRawCommandAndNormalizesDescription(t *testing.T) {
	description := "  launch the local shell  "
	value := Shortcut{Name: "  Shell  ", Command: `cmd /c "echo hello" && dir`, Description: &description}

	if err := value.Normalize(); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if value.Name != "Shell" {
		t.Fatalf("Name = %q, want Shell", value.Name)
	}
	if value.Command != `cmd /c "echo hello" && dir` {
		t.Fatalf("Command = %q, want original command text", value.Command)
	}
	if value.Description == nil || *value.Description != "launch the local shell" {
		t.Fatalf("Description = %#v, want normalized description", value.Description)
	}
}

func TestNormalizeRejectsBlankRequiredFields(t *testing.T) {
	for _, value := range []Shortcut{{Command: "cmd"}, {Name: "shell", Command: " \t "}} {
		if err := value.Normalize(); err == nil {
			t.Fatalf("Normalize(%#v) error = nil, want required-field error", value)
		}
	}
}

func TestNormalizeClearsBlankDescription(t *testing.T) {
	description := " \n "
	value := Shortcut{Name: "shell", Command: "cmd", Description: &description}

	if err := value.Normalize(); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if value.Description != nil {
		t.Fatalf("Description = %#v, want nil", value.Description)
	}
}
