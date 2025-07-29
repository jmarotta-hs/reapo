package completion

import (
	"sort"
	"strings"
	"unicode"
)

type fuzzyMatch struct {
	item  CompletionItem
	score int
}

func FuzzyMatch(query string, items []CompletionItem) []CompletionItem {
	if query == "" {
		return items
	}

	query = strings.ToLower(query)
	var matches []fuzzyMatch

	for _, item := range items {
		score := calculateScore(query, strings.ToLower(item.Text))
		if score > 0 {
			matches = append(matches, fuzzyMatch{
				item:  item,
				score: score,
			})
		}
	}

	// Sort by score (highest first)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})

	// Extract items from matches
	result := make([]CompletionItem, len(matches))
	for i, match := range matches {
		result[i] = match.item
		result[i].Score = match.score
	}

	return result
}

func calculateScore(query, text string) int {
	if len(query) == 0 {
		return 100
	}
	if len(text) == 0 {
		return 0
	}

	// Exact match gets highest score
	if query == text {
		return 1000
	}

	// Prefix match gets high score
	if strings.HasPrefix(text, query) {
		return 800 + (100 - len(text)) // Prefer shorter matches
	}

	// Fuzzy matching
	score := 0
	queryIdx := 0
	lastMatchIdx := -1
	consecutiveMatches := 0

	for textIdx, char := range text {
		if queryIdx >= len(query) {
			break
		}

		if unicode.ToLower(char) == unicode.ToLower(rune(query[queryIdx])) {
			score += 10

			// Bonus for consecutive matches
			if textIdx == lastMatchIdx+1 {
				consecutiveMatches++
				score += consecutiveMatches * 5
			} else {
				consecutiveMatches = 0
			}

			// Bonus for word boundary matches
			if textIdx == 0 || !unicode.IsLetter(rune(text[textIdx-1])) {
				score += 15
			}

			lastMatchIdx = textIdx
			queryIdx++
		}
	}

	// Must match all characters in query
	if queryIdx < len(query) {
		return 0
	}

	// Penalty for length difference
	lengthPenalty := len(text) - len(query)
	if lengthPenalty > 0 {
		score -= lengthPenalty
	}

	return max(score, 1)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
