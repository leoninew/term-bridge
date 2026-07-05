package api

import "errors"

const ProviderLocalAdmin = "local_admin"

var ErrInvalidCredentials = errors.New("invalid credentials")

type UserView struct {
	Id            string `json:"id"`
	Email         string `json:"email"`
	DisplayName   string `json:"display_name"`
	Provider      string `json:"provider"`
	EmailVerified bool   `json:"email_verified"`
}

type AuthResult struct {
	Token string
	User  UserView
}
