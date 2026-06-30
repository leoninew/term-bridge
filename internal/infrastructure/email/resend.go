package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	authapp "termbridge-go/internal/application/auth"
	"termbridge-go/internal/infrastructure/config"
)

type ResendSender struct {
	apiKey   string
	from     string
	client   *http.Client
	endpoint string
}

func NewResendSender(cfg config.ResendConfig) *ResendSender {
	if !config.IsResendEnabled(cfg) {
		return nil
	}
	return &ResendSender{apiKey: cfg.APIKey, from: cfg.FromEmail, client: &http.Client{Timeout: 10 * time.Second}, endpoint: "https://api.resend.com/emails"}
}

func (s *ResendSender) Send(ctx context.Context, to, subject, html string) (authapp.EmailResult, error) {
	payload := map[string]any{"from": s.from, "to": []string{to}, "subject": subject, "html": html}
	data, err := json.Marshal(payload)
	if err != nil {
		return authapp.EmailResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(data))
	if err != nil {
		return authapp.EmailResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return authapp.EmailResult{}, err
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var body struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(bodyBytes, &body)
	result := authapp.EmailResult{MessageID: body.ID, Success: resp.StatusCode >= 200 && resp.StatusCode < 300, ResponseBody: string(bodyBytes)}
	if !result.Success {
		return result, fmt.Errorf("resend status %d", resp.StatusCode)
	}
	return result, nil
}
