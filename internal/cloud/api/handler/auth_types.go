package api

import cloudauth "termbridge/internal/cloud/model/user/auth"

type UserView = cloudauth.UserView

type AuthResult = cloudauth.Result

const ProviderLocalAdmin = "local_admin"

var (
	ErrInvalidCredentials  = cloudauth.ErrInvalidCredentials
	ErrEmailNotVerified    = cloudauth.ErrEmailNotVerified
	ErrEmailAlreadyUsed    = cloudauth.ErrEmailAlreadyUsed
	ErrCodeInvalid         = cloudauth.ErrCodeInvalid
	ErrCodeCooldown        = cloudauth.ErrCodeCooldown
	ErrProviderUnsupported = cloudauth.ErrProviderUnsupported
)
