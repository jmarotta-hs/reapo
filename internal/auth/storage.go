package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
	
	"reapo/internal/logger"
)

var (
	storageMutex sync.RWMutex
	storageFile  string
)

func init() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("failed to get home directory: %v", err))
	}
	
	// Create ~/.local/share/reapo directory if it doesn't exist
	dataDir := filepath.Join(homeDir, ".local", "share", "reapo")
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		panic(fmt.Sprintf("failed to create reapo data directory: %v", err))
	}
	
	storageFile = filepath.Join(dataDir, "auth.json")
	logger.Info("Auth storage initialized", "storage_file", storageFile, "data_dir", dataDir)
}

// Get retrieves auth info for a specific provider
func Get(provider string) (AuthInfo, error) {
	storageMutex.RLock()
	defer storageMutex.RUnlock()
	
	data, err := readStorage()
	if err != nil {
		return nil, err
	}
	
	rawAuth, exists := data[provider]
	if !exists {
		return nil, nil
	}
	
	return parseAuthInfo(rawAuth)
}

// Set stores auth info for a specific provider
func Set(provider string, info AuthInfo) error {
	storageMutex.Lock()
	defer storageMutex.Unlock()
	
	logger.Info("Set auth info called", "provider", provider, "storage_file", storageFile)
	
	data, err := readStorage()
	if err != nil && !os.IsNotExist(err) {
		logger.Error("Failed to read storage", "error", err)
		return err
	}
	
	if data == nil {
		data = make(Storage)
	}
	
	// Convert AuthInfo to a map for JSON marshaling
	var authMap map[string]interface{}
	
	switch auth := info.(type) {
	case *OAuthInfo:
		authMap = map[string]interface{}{
			"type":    string(auth.AuthType),
			"refresh": auth.RefreshToken,
			"access":  auth.AccessToken,
			"expires": auth.ExpiresAt.Unix(),
		}
		logger.Info("Created auth map for OAuth", "auth_type", auth.AuthType)
	default:
		return fmt.Errorf("unknown auth type")
	}
	
	data[provider] = authMap
	
	logger.Info("Writing storage data", "providers", len(data))
	return writeStorage(data)
}

// Remove deletes auth info for a specific provider
func Remove(provider string) error {
	storageMutex.Lock()
	defer storageMutex.Unlock()
	
	data, err := readStorage()
	if err != nil {
		return err
	}
	
	delete(data, provider)
	
	return writeStorage(data)
}

// All returns all stored auth info
func All() (map[string]AuthInfo, error) {
	storageMutex.RLock()
	defer storageMutex.RUnlock()
	
	data, err := readStorage()
	if err != nil {
		return nil, err
	}
	
	result := make(map[string]AuthInfo)
	for provider, rawAuth := range data {
		info, err := parseAuthInfo(rawAuth)
		if err != nil {
			continue // Skip invalid entries
		}
		result[provider] = info
	}
	
	return result, nil
}

// Helper functions

func readStorage() (Storage, error) {
	file, err := os.Open(storageFile)
	if err != nil {
		if os.IsNotExist(err) {
			return make(Storage), nil
		}
		return nil, err
	}
	defer file.Close()
	
	var data Storage
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}
	
	return data, nil
}

func writeStorage(data Storage) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		logger.Error("Failed to marshal storage data", "error", err)
		return err
	}
	
	logger.Info("Writing auth data", "size", len(jsonData), "file", storageFile)
	
	// Write to temp file first
	tempFile := storageFile + ".tmp"
	if err := os.WriteFile(tempFile, jsonData, 0600); err != nil {
		logger.Error("Failed to write temp file", "tempFile", tempFile, "error", err)
		return err
	}
	
	// Rename to actual file (atomic operation)
	if err := os.Rename(tempFile, storageFile); err != nil {
		logger.Error("Failed to rename temp file", "tempFile", tempFile, "targetFile", storageFile, "error", err)
		return err
	}
	
	logger.Info("Successfully wrote auth file", "file", storageFile)
	return nil
}

func parseAuthInfo(raw interface{}) (AuthInfo, error) {
	authMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid auth data format")
	}
	
	authType, ok := authMap["type"].(string)
	if !ok {
		return nil, fmt.Errorf("missing auth type")
	}
	
	switch AuthType(authType) {
	case AuthTypeOAuth:
		refresh, _ := authMap["refresh"].(string)
		access, _ := authMap["access"].(string)
		
		var expiresAt time.Time
		if expires, ok := authMap["expires"].(float64); ok {
			expiresAt = time.Unix(int64(expires), 0)
		}
		
		return &OAuthInfo{
			AuthType:     AuthTypeOAuth,
			RefreshToken: refresh,
			AccessToken:  access,
			ExpiresAt:    expiresAt,
		}, nil
		
	default:
		return nil, fmt.Errorf("unknown auth type: %s", authType)
	}
}