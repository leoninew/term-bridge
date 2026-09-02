package cloudapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/oauth2"

	agentapp "gitee.com/leoninew/TermBridge-go/internal/agent/application/user"
	cloudproto "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/cloud/v1"
	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/requestid"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/codec"
)

const requestIdHeader = requestid.Header

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
	httpClient := *config.HttpClient
	httpClient.Transport = requestIdRoundTripper{base: config.HttpClient.Transport}
	return &Client{apiBaseUrl: config.ApiBaseUrl, httpClient: &httpClient}
}

func (c *Client) RegisterCurrentDevice(ctx context.Context, cloudToken string, device agentapp.Device) error {
	reportBody, err := codec.MarshalProtoJSON(&cloudproto.CurrentDeviceReq{Id: device.Id, Name: device.Name, PublicKey: device.PublicKey})
	if err != nil {
		return agentapp.NewCloudUpstreamError(agentapp.CloudOperationRegisterCurrentDevice, 0, nil, err)
	}
	reportReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBaseUrl+"/devices/current", bytes.NewReader(reportBody))
	if err != nil {
		return agentapp.NewCloudUpstreamError(agentapp.CloudOperationRegisterCurrentDevice, 0, nil, err)
	}
	reportReq.Header.Set("Content-Type", "application/json")
	reportReq.Header.Set("Authorization", "Bearer "+cloudToken)
	reportResp, err := c.httpClient.Do(reportReq)
	if err != nil {
		return agentapp.NewCloudUpstreamError(agentapp.CloudOperationRegisterCurrentDevice, 0, nil, err)
	}
	defer func() { _ = reportResp.Body.Close() }()
	if reportResp.StatusCode < http.StatusOK || reportResp.StatusCode >= http.StatusMultipleChoices {
		return cloudResponseError(agentapp.CloudOperationRegisterCurrentDevice, reportResp.StatusCode, reportResp.Body)
	}
	return nil
}

func (c *Client) AuthMe(ctx context.Context, cloudToken string) (*cloudproto.AuthMeResp, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBaseUrl+"/auth/me", nil)
	if err != nil {
		return nil, agentapp.NewCloudUpstreamError(agentapp.CloudOperationAuthMe, 0, nil, err)
	}
	request.Header.Set("Authorization", "Bearer "+cloudToken)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, agentapp.NewCloudUpstreamError(agentapp.CloudOperationAuthMe, 0, nil, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, cloudResponseError(agentapp.CloudOperationAuthMe, response.StatusCode, response.Body)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, agentapp.NewCloudUpstreamError(agentapp.CloudOperationAuthMe, response.StatusCode, nil, err)
	}
	var result cloudproto.AuthMeResp
	if err := codec.UnmarshalProtoJSON(body, &result); err != nil {
		return nil, agentapp.NewCloudUpstreamError(agentapp.CloudOperationAuthMe, response.StatusCode, nil, err)
	}
	return &result, nil
}

func (c *Client) ListDevices(ctx context.Context, cloudToken string) (*cloudproto.ListDevicesResp, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBaseUrl+"/devices", nil)
	if err != nil {
		return nil, agentapp.NewCloudUpstreamError(agentapp.CloudOperationListDevices, 0, nil, err)
	}
	request.Header.Set("Authorization", "Bearer "+cloudToken)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, agentapp.NewCloudUpstreamError(agentapp.CloudOperationListDevices, 0, nil, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, cloudResponseError(agentapp.CloudOperationListDevices, response.StatusCode, response.Body)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, agentapp.NewCloudUpstreamError(agentapp.CloudOperationListDevices, response.StatusCode, nil, err)
	}
	var result cloudproto.ListDevicesResp
	if err := codec.UnmarshalProtoJSON(body, &result); err != nil {
		return nil, agentapp.NewCloudUpstreamError(agentapp.CloudOperationListDevices, response.StatusCode, nil, err)
	}
	return &result, nil
}

func (c *Client) ExchangeOAuthCode(ctx context.Context, client agentapp.OAuthClientConfig, code string) (string, error) {
	cfg := oauth2.Config{
		ClientID:     client.ClientId,
		ClientSecret: client.ClientSecret,
		RedirectURL:  client.RedirectUrl,
		Scopes:       append([]string(nil), client.Scopes...),
		Endpoint: oauth2.Endpoint{
			TokenURL:  c.apiBaseUrl + "/oauth2/token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
	}
	requestCtx := context.WithValue(ctx, oauth2.HTTPClient, c.httpClient)
	token, err := cfg.Exchange(requestCtx, code)
	if err != nil {
		var retrieveError *oauth2.RetrieveError
		if errors.As(err, &retrieveError) && retrieveError.Response != nil {
			return "", cloudResponseErrorFromBody(agentapp.CloudOperationExchangeOAuthCode, retrieveError.Response.StatusCode, retrieveError.Body, err)
		}
		return "", agentapp.NewCloudUpstreamError(agentapp.CloudOperationExchangeOAuthCode, 0, nil, err)
	}
	if token.AccessToken == "" {
		return "", agentapp.NewCloudUpstreamError(agentapp.CloudOperationExchangeOAuthCode, http.StatusOK, nil, fmt.Errorf("cloud OAuth token response missing access token"))
	}
	return token.AccessToken, nil
}

func cloudResponseError(operation agentapp.CloudOperation, status int, body io.Reader) *agentapp.CloudUpstreamError {
	data, err := io.ReadAll(body)
	if err != nil {
		return agentapp.NewCloudUpstreamError(operation, status, nil, err)
	}
	return cloudResponseErrorFromBody(operation, status, data, fmt.Errorf("cloud %s status %d", operation, status))
}

func cloudResponseErrorFromBody(operation agentapp.CloudOperation, status int, body []byte, cause error) *agentapp.CloudUpstreamError {
	var cloudError shared.ErrorResp
	if err := codec.UnmarshalProtoJSON(body, &cloudError); err == nil && strings.TrimSpace(cloudError.GetCode()) != "" {
		return agentapp.NewCloudUpstreamError(operation, status, &cloudError, cause)
	} else if err != nil {
		cause = fmt.Errorf("%w: decode cloud error response", cause)
	} else {
		cause = fmt.Errorf("%w: cloud error response missing code", cause)
	}
	return agentapp.NewCloudUpstreamError(operation, status, nil, cause)
}

type requestIdRoundTripper struct {
	base http.RoundTripper
}

func (t requestIdRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	cloned := request.Clone(request.Context())
	cloned.Header = request.Header.Clone()
	if cloned.Header == nil {
		cloned.Header = make(http.Header)
	}
	if requestId := requestid.From(cloned.Context()); requestId != "" {
		cloned.Header.Set(requestIdHeader, requestId)
	}
	if t.base != nil {
		return t.base.RoundTrip(cloned)
	}
	return http.DefaultTransport.RoundTrip(cloned)
}
