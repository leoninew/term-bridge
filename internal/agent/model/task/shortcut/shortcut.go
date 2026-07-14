package shortcut

import (
	"errors"
	"strings"
	"time"
)

const SchemaVersion = 1

type Shortcut struct {
	SchemaVersion int       `json:"schema_version"`
	Id            string    `json:"shortcut_id"`
	Name          string    `json:"name"`
	Command       string    `json:"command"`
	Description   *string   `json:"description,omitempty"`
	Icon          *string   `json:"icon,omitempty"`
	Enabled       *bool     `json:"enabled,omitempty"`
	Tags          []string  `json:"tags,omitempty"`
	LastUsedAt    time.Time `json:"last_used_at,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (value *Shortcut) Normalize() error {
	value.Id = strings.TrimSpace(value.Id)
	value.Name = strings.TrimSpace(value.Name)
	if value.Name == "" {
		return errors.New("shortcut name is required")
	}
	if strings.TrimSpace(value.Command) == "" {
		return errors.New("shortcut command is required")
	}
	value.Description = normalizeOptionalText(value.Description)
	value.Icon = normalizeOptionalText(value.Icon)
	value.Tags = normalizeTags(value.Tags)
	return nil
}

func normalizeTags(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	tags := make([]string, 0, len(values))
	for _, value := range values {
		tag := strings.TrimSpace(value)
		if tag == "" {
			continue
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}
	return tags
}

func normalizeOptionalText(value *string) *string {
	if value == nil {
		return nil
	}
	text := strings.TrimSpace(*value)
	if text == "" {
		return nil
	}
	return &text
}
