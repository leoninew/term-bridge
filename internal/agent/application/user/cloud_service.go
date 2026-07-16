package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	cloudproto "gitee.com/leoninew/TermBridge-go/internal/gen/proto/termbridge/cloud/v1"
	"gitee.com/leoninew/TermBridge-go/internal/shared/common/utils/prototime"
)

type CloudApi interface {
	RegisterCurrentDevice(ctx context.Context, cloudToken string, device Device) error
	AuthMe(ctx context.Context, cloudToken string) (*cloudproto.AuthMeResp, error)
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

func (s *CloudService) ExchangeOAuthCode(ctx context.Context, code string) (string, error) {
	return s.cloudApi.ExchangeOAuthCode(ctx, s.config.OAuthClient, code)
}
