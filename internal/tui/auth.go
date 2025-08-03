package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkg/browser"
	"reapo/internal/auth"
	"reapo/internal/logger"
	"reapo/internal/tui/components"
)

// Auth flow message types

type SetProcessingMsg struct {
	Text   string
	Active bool
}


type AuthFlowCompleteMsg struct {
	Success bool
	Message string
}

// Auth flow methods

func (m Model) startLoginFlow() tea.Cmd {
	// Start OAuth flow directly
	authResult, err := auth.Authorize()
	if err != nil {
		return func() tea.Msg {
			return ShowStatuslineMsg{
				Type:     components.StatuslineError,
				Text:     fmt.Sprintf("Error: Failed to generate auth URL: %v", err),
				Duration: 6 * time.Second,
			}
		}
	}
	
	// Store verifier
	verifier := authResult.Verifier
	
	// Try to open browser
	browserErr := browser.OpenURL(authResult.URL)
	
	// Return a sequence of messages instead of batch to ensure order
	return func() tea.Msg {
		return StoreVerifierAndShowModalMsg{
			Verifier:      verifier,
			URL:           authResult.URL,
			BrowserOpened: browserErr == nil,
		}
	}
}

func (m Model) startLogoutFlow() tea.Cmd {
	// Get all auth info
	authInfo, err := auth.All()
	if err != nil || len(authInfo) == 0 {
		return func() tea.Msg {
			return ShowStatuslineMsg{
				Type:     components.StatuslineWarning,
				Text:     "Warning: No authentication configured",
				Duration: 4 * time.Second,
			}
		}
	}
	
	// Log the logout attempt
	logger.Info("Logging out user")
	
	// Remove auth immediately
	if err := auth.Remove("anthropic"); err != nil {
		logger.Error("Failed to logout", "error", err)
		return func() tea.Msg {
			return ShowStatuslineMsg{
				Type:     components.StatuslineError,
				Text:     fmt.Sprintf("Error: Failed to logout: %v", err),
				Duration: 6 * time.Second,
			}
		}
	}
	
	logger.Info("Successfully logged out")
	
	return func() tea.Msg {
		return AuthFlowCompleteMsg{
			Success: true,
			Message: "Successfully logged out",
		}
	}
}

func (m Model) handleAuthCode(code string, verifier string) tea.Cmd {
	// Show exchanging message immediately
	cmds := []tea.Cmd{
		func() tea.Msg {
			return ShowStatuslineMsg{
				Type:     components.StatuslineInfo,
				Text:     "Exchanging authorization code...",
				Duration: 0, // No duration - will be replaced by next message
			}
		},
	}
	
	// Exchange code for tokens in a separate goroutine
	cmds = append(cmds, func() tea.Msg {
		// Log the exchange attempt
		logger.Info("Attempting OAuth code exchange", 
			"code_length", len(code),
			"verifier_length", len(verifier))
		
		oauthInfo, err := auth.Exchange(code, verifier)
		if err != nil {
			logger.Error("OAuth exchange failed", "error", err)
			// Parse specific error types
			errorMsg := "Error: Authentication failed"
			if len(code) == 0 {
				errorMsg = "Error: Authentication cancelled by user"
			} else if err.Error() == "token exchange failed with status 400" {
				errorMsg = "Error: Authentication failed: Invalid authorization code"
			} else if err.Error() == "token exchange failed with status 401" {
				errorMsg = "Error: Authentication failed: Authorization expired"
			} else if err.Error() == "token exchange failed with status 403" {
				errorMsg = "Error: Authentication failed: Access denied"
			} else if strings.Contains(err.Error(), "failed to exchange code") {
				errorMsg = "Error: Authentication failed: Network error"
			} else {
				errorMsg = fmt.Sprintf("Error: Authentication failed: %v", err)
			}
			
			return ShowStatuslineMsg{
				Type:     components.StatuslineError,
				Text:     errorMsg,
				Duration: 6 * time.Second,
			}
		}
		
		logger.Info("OAuth exchange successful",
			"has_access_token", oauthInfo.AccessToken != "",
			"has_refresh_token", oauthInfo.RefreshToken != "",
			"expires_at", oauthInfo.ExpiresAt)
		
		// Save auth info
		if err := auth.Set("anthropic", oauthInfo); err != nil {
			logger.Error("Failed to save auth info", "error", err)
			return ShowStatuslineMsg{
				Type:     components.StatuslineError,
				Text:     fmt.Sprintf("Error: Failed to save authentication: %v", err),
				Duration: 6 * time.Second,
			}
		}
		
		logger.Info("Auth info saved successfully", "provider", "anthropic")
		
		return AuthFlowCompleteMsg{
			Success: true,
			Message: "Successfully authenticated with Claude Max",
		}
	})
	
	return tea.Batch(cmds...)
}




// Helper message types
type ShowAuthModalMsg struct {
	URL           string
	BrowserOpened bool
}

type StoreVerifierAndShowModalMsg struct {
	Verifier      string
	URL           string
	BrowserOpened bool
}