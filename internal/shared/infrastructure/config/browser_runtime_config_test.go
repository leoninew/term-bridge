package config

import (
	"reflect"
	"testing"

	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/version"
)

func TestBuildBrowserRuntimeConfigProjectsPublicConfiguration(t *testing.T) {
	cfg := Config{
		Local: LocalConfig{
			PublicUrl: "http://localhost:9030",
			OAuth: LocalOAuthConfig{
				ClientId:     "termbridge-agent",
				ClientSecret: "must-not-be-exposed",
				RedirectUrl:  "http://localhost:9030/oauth/callback",
				Scopes:       []string{"openid", "email"},
			},
		},
		Cloud: CloudConfig{
			PublicUrl: "https://termbridge.preflite.cn",
			Turnstile: TurnstileConfig{
				SecretKey: "must-not-be-exposed",
			},
		},
		Jwt: JwtConfig{SecretKey: "must-not-be-exposed"},
	}

	runtimeConfig, err := BuildBrowserRuntimeConfig(cfg, "cloud")
	if err != nil {
		t.Fatalf("BuildBrowserRuntimeConfig() error = %v", err)
	}

	if runtimeConfig.Version != version.Version {
		t.Fatalf("Version = %q, want %q", runtimeConfig.Version, version.Version)
	}
	if runtimeConfig.Local.Mode != "cloud" {
		t.Fatalf("Local.Mode = %q, want cloud", runtimeConfig.Local.Mode)
	}
	if runtimeConfig.Local.PublicUrl != "http://localhost:9030" || runtimeConfig.Cloud.PublicUrl != "https://termbridge.preflite.cn" {
		t.Fatalf("public URLs = local %q cloud %q", runtimeConfig.Local.PublicUrl, runtimeConfig.Cloud.PublicUrl)
	}
	if runtimeConfig.Local.ApiBaseUrl != "/api" || runtimeConfig.Cloud.ApiBaseUrl != "/api" {
		t.Fatalf("API bases = local %q cloud %q, want /api", runtimeConfig.Local.ApiBaseUrl, runtimeConfig.Cloud.ApiBaseUrl)
	}
	if runtimeConfig.Local.CloudOAuth.ClientId != "termbridge-agent" || runtimeConfig.Local.CloudOAuth.RedirectUrl != "http://localhost:9030/oauth/callback" {
		t.Fatalf("CloudOAuth = %#v", runtimeConfig.Local.CloudOAuth)
	}
	if !reflect.DeepEqual(runtimeConfig.Local.CloudOAuth.Scopes, []string{"openid", "email"}) {
		t.Fatalf("CloudOAuth.Scopes = %#v", runtimeConfig.Local.CloudOAuth.Scopes)
	}

	cfg.Local.OAuth.Scopes[0] = "changed"
	if runtimeConfig.Local.CloudOAuth.Scopes[0] != "openid" {
		t.Fatalf("CloudOAuth.Scopes was not copied: %#v", runtimeConfig.Local.CloudOAuth.Scopes)
	}
}

func TestBuildBrowserRuntimeConfigRejectsIncompletePublicConfiguration(t *testing.T) {
	cfg := Config{
		Local: LocalConfig{PublicUrl: "http://localhost:9030"},
		Cloud: CloudConfig{PublicUrl: "https://termbridge.preflite.cn"},
	}

	if _, err := BuildBrowserRuntimeConfig(cfg, "local"); err == nil {
		t.Fatal("BuildBrowserRuntimeConfig() succeeded without browser OAuth configuration")
	}
}
