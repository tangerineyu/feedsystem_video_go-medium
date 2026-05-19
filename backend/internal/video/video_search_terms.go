package video

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxSearchTermLength = 64

func buildVideoSearchTerms(parts ...string) []string {
	seen := make(map[string]struct{})
	terms := make([]string, 0)

	for _, part := range parts {
		for _, token := range splitSearchText(part) {
			if utf8.RuneCountInString(token) > maxSearchTermLength {
				token = string([]rune(token)[:maxSearchTermLength])
			}
			if token == "" {
				continue
			}
			if _, ok := seen[token]; ok {
				continue
			}
			seen[token] = struct{}{}
			terms = append(terms, token)
		}
	}

	return terms
}

func splitSearchText(text string) []string {
	return strings.FieldsFunc(strings.ToLower(strings.TrimSpace(text)), func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
	})
}
