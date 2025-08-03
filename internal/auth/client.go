package auth

import (
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// NewClient creates an authenticated Anthropic client
// Priority: Claude Max (OAuth) > Environment variable
func NewClient() (anthropic.Client, error) {
	// First, try to get OAuth token for Claude Max
	token, err := GetAccessToken("anthropic")
	if err == nil && token != "" {
		// Use OAuth token
		return anthropic.NewClient(
			option.WithHeader("Authorization", fmt.Sprintf("Bearer %s", token)),
		), nil
	}
	
	// Fall back to environment variable
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey != "" {
		return anthropic.NewClient(), nil // SDK will use env var automatically
	}
	
	return anthropic.Client{}, fmt.Errorf("no authentication method available. Please run /login or set ANTHROPIC_API_KEY")
}

// GetAuthStatus returns the current authentication status
func GetAuthStatus() string {
	// Check OAuth
	token, err := GetAccessToken("anthropic")
	if err == nil && token != "" {
		return "Claude Max (OAuth)"
	}
	
	// Check environment variable
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return "Environment Variable"
	}
	
	return "Not authenticated"
}