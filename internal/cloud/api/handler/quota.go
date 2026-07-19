package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	quotapkg "gitee.com/leoninew/TermBridge-go/internal/shared/application/quota"
	sharedauth "gitee.com/leoninew/TermBridge-go/internal/shared/common/auth"
)

type quotaLimitView struct {
	Limit  int    `json:"limit"`
	Usage  int    `json:"usage"`
	Source string `json:"source"`
}

type quotaSummary struct {
	Subject            string         `json:"subject"`
	ConcurrentAttaches quotaLimitView `json:"concurrentAttaches"`
}

type quotaUpdateRequest struct {
	ConcurrentAttaches *int `json:"concurrentAttaches"`
}

func (s *Handler) attachLimitForUser(ctx context.Context, userID string) (int, error) {
	limit := s.config.ConcurrentAttaches
	if limit <= 0 {
		limit = quotapkg.DefaultConcurrentAttaches
	}
	if s.config.QuotaRepository == nil {
		return limit, nil
	}
	override, ok, err := s.config.QuotaRepository.GetLimit(ctx, userID, quotapkg.KeyConcurrentAttaches)
	if err != nil {
		return 0, err
	}
	if ok {
		return override, nil
	}
	return limit, nil
}

func (s *Handler) quotaSummaryFor(ctx context.Context, userID string) (quotaSummary, error) {
	defaultLimit := s.config.ConcurrentAttaches
	if defaultLimit <= 0 {
		defaultLimit = quotapkg.DefaultConcurrentAttaches
	}
	source := "default"
	limit := defaultLimit
	if s.config.QuotaRepository != nil {
		override, ok, err := s.config.QuotaRepository.GetLimit(ctx, userID, quotapkg.KeyConcurrentAttaches)
		if err != nil {
			return quotaSummary{}, err
		}
		if ok {
			limit = override
			source = "override"
		}
	}
	return quotaSummary{
		Subject: userID,
		ConcurrentAttaches: quotaLimitView{
			Limit:  limit,
			Usage:  s.attachQuota.Usage(userID),
			Source: source,
		},
	}, nil
}

func (s *Handler) handleMyQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, r, http.MethodGet)
		return
	}
	claims, ok := s.claimsFromRequest(r)
	if !ok {
		s.writeUnauthorized(w, r)
		return
	}
	summary, err := s.quotaSummaryFor(r.Context(), claims.Sub)
	if err != nil {
		s.writeAPIError(w, r, http.StatusInternalServerError, errorCodeInternal, errorMessageInternal, err)
		return
	}
	writeJSONObject(w, http.StatusOK, summary)
}

func (s *Handler) handleAdminUserQuota(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.claimsFromRequest(r)
	if !ok {
		s.writeUnauthorized(w, r)
		return
	}
	if !s.isAdmin(r.Context(), claims.Sub) {
		s.writeAPIError(w, r, http.StatusForbidden, "forbidden", "Administrator access is required.", nil)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[1] != "quota" || strings.TrimSpace(parts[0]) == "" {
		s.writeNotFound(w, r)
		return
	}
	userID := strings.TrimSpace(parts[0])
	switch r.Method {
	case http.MethodGet:
		summary, err := s.quotaSummaryFor(r.Context(), userID)
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, errorCodeInternal, errorMessageInternal, err)
			return
		}
		writeJSONObject(w, http.StatusOK, summary)
	case http.MethodPut:
		if s.config.QuotaRepository == nil {
			s.writeAPIError(w, r, http.StatusServiceUnavailable, errorCodeInternal, "Quota repository is not configured.", nil)
			return
		}
		var body quotaUpdateRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil || body.ConcurrentAttaches == nil {
			s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, errorMessageBadRequest, err)
			return
		}
		value := *body.ConcurrentAttaches
		if value < 1 || value > 256 {
			s.writeAPIError(w, r, http.StatusBadRequest, errorCodeBadRequest, "concurrentAttaches must be between 1 and 256.", nil)
			return
		}
		if err := s.config.QuotaRepository.UpsertLimit(r.Context(), userID, quotapkg.KeyConcurrentAttaches, value, claims.Sub); err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, errorCodeInternal, errorMessageInternal, err)
			return
		}
		summary, err := s.quotaSummaryFor(r.Context(), userID)
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, errorCodeInternal, errorMessageInternal, err)
			return
		}
		writeJSONObject(w, http.StatusOK, summary)
	case http.MethodDelete:
		if s.config.QuotaRepository == nil {
			s.writeAPIError(w, r, http.StatusServiceUnavailable, errorCodeInternal, "Quota repository is not configured.", nil)
			return
		}
		if err := s.config.QuotaRepository.DeleteLimit(r.Context(), userID, quotapkg.KeyConcurrentAttaches); err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, errorCodeInternal, errorMessageInternal, err)
			return
		}
		summary, err := s.quotaSummaryFor(r.Context(), userID)
		if err != nil {
			s.writeAPIError(w, r, http.StatusInternalServerError, errorCodeInternal, errorMessageInternal, err)
			return
		}
		writeJSONObject(w, http.StatusOK, summary)
	default:
		s.methodNotAllowed(w, r, http.MethodGet, http.MethodPut, http.MethodDelete)
	}
}

func (s *Handler) isAdmin(ctx context.Context, userID string) bool {
	userID = strings.TrimSpace(userID)
	for _, id := range s.config.AdminUserIds {
		if strings.TrimSpace(id) == userID {
			return true
		}
	}
	if len(s.config.AdminEmails) == 0 || s.authService == nil {
		return false
	}
	user, err := s.authService.UserFromClaims(ctx, sharedauth.Claims{Sub: userID})
	if err != nil || user == nil {
		return false
	}
	email := strings.ToLower(strings.TrimSpace(user.GetEmail()))
	for _, allowed := range s.config.AdminEmails {
		if email != "" && email == strings.ToLower(strings.TrimSpace(allowed)) {
			return true
		}
	}
	return false
}
