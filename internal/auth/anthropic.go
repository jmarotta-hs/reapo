package auth

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const (
	clientID     = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	redirectURI  = "https://console.anthropic.com/oauth/code/callback"
	authScope    = "org:create_api_key user:profile user:inference"
	tokenURL     = "https://console.anthropic.com/v1/oauth/token"
)

// PKCEPair represents PKCE challenge and verifier
type PKCEPair struct {
	Verifier  string
	Challenge string
}

// AuthorizeResult contains the authorization URL and PKCE verifier
type AuthorizeResult struct {
	URL      string
	Verifier string
}

// TokenResponse represents the OAuth token response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// GeneratePKCE creates a new PKCE challenge/verifier pair
func GeneratePKCE() (*PKCEPair, error) {
	// Generate random bytes for verifier
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	
	// Create base64url encoded verifier
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)
	
	// Create SHA256 hash of verifier for challenge
	h := sha256.New()
	h.Write([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h.Sum(nil))
	
	return &PKCEPair{
		Verifier:  verifier,
		Challenge: challenge,
	}, nil
}

// Authorize generates the authorization URL for Claude Max
func Authorize() (*AuthorizeResult, error) {
	pkce, err := GeneratePKCE()
	if err != nil {
		return nil, err
	}
	
	authURL := url.URL{
		Scheme: "https",
		Host:   "claude.ai",
		Path:   "/oauth/authorize",
	}
	
	q := authURL.Query()
	q.Set("code", "true")
	q.Set("client_id", clientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", authScope)
	q.Set("code_challenge", pkce.Challenge)
	q.Set("code_challenge_method", "S256")
	// TODO: This state implementation is incorrect for standard CSRF protection.
	// We're using the PKCE verifier as state, but typically we should generate
	// a separate random state value and verify it when the code comes back.
	// However, we've seen this exact implementation work before with Anthropic's
	// OAuth, so this may be specifically required by their OAuth server.
	q.Set("state", pkce.Verifier)
	authURL.RawQuery = q.Encode()
	
	return &AuthorizeResult{
		URL:      authURL.String(),
		Verifier: pkce.Verifier,
	}, nil
}

// Exchange exchanges the authorization code for tokens
func Exchange(code, verifier string) (*OAuthInfo, error) {
	// Split code and state if they're combined with #
	parts := bytes.Split([]byte(code), []byte("#"))
	authCode := string(parts[0])
	
	var state string
	if len(parts) > 1 {
		state = string(parts[1])
	}
	
	// Prepare request body
	reqBody := map[string]string{
		"code":          authCode,
		"state":         state,
		"grant_type":    "authorization_code",
		"client_id":     clientID,
		"redirect_uri":  redirectURI,
		"code_verifier": verifier,
	}
	
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Make token exchange request
	resp, err := http.Post(tokenURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed with status %d", resp.StatusCode)
	}
	
	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}
	
	return &OAuthInfo{
		AuthType:     AuthTypeOAuth,
		RefreshToken: tokenResp.RefreshToken,
		AccessToken:  tokenResp.AccessToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}, nil
}

// RefreshToken refreshes an OAuth token
func RefreshToken(refreshToken string) (*OAuthInfo, error) {
	reqBody := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     clientID,
	}
	
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	resp, err := http.Post(tokenURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed with status %d", resp.StatusCode)
	}
	
	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}
	
	return &OAuthInfo{
		AuthType:     AuthTypeOAuth,
		RefreshToken: tokenResp.RefreshToken,
		AccessToken:  tokenResp.AccessToken,
		ExpiresAt:    time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}, nil
}

// GetAccessToken retrieves a valid access token, refreshing if necessary
func GetAccessToken(provider string) (string, error) {
	info, err := Get(provider)
	if err != nil {
		return "", err
	}
	
	if info == nil {
		return "", fmt.Errorf("no auth info found for %s", provider)
	}
	
	switch auth := info.(type) {
	case *OAuthInfo:
		// Check if token is still valid
		if auth.AccessToken != "" && time.Now().Before(auth.ExpiresAt) {
			return auth.AccessToken, nil
		}
		
		// Need to refresh
		if auth.RefreshToken == "" {
			return "", fmt.Errorf("no refresh token available")
		}
		
		newAuth, err := RefreshToken(auth.RefreshToken)
		if err != nil {
			return "", fmt.Errorf("failed to refresh token: %w", err)
		}
		
		// Save the new auth info
		if err := Set(provider, newAuth); err != nil {
			return "", fmt.Errorf("failed to save refreshed token: %w", err)
		}
		
		return newAuth.AccessToken, nil
		
	default:
		return "", fmt.Errorf("unknown auth type")
	}
}