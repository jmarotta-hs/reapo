package auth

import "time"

// AuthType represents the type of authentication
type AuthType string

const (
	AuthTypeOAuth AuthType = "oauth"
)

// AuthInfo represents authentication information
type AuthInfo interface {
	Type() AuthType
	IsValid() bool
}

// OAuthInfo represents OAuth authentication details
type OAuthInfo struct {
	AuthType     AuthType  `json:"type"`
	RefreshToken string    `json:"refresh"`
	AccessToken  string    `json:"access"`
	ExpiresAt    time.Time `json:"expires"`
}

func (o OAuthInfo) Type() AuthType {
	return AuthTypeOAuth
}

func (o OAuthInfo) IsValid() bool {
	return o.RefreshToken != "" && (o.AccessToken != "" || time.Now().Before(o.ExpiresAt))
}

// Storage represents the complete auth storage structure
type Storage map[string]interface{}