package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/pkg/browser"
	"reapo/internal/auth"
	"reapo/internal/logger"
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
			return SetProcessingMsg{
				Text:   fmt.Sprintf("Failed to generate auth URL: %v", err),
				Active: true,
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
			return AuthFlowCompleteMsg{
				Success: false,
				Message: "No authentication configured.",
			}
		}
	}
	
	// Log the logout attempt
	logger.Info("Logging out user")
	
	// Remove auth immediately
	if err := auth.Remove("anthropic"); err != nil {
		logger.Error("Failed to logout", "error", err)
		return func() tea.Msg {
			return AuthFlowCompleteMsg{
				Success: false,
				Message: fmt.Sprintf("Failed to logout: %v", err),
			}
		}
	}
	
	logger.Info("Successfully logged out")
	
	return func() tea.Msg {
		return AuthFlowCompleteMsg{
			Success: true,
			Message: "Successfully logged out.",
		}
	}
}

func (m Model) handleAuthCode(code string, verifier string) tea.Cmd {
	// Update processing text
	cmds := []tea.Cmd{
		func() tea.Msg {
			return SetProcessingMsg{
				Text:   "Exchanging authorization code...",
				Active: true,
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
			return AuthFlowCompleteMsg{
				Success: false,
				Message: fmt.Sprintf("Failed to exchange code: %v", err),
			}
		}
		
		logger.Info("OAuth exchange successful",
			"has_access_token", oauthInfo.AccessToken != "",
			"has_refresh_token", oauthInfo.RefreshToken != "",
			"expires_at", oauthInfo.ExpiresAt)
		
		// Save auth info
		if err := auth.Set("anthropic", oauthInfo); err != nil {
			logger.Error("Failed to save auth info", "error", err)
			return AuthFlowCompleteMsg{
				Success: false,
				Message: fmt.Sprintf("Failed to save auth: %v", err),
			}
		}
		
		logger.Info("Auth info saved successfully", "provider", "anthropic")
		
		return AuthFlowCompleteMsg{
			Success: true,
			Message: "Successfully logged in with Claude Max!",
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