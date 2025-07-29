package completion

import (
	"strings"
)

var slashCommands = []CompletionItem{
	{Text: "/help", Description: "Show all available commands"},
	{Text: "/clear", Description: "Clear conversation context"},
	{Text: "/editor", Description: "Open external editor ($EDITOR)"},
}

type CompletionEngine struct {
	workingDir string
	commands   []CompletionItem
}

func NewCompletionEngine(workingDir string) *CompletionEngine {
	return &CompletionEngine{
		workingDir: workingDir,
		commands:   slashCommands,
	}
}

func (e *CompletionEngine) GetWorkingDir() string {
	return e.workingDir
}

func (e *CompletionEngine) GetCompletions(trigger rune, query string) []CompletionItem {
	switch trigger {
	case '/':
		return e.getSlashCompletions(query)
	case '@':
		return e.getFileCompletions(query)
	}
	return nil
}

func (e *CompletionEngine) getSlashCompletions(query string) []CompletionItem {
	// Remove the leading '/' from query if present
	if strings.HasPrefix(query, "/") {
		query = query[1:]
	}

	return FuzzyMatch(query, e.commands)
}

func (e *CompletionEngine) getFileCompletions(query string) []CompletionItem {
	return GetFileCompletions(e.workingDir, query)
}
