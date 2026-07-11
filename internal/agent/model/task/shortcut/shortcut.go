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
	if value.Description != nil {
		description := strings.TrimSpace(*value.Description)
		if description == "" {
			value.Description = nil
		} else {
			value.Description = &description
		}
	}
	return nil
}
