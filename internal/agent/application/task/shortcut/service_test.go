package shortcut

import (
	"errors"
	"os"
	"testing"
	"time"

	shortcutmodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/shortcut"
	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
	apperrors "gitee.com/leoninew/TermBridge-go/internal/shared/common/errors"
)

func TestServiceCreatesAndUpdatesRawCommandShortcut(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)
	description := "review changes"
	commandText := `codex --dangerously-bypass-approvals-and-sandbox -c "review changes"`

	created, err := service.Create(&agent.CreateShortcutReq{Name: "review", Command: commandText, Description: &description})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Command != commandText || created.Description == nil || *created.Description != description {
		t.Fatalf("Create() = %#v, want raw command and description", created)
	}
	if store.value.Command != commandText {
		t.Fatalf("stored command = %q, want %q", store.value.Command, commandText)
	}

	clearDescription := ""
	updated, err := service.Update(created.Id, &agent.UpdateShortcutReq{Description: &clearDescription})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Command != commandText || updated.Description != nil {
		t.Fatalf("Update() = %#v, want preserved raw command and cleared description", updated)
	}
}

func TestServiceMapsMissingShortcutToNotFound(t *testing.T) {
	service := NewService(&fakeStore{deleteErr: os.ErrNotExist})

	err := service.Delete("missing")
	if !apperrors.IsNotFound(err) {
		t.Fatalf("Delete() error kind = %s, want not_found; error=%v", apperrors.KindOf(err), err)
	}
}

func TestServiceRejectsEmptyUpdate(t *testing.T) {
	service := NewService(&fakeStore{})

	if _, err := service.Update("shortcut-1", &agent.UpdateShortcutReq{}); !apperrors.IsUsage(err) {
		t.Fatalf("Update(empty) error kind = %s, want usage; error=%v", apperrors.KindOf(err), err)
	}
}

func TestServiceRejectsMalformedShortcutOrder(t *testing.T) {
	service := NewService(&fakeStore{orderErr: errors.Join(errors.New("duplicate shortcut_id \"shortcut-1\""), os.ErrInvalid)})

	if _, err := service.UpdateOrder(&agent.UpdateShortcutOrderReq{ShortcutIds: []string{"shortcut-1", "shortcut-1"}}); !apperrors.IsUsage(err) {
		t.Fatalf("UpdateOrder(duplicate) error kind = %s, want usage; error=%v", apperrors.KindOf(err), err)
	}
}

func TestServiceMapsMissingShortcutOrderToNotFound(t *testing.T) {
	service := NewService(&fakeStore{orderErr: os.ErrNotExist})

	if _, err := service.UpdateOrder(&agent.UpdateShortcutOrderReq{ShortcutIds: []string{"missing"}}); !apperrors.IsNotFound(err) {
		t.Fatalf("UpdateOrder(missing) error kind = %s, want not_found; error=%v", apperrors.KindOf(err), err)
	}
}

type fakeStore struct {
	value     shortcutmodel.Shortcut
	deleteErr error
	orderErr  error
}

func (s *fakeStore) ListShortcuts() ([]shortcutmodel.Shortcut, error) {
	if s.value.Id == "" {
		return nil, nil
	}
	return []shortcutmodel.Shortcut{s.value}, nil
}

func (s *fakeStore) CreateShortcut(value shortcutmodel.Shortcut) (shortcutmodel.Shortcut, error) {
	if err := value.Normalize(); err != nil {
		return shortcutmodel.Shortcut{}, err
	}
	value.Id = "shortcut-1"
	value.SchemaVersion = shortcutmodel.SchemaVersion
	value.CreatedAt = time.Now().UTC()
	value.UpdatedAt = value.CreatedAt
	s.value = value
	return value, nil
}

func (s *fakeStore) UpdateShortcut(shortcutId string, update func(*shortcutmodel.Shortcut) error) (shortcutmodel.Shortcut, error) {
	if s.value.Id == "" || s.value.Id != shortcutId {
		return shortcutmodel.Shortcut{}, os.ErrNotExist
	}
	if err := update(&s.value); err != nil {
		return shortcutmodel.Shortcut{}, err
	}
	if err := s.value.Normalize(); err != nil {
		return shortcutmodel.Shortcut{}, err
	}
	return s.value, nil
}

func (s *fakeStore) UpdateShortcutOrder([]string) ([]shortcutmodel.Shortcut, error) {
	if s.orderErr != nil {
		return nil, s.orderErr
	}
	return s.ListShortcuts()
}

func (s *fakeStore) DeleteShortcut(string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	return errors.New("unexpected delete")
}
