package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	cloudproto "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/cloud/v1"
	shared "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/shared/v1"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/prototime"
)

type CloudOperation string

const (
	CloudOperationRegisterCurrentDevice CloudOperation = "register_current_device"
	CloudOperationAuthMe                CloudOperation = "auth_me"
	CloudOperationListDevices           CloudOperation = "list_devices"
	CloudOperationExchangeOAuthCode     CloudOperation = "exchange_oauth_code"
)

type CloudUpstreamError struct {
	Operation      CloudOperation
	UpstreamStatus int
	CloudError     *shared.ErrorResp
	Cause          error
}

func NewCloudUpstreamError(operation CloudOperation, upstreamStatus int, cloudError *shared.ErrorResp, cause error) *CloudUpstreamError {
	return &CloudUpstreamError{Operation: operation, UpstreamStatus: upstreamStatus, CloudError: cloudError, Cause: cause}
}

func (e *CloudUpstreamError) Error() string {
	if e == nil {
		return "cloud upstream error"
	}
	if e.CloudError != nil && strings.TrimSpace(e.CloudError.GetError()) != "" {
		return e.CloudError.GetError()
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "cloud upstream error"
}

func (e *CloudUpstreamError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *CloudUpstreamError) CloudCode() string {
	if e == nil || e.CloudError == nil {
		return ""
	}
	return e.CloudError.GetCode()
}

type CloudApi interface {
	RegisterCurrentDevice(ctx context.Context, cloudToken string, device Device) error
	AuthMe(ctx context.Context, cloudToken string) (*cloudproto.AuthMeResp, error)
	ListDevices(ctx context.Context, cloudToken string) (*cloudproto.ListDevicesResp, error)
	ExchangeOAuthCode(ctx context.Context, client OAuthClientConfig, code string) (string, error)
}

type OAuthClientConfig struct {
	ClientId     string
	ClientSecret string
	RedirectUrl  string
	Scopes       []string
}

type CloudServiceConfig struct {
	PublicUrl   string
	OAuthClient OAuthClientConfig
}

type CloudService struct {
	cloudApi CloudApi
	config   CloudServiceConfig
	now      func() time.Time
}

func NewCloudService(cloudApi CloudApi, config CloudServiceConfig) *CloudService {
	if cloudApi == nil {
		panic("agent cloud API client is required")
	}
	return &CloudService{cloudApi: cloudApi, config: config, now: time.Now}
}

func (s *CloudService) Connect(ctx context.Context, cloudToken string, device Device) (*cloudproto.CloudSessionSummary, error) {
	if device.Id == "" || device.Name == "" {
		return nil, fmt.Errorf("local device identity is incomplete")
	}
	if strings.TrimSpace(device.PublicKey) == "" {
		return nil, fmt.Errorf("local device public key is incomplete")
	}
	if err := s.cloudApi.RegisterCurrentDevice(ctx, cloudToken, device); err != nil {
		return nil, err
	}
	return &cloudproto.CloudSessionSummary{
		PublicUrl:   s.config.PublicUrl,
		DeviceId:    device.Id,
		DeviceName:  device.Name,
		ConnectedAt: prototime.FromTime(s.now().UTC()),
	}, nil
}

func (s *CloudService) AuthMe(ctx context.Context, cloudToken string) (*cloudproto.AuthMeResp, error) {
	return s.cloudApi.AuthMe(ctx, cloudToken)
}

func (s *CloudService) ListDevices(ctx context.Context, cloudToken string) (*cloudproto.ListDevicesResp, error) {
	return s.cloudApi.ListDevices(ctx, cloudToken)
}

func (s *CloudService) ExchangeOAuthCode(ctx context.Context, code string) (string, error) {
	return s.cloudApi.ExchangeOAuthCode(ctx, s.config.OAuthClient, code)
}
