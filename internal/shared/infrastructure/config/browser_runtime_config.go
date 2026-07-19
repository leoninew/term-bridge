package config

import (
	"fmt"
	"net/url"
	"strings"

	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/version"
	browserdto "gitee.com/leoninew/TermBridge-go/internal/shared/dto/browser"
)

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
		Version: version.Version,
		Local: browserdto.RuntimeLocalConfig{
			Mode:        mode,
			PublicUrl:   cfg.Local.PublicUrl,
			ApiBasePath: cfg.Local.ApiBasePath,
			CloudOAuth: browserdto.CloudOAuthConfig{
				ClientId:    cfg.Local.OAuth.ClientId,
				RedirectUrl: cfg.Local.OAuth.RedirectUrl,
				Scopes:      append([]string(nil), cfg.Local.OAuth.Scopes...),
			},
		},
		Cloud: browserdto.RuntimeCloudConfig{
			PublicUrl:               cfg.Cloud.PublicUrl,
			ApiBaseUrl:              cfg.Cloud.ApiBaseUrl,
			ExternalAuthProviderIds: enabledExternalAuthProviderIds(cfg.Cloud),
		},
		Terminal: &browserdto.RuntimeTerminalConfig{
			KeepAlive: browserdto.RuntimeTerminalKeepAliveConfig{
				MaxHotTerminals: cfg.Terminal.KeepAlive.MaxHotTerminals,
				DisposeDelayMs:  cfg.Terminal.KeepAlive.DisposeDelayMs,
			},
		},
	}, nil
}

func enabledExternalAuthProviderIds(cfg CloudConfig) []string {
	providerIds := make([]string, 0, 2)
	if IsGoogleAuthEnabled(cfg.Google) {
		providerIds = append(providerIds, "google")
	}
	if strings.TrimSpace(cfg.GitHub.ClientId) != "" {
		providerIds = append(providerIds, "github")
	}
	return providerIds
}

func validateBrowserPublicUrl(key string, value string) error {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("%s must be an absolute HTTP(S) URL", key)
	}
	return nil
}