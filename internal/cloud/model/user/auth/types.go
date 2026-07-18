package auth

import "errors"

var (
	ErrInvalidCredentials    = errors.New("invalid credentials")
	ErrEmailNotVerified      = errors.New("email not verified")
	ErrEmailAlreadyUsed      = errors.New("email already used")
	ErrCodeInvalid           = errors.New("code invalid or expired")
	ErrCodeCooldown          = errors.New("code resend cooldown is active")
	ErrPasswordInvalid       = errors.New("password does not meet policy")
	ErrPasswordUnchanged     = errors.New("new password must differ from current password")
	ErrProviderUnsupported   = errors.New("provider unsupported")
	ErrOAuthDisabled         = errors.New("oauth provider is not configured")
	ErrOAuthEmailUnavailable = errors.New("oauth provider did not provide a usable verified email")
)