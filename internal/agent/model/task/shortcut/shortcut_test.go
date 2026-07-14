package shortcut

import (
	"testing"
	"time"
)

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

func TestNormalizeTagsRemovesBlankAndDuplicateValues(t *testing.T) {
	value := Shortcut{Name: "shell", Command: "cmd", Tags: []string{"  build  ", "build", "", "release"}}

	if err := value.Normalize(); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if len(value.Tags) != 2 || value.Tags[0] != "build" || value.Tags[1] != "release" {
		t.Fatalf("Tags = %#v, want normalized unique tags", value.Tags)
	}
}

func TestNormalizePreservesLastUsedAt(t *testing.T) {
	lastUsedAt := time.Date(2026, time.July, 14, 12, 30, 0, 0, time.UTC)
	value := Shortcut{Name: "shell", Command: "cmd", LastUsedAt: lastUsedAt}

	if err := value.Normalize(); err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if !value.LastUsedAt.Equal(lastUsedAt) {
		t.Fatalf("LastUsedAt = %s, want %s", value.LastUsedAt, lastUsedAt)
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
