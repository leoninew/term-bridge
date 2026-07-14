package shortcut

import (
	"errors"
	"os"
	"strings"
	"time"

	shortcutmodel "gitee.com/leoninew/TermBridge-go/internal/agent/model/task/shortcut"
	agent "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/agent/v1"
	apperrors "gitee.com/leoninew/TermBridge-go/internal/shared/common/errors"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/prototime"
)

type Store interface {
	ListShortcuts() ([]shortcutmodel.Shortcut, error)
	CreateShortcut(shortcutmodel.Shortcut) (shortcutmodel.Shortcut, error)
	UpdateShortcut(shortcutId string, update func(*shortcutmodel.Shortcut) error) (shortcutmodel.Shortcut, error)
	UpdateShortcutOrder(shortcutIds []string) ([]shortcutmodel.Shortcut, error)
	DeleteShortcut(shortcutId string) error
}

type Service struct {
	store Store
}

func NewService(store Store) Service {
	return Service{store: store}
}

func (s Service) List() ([]*agent.Shortcut, error) {
	values, err := s.store.ListShortcuts()
	if err != nil {
		return nil, apperrors.Runtime("list shortcuts", err)
	}
	items := make([]*agent.Shortcut, 0, len(values))
	for _, value := range values {
		items = append(items, protoFromModel(value))
	}
	return items, nil
}

func (s Service) Create(request *agent.CreateShortcutReq) (*agent.Shortcut, error) {
	if request == nil {
		return nil, apperrors.Usage("shortcut create request is required")
	}
	enabled := true
	if request.Enabled != nil {
		enabled = request.GetEnabled()
	}
	value := shortcutmodel.Shortcut{
		Name:        request.GetName(),
		Command:     request.GetCommand(),
		Description: request.Description,
		Icon:        request.Icon,
		Enabled:     &enabled,
		Tags:        request.Tags,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	created, err := s.store.CreateShortcut(value)
	if err != nil {
		return nil, shortcutError("create shortcut", err)
	}
	return protoFromModel(created), nil
}

func (s Service) Update(shortcutId string, request *agent.UpdateShortcutReq) (*agent.Shortcut, error) {
	shortcutId = strings.TrimSpace(shortcutId)
	if shortcutId == "" {
		return nil, apperrors.Usage("shortcut id is required")
	}
	if request == nil || (request.Name == nil && request.Command == nil && request.Description == nil && request.Icon == nil && request.Enabled == nil && request.Tags == nil) {
		return nil, apperrors.Usage("shortcut update requires a field")
	}
	updated, err := s.store.UpdateShortcut(shortcutId, func(value *shortcutmodel.Shortcut) error {
		if request.Name != nil {
			value.Name = request.GetName()
		}
		if request.Command != nil {
			value.Command = request.GetCommand()
		}
		if request.Description != nil {
			value.Description = request.Description
		}
		if request.Icon != nil {
			value.Icon = request.Icon
		}
		if request.Enabled != nil {
			value.Enabled = request.Enabled
		}
		if request.Tags != nil {
			value.Tags = request.Tags
		}
		value.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return nil, shortcutError("update shortcut", err)
	}
	return protoFromModel(updated), nil
}

func (s Service) UpdateOrder(request *agent.UpdateShortcutOrderReq) ([]*agent.Shortcut, error) {
	if request == nil || len(request.GetShortcutIds()) == 0 {
		return nil, apperrors.Usage("shortcut_ids is required")
	}
	updated, err := s.store.UpdateShortcutOrder(request.GetShortcutIds())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, apperrors.NotFound("shortcut not found", err)
		}
		if errors.Is(err, os.ErrInvalid) {
			return nil, apperrors.Usage(err.Error())
		}
		return nil, apperrors.Runtime("update shortcut order", err)
	}
	items := make([]*agent.Shortcut, 0, len(updated))
	for _, value := range updated {
		items = append(items, protoFromModel(value))
	}
	return items, nil
}

func (s Service) Delete(shortcutId string) error {
	shortcutId = strings.TrimSpace(shortcutId)
	if shortcutId == "" {
		return apperrors.Usage("shortcut id is required")
	}
	if err := s.store.DeleteShortcut(shortcutId); err != nil {
		return shortcutError("delete shortcut", err)
	}
	return nil
}

func shortcutError(operation string, err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return apperrors.NotFound("shortcut not found", err)
	}
	if strings.Contains(err.Error(), "shortcut name is required") || strings.Contains(err.Error(), "shortcut command is required") {
		return apperrors.Usage(err.Error())
	}
	return apperrors.Runtime(operation, err)
}

func protoFromModel(value shortcutmodel.Shortcut) *agent.Shortcut {
	return &agent.Shortcut{
		Id:          value.Id,
		Name:        value.Name,
		Command:     value.Command,
		Description: value.Description,
		Icon:        value.Icon,
		Enabled:     value.Enabled,
		Tags:        value.Tags,
		LastUsedAt:  prototime.FromTime(value.LastUsedAt),
		CreatedAt:   prototime.FromTime(value.CreatedAt),
		UpdatedAt:   prototime.FromTime(value.UpdatedAt),
	}
}
