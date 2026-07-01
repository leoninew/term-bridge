package gatewayapi

import (
	"time"

	authapp "termbridge-go/internal/application/auth"
)

type DeviceSummary struct {
	Id          string    `json:"id"`
	Name        string    `json:"name"`
	Online      bool      `json:"online"`
	Status      string    `json:"status"`
	ConnectedAt time.Time `json:"connected_at"`
	LastSeen    time.Time `json:"last_seen"`
}

type CloudSessionSummary struct {
	GateURL     string    `json:"gate_url"`
	DeviceId    string    `json:"device_id"`
	DeviceName  string    `json:"device_name"`
	ConnectedAt time.Time `json:"connected_at"`
}

type APIErrorResp[T any] struct {
	Code      string `json:"code"`
	Error     string `json:"error"`
	RequestId string `json:"requestId"`
	Details   T      `json:"details,omitempty"`
}

type errorResponse = APIErrorResp[any]

type HealthResp struct {
	Status string `json:"status"`
}

type PaginatedResp[T any] struct {
	Items      []T `json:"items"`
	Total      int `json:"total"`
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalPages int `json:"total_pages"`
}

type ListResp[T any] struct {
	Items []T `json:"items"`
}

type ListDevicesResp struct {
	Items []DeviceSummary `json:"items"`
}

type TokenResp struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type AuthLoginReq struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthRegisterReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthVerifyEmailReq struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type AuthResendVerificationReq struct {
	Email string `json:"email"`
}

type AuthChangePasswordReq struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type AuthPasswordResetRequestReq struct {
	Email string `json:"email"`
}

type AuthPasswordResetConfirmReq struct {
	Email       string `json:"email"`
	Code        string `json:"code"`
	NewPassword string `json:"new_password"`
}

type GoogleAuthURLResp struct {
	AuthURL string `json:"auth_url"`
}

type AuthGoogleCallbackReq struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

type AuthMeResp struct {
	Authenticated bool                  `json:"authenticated"`
	Username      string                `json:"username,omitempty"`
	User          *authapp.UserView     `json:"user,omitempty"`
	Capabilities  *authapp.Capabilities `json:"capabilities,omitempty"`
	CloudSession  *CloudSessionSummary  `json:"cloud_session"`
}

type CloudOAuthStartResp struct {
	AuthorizeURL string `json:"authorize_url"`
}

type CloudOAuthAuthorizeResp struct {
	RedirectURL string `json:"redirect_url"`
}

type CloudOAuthExchangeReq struct {
	Code string `json:"code"`
}

type CloudOAuthCallbackReq struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

type CloudOAuthCallbackResp struct {
	CloudSession CloudSessionSummary `json:"cloud_session"`
	Redirect     string              `json:"redirect"`
}

type CurrentDeviceReq struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
}

type CurrentDeviceResp struct {
	Accepted bool          `json:"accepted"`
	Device   DeviceSummary `json:"device"`
}

type CloudOAuthExchangeTokenReq struct {
	Code string `json:"code"`
}

type CloudOAuthDeviceReportReq struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	PublicKey string `json:"public_key"`
}

type CloudOAuthExchangeTokenResp struct {
	AccessToken string `json:"access_token"`
}
