package cloudapi

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"

	agentapp "gitee.com/leoninew/TermBridge-go/internal/agent/application/user"
	cloudproto "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/cloud/v1"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/codec"
)

type Config struct {
	ApiBaseUrl string
	HttpClient *http.Client
}

type Client struct {
	apiBaseUrl string
	httpClient *http.Client
}

func New(config Config) *Client {
	if config.HttpClient == nil {
		panic("cloud API HTTP client is required")
	}
	return &Client{apiBaseUrl: config.ApiBaseUrl, httpClient: config.HttpClient}
}

func (c *Client) RegisterCurrentDevice(ctx context.Context, cloudToken string, device agentapp.Device) error {
	reportBody, err := codec.MarshalProtoJSON(&cloudproto.CurrentDeviceReq{Id: device.Id, Name: device.Name, PublicKey: device.PublicKey})
	if err != nil {
		return err
	}
	reportReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBaseUrl+"/api/devices/current", bytes.NewReader(reportBody))
	if err != nil {
		return err
	}
	reportReq.Header.Set("Content-Type", "application/json")
	reportReq.Header.Set("Authorization", "Bearer "+cloudToken)
	reportResp, err := c.httpClient.Do(reportReq)
	if err != nil {
		return err
	}
	defer func() { _ = reportResp.Body.Close() }()
	if reportResp.StatusCode < http.StatusOK || reportResp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("cloud device report status %d", reportResp.StatusCode)
	}
	return nil
}

func (c *Client) ExchangeOAuthCode(ctx context.Context, client agentapp.OAuthClientConfig, code string) (string, error) {
	cfg := oauth2.Config{
		ClientID:     client.ClientId,
		ClientSecret: client.ClientSecret,
		RedirectURL:  client.RedirectUrl,
		Scopes:       append([]string(nil), client.Scopes...),
		Endpoint: oauth2.Endpoint{
			TokenURL:  c.apiBaseUrl + "/api/oauth2/token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
	requestCtx := context.WithValue(ctx, oauth2.HTTPClient, c.httpClient)
	token, err := cfg.Exchange(requestCtx, code)
	if err != nil {
		return "", err
	}
	if token.AccessToken == "" {
		return "", fmt.Errorf("cloud OAuth token response missing access token")
	}
	return token.AccessToken, nil
}
