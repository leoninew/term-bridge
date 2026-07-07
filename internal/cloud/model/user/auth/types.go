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
