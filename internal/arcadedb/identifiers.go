package arcadedb

import (
	"strings"
	"unicode"
)

// QueryIdentifiers are the query's exact technical names: a token holding a letter and
// a digit or an underscore. Similarity cannot establish an exact technical identity, so
// memory recall admits a candidate only when it carries every one of them, including when
// ranking fell back to lexical search. Ordinary words and purely numeric dates are not
// identifiers.
func QueryIdentifiers(query string) []string {
	var identifiers []string
	for _, token := range identifierTokens(query) {
		var letter, digit bool
		for _, symbol := range token {
			letter = letter || unicode.IsLetter(symbol)
			digit = digit || unicode.IsDigit(symbol)
		}
		if letter && (digit || strings.ContainsRune(token, '_')) {
			identifiers = append(identifiers, token)
		}
	}
	return identifiers
}

func identifierTokens(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(symbol rune) bool {
		return !unicode.IsLetter(symbol) && !unicode.IsDigit(symbol) &&
			!unicode.IsMark(symbol) && symbol != '_' && symbol != '-'
	})
}

// IdentifiersMatch reports whether texts carry every identifier as a whole token; no
// identifiers match anything.
func IdentifiersMatch(identifiers []string, texts ...string) bool {
	if len(identifiers) == 0 {
		return true
	}
	available := make(map[string]struct{})
	for _, text := range texts {
		for _, token := range identifierTokens(text) {
			available[token] = struct{}{}
		}
	}
	for _, identifier := range identifiers {
		if _, found := available[identifier]; !found {
			return false
		}
	}
	return true
}
