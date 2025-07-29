package completion

import (
	"os"
	"path/filepath"
	"strings"
)

const maxCompletionItems = 50

func GetFileCompletions(workingDir, query string) []CompletionItem {
	var items []CompletionItem

	// Walk the entire directory tree from working directory
	err := filepath.WalkDir(workingDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		// Skip the root directory itself
		if path == workingDir {
			return nil
		}

		// Skip hidden files and directories unless explicitly requested
		if strings.HasPrefix(d.Name(), ".") && !strings.HasPrefix(query, ".") {
			if d.IsDir() {
				return filepath.SkipDir // Skip entire hidden directory
			}
			return nil // Skip hidden file
		}

		// Skip .git directory contents
		if strings.Contains(path, ".git") {
			if d.IsDir() && d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		// Calculate relative path from working directory
		relPath, err := filepath.Rel(workingDir, path)
		if err != nil {
			return nil // Skip if we can't get relative path
		}

		// Add trailing slash for directories
		completionText := relPath
		if d.IsDir() {
			completionText += "/"
		}

		items = append(items, CompletionItem{
			Text:        completionText,
			Description: "",
		})

		// Limit results to prevent overwhelming the UI
		if len(items) >= maxCompletionItems {
			return filepath.SkipAll // Stop walking entirely
		}

		return nil
	})

	if err != nil {
		return nil
	}

	// Apply fuzzy matching if there's a query
	if query != "" {
		items = FuzzyMatch(query, items)
	}

	return items
}
