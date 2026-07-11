package config

import (
	"fmt"
	"net/url"
	"strings"

	browserdto "gitee.com/leoninew/TermBridge-go/internal/shared/dto/browser"
)

const browserApiBaseUrl = "/api"

func BuildBrowserRuntimeConfig(cfg Config, mode string) (browserdto.RuntimeConfig, error) {
	if mode != "local" && mode != "cloud" {
		return browserdto.RuntimeConfig{}, fmt.Errorf("unsupported browser runtime mode %q", mode)
	}
	if err := validateBrowserPublicUrl("local.public_url", cfg.Local.PublicUrl); err != nil {
		return browserdto.RuntimeConfig{}, err
	}
	if err := validateBrowserPublicUrl("cloud.public_url", cfg.Cloud.PublicUrl); err != nil {
		return browserdto.RuntimeConfig{}, err
	}
	if strings.TrimSpace(cfg.Local.OAuth.ClientId) == "" {
		return browserdto.RuntimeConfig{}, fmt.Errorf("local.oauth.client_id is required for browser runtime config")
	}
	if err := validateBrowserPublicUrl("local.oauth.redirect_url", cfg.Local.OAuth.RedirectUrl); err != nil {
		return browserdto.RuntimeConfig{}, err
	}
	if len(cfg.Local.OAuth.Scopes) == 0 {
		return browserdto.RuntimeConfig{}, fmt.Errorf("local.oauth.scopes is required for browser runtime config")
	}

	return browserdto.RuntimeConfig{
		Local: browserdto.RuntimeLocalConfig{
			Mode:       mode,
			PublicUrl:  cfg.Local.PublicUrl,
			ApiBaseUrl: browserApiBaseUrl,
			CloudOAuth: browserdto.CloudOAuthConfig{
				ClientId:    cfg.Local.OAuth.ClientId,
				RedirectUrl: cfg.Local.OAuth.RedirectUrl,
				Scopes:      append([]string(nil), cfg.Local.OAuth.Scopes...),
			},
		},
		Cloud: browserdto.RuntimeCloudConfig{
			PublicUrl:  cfg.Cloud.PublicUrl,
			ApiBaseUrl: browserApiBaseUrl,
		},
	}, nil
}

func validateBrowserPublicUrl(key string, value string) error {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%s must be an absolute HTTP(S) URL", key)
	}
	return nil
}
