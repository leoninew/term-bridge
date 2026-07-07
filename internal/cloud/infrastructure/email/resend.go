package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	cloudauth "gitee.com/leoninew/TermBridge-go/internal/cloud/application/user/auth"
)

type ResendSender struct {
	apiKey   string
	from     string
	client   *http.Client
	endpoint string
}

type Config struct {
	ApiKey    string
	FromEmail string
}

func NewResendSender(cfg Config) *ResendSender {
	if cfg.ApiKey == "" {
		return nil
	}
	return &ResendSender{apiKey: cfg.ApiKey, from: cfg.FromEmail, client: &http.Client{Timeout: 10 * time.Second}, endpoint: "https://api.resend.com/emails"}
}

func (s *ResendSender) Send(ctx context.Context, to, subject, html string) (cloudauth.EmailResult, error) {
	payload := map[string]any{"from": s.from, "to": []string{to}, "subject": subject, "html": html}
	data, err := json.Marshal(payload)
	if err != nil {
		return cloudauth.EmailResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(data))
	if err != nil {
		return cloudauth.EmailResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return cloudauth.EmailResult{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var body struct {
		Id string `json:"id"`
	}
	_ = json.Unmarshal(bodyBytes, &body)
	result := cloudauth.EmailResult{MessageId: body.Id, Success: resp.StatusCode >= 200 && resp.StatusCode < 300, ResponseBody: string(bodyBytes)}
	if !result.Success {
		return result, fmt.Errorf("resend status %d", resp.StatusCode)
	}
	return result, nil
}
