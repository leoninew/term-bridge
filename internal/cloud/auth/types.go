package auth

import "errors"

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrEmailNotVerified    = errors.New("email not verified")
	ErrEmailAlreadyUsed    = errors.New("email already used")
	ErrCodeInvalid         = errors.New("code invalid or expired")
	ErrCodeCooldown        = errors.New("code resend cooldown is active")
	ErrProviderUnsupported = errors.New("provider unsupported")
)

type UserView struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	DisplayName   string `json:"display_name"`
	Provider      string `json:"provider"`
	EmailVerified bool   `json:"email_verified"`
}

type Capabilities struct {
	Providers                []string `json:"providers"`
	PasswordResetEnabled     bool     `json:"password_reset_enabled"`
	EmailVerificationEnabled bool     `json:"email_verification_enabled"`
	AccountAuthEnabled       bool     `json:"account_auth_enabled"`
	CloudOAuthEnabled        bool     `json:"cloud_oauth_enabled"`
}

type Result struct {
	Token string
	User  UserView
}
