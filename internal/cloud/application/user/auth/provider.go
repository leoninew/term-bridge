package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	authmodel "gitee.com/leoninew/TermBridge-go/internal/cloud/model/user/auth"
	repository "gitee.com/leoninew/TermBridge-go/internal/cloud/repository/user/auth"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

type ExternalIdentity struct {
	Provider      string
	Subject       string
	Email         string
	EmailVerified bool
}

type ExternalProvider interface {
	Id() string
	AuthCodeURL(state string) string
	ExchangeIdentity(ctx context.Context, code string) (ExternalIdentity, error)
}

type ProviderRegistry struct {
	providers []ExternalProvider
}

func NewProviderRegistry(providers ...ExternalProvider) ProviderRegistry {
	filtered := make([]ExternalProvider, 0, len(providers))
	for _, provider := range providers {
		if provider != nil {
			filtered = append(filtered, provider)
		}
	}
	return ProviderRegistry{providers: filtered}
}

func (r ProviderRegistry) Get(id string) (ExternalProvider, bool) {
	id = strings.TrimSpace(id)
	for _, provider := range r.providers {
		if provider.Id() == id {
			return provider, true
		}
	}
	return nil, false
}

type OAuthGitHubClient struct{ cfg *oauth2.Config }

type GitHubConfig struct {
	ClientId     string
	ClientSecret string
	RedirectUrl  string
}

func NewOAuthGitHubClient(cfg GitHubConfig) ExternalProvider {
	if strings.TrimSpace(cfg.ClientId) == "" {
		return nil
	}
	return &OAuthGitHubClient{cfg: &oauth2.Config{
		ClientID: cfg.ClientId, ClientSecret: cfg.ClientSecret, RedirectURL: cfg.RedirectUrl,
		Scopes: []string{"read:user", "user:email"}, Endpoint: github.Endpoint,
	}}
}

func (g *OAuthGitHubClient) Id() string { return repository.ProviderGitHub }

func (g *OAuthGitHubClient) AuthCodeURL(state string) string {
	return g.cfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (g *OAuthGitHubClient) ExchangeIdentity(ctx context.Context, code string) (ExternalIdentity, error) {
	token, err := g.cfg.Exchange(ctx, code)
	if err != nil {
		return ExternalIdentity{}, err
	}
	client := g.cfg.Client(ctx, token)
	var user struct {
		Id json.Number `json:"id"`
	}
	if err := getGitHubJSON(ctx, client, "https://api.github.com/user", &user); err != nil {
		return ExternalIdentity{}, err
	}
	subject := strings.TrimSpace(user.Id.String())
	if subject == "" || subject == "0" {
		return ExternalIdentity{}, errors.New("github user id is missing")
	}
	if _, err := strconv.ParseInt(subject, 10, 64); err != nil {
		return ExternalIdentity{}, fmt.Errorf("github user id is invalid: %w", err)
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := getGitHubJSON(ctx, client, "https://api.github.com/user/emails", &emails); err != nil {
		return ExternalIdentity{}, err
	}
	email, ok := selectGitHubEmail(emails)
	if !ok {
		return ExternalIdentity{}, authmodel.ErrOAuthEmailUnavailable
	}
	return ExternalIdentity{Provider: g.Id(), Subject: subject, Email: email, EmailVerified: true}, nil
}

func getGitHubJSON(ctx context.Context, client *http.Client, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("github API status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, target)
}

func selectGitHubEmail(emails []struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}) (string, bool) {
	var verified []string
	for _, candidate := range emails {
		email := repository.NormalizeEmail(candidate.Email)
		if !candidate.Verified || email == "" {
			continue
		}
		if candidate.Primary {
			return email, true
		}
		verified = append(verified, email)
	}
	if len(verified) == 1 {
		return verified[0], true
	}
	return "", false
}
