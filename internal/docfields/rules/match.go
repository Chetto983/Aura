// Package rules matches a document's text and file name against the rule an operator put on a
// type, a counterparty or a tag. The algorithms are Paperless-ngx's behaviours, read from its
// documentation and re-implemented here (Paperless is GPL-3.0, no code is copied):
// docs/paperless-ngx-behaviours-2026-10-17.md.
package rules

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/agnivade/levenshtein"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Algorithm names how a label's pattern is matched.
type Algorithm string

// The algorithms, Paperless-ngx's set: None never matches, so a label without a rule is only set
// by hand.
const (
	None  Algorithm = "none"
	Any   Algorithm = "any"
	All   Algorithm = "all"
	Exact Algorithm = "exact"
	Regex Algorithm = "regex"
	Fuzzy Algorithm = "fuzzy"
)

// fuzzyThreshold is the similarity a window of the text must reach against the pattern. Paperless's
// own threshold is not copied (its code is GPL); this is the plan's ruling, to be tuned on the
// office set.
const fuzzyThreshold = 0.8

// ErrInvalidPattern refuses a rule that cannot match anything, or a regex that does not parse.
var ErrInvalidPattern = errors.New("invalid matching pattern")

// Matcher reports whether a text matches a compiled rule.
type Matcher interface {
	Match(text string) bool
}

// Compile validates a rule once, when the operator saves it, so a pass never meets a broken one.
// RE2 has no backtracking, so no pattern can stall a pass.
func Compile(alg Algorithm, pattern string, caseInsensitive bool) (Matcher, error) {
	switch alg {
	case None:
		return never{}, nil
	case Regex:
		if strings.TrimSpace(pattern) == "" {
			return nil, fmt.Errorf("%w: a regex rule needs a pattern", ErrInvalidPattern)
		}
		expr := pattern
		if caseInsensitive {
			expr = "(?i)" + expr
		}
		re, err := regexp.Compile(expr)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidPattern, err)
		}
		return regexMatcher{re}, nil
	case Any, All, Exact, Fuzzy:
		words := tokens(pattern, caseInsensitive)
		if len(words) == 0 {
			return nil, fmt.Errorf("%w: a %s rule needs at least one word", ErrInvalidPattern, alg)
		}
		return wordMatcher{alg: alg, words: words, insensitive: caseInsensitive}, nil
	default:
		return nil, fmt.Errorf("%w: unknown algorithm %q", ErrInvalidPattern, alg)
	}
}

type never struct{}

func (never) Match(string) bool { return false }

type regexMatcher struct{ re *regexp.Regexp }

func (m regexMatcher) Match(text string) bool { return m.re.MatchString(text) }

type wordMatcher struct {
	alg         Algorithm
	words       []string
	insensitive bool
}

func (m wordMatcher) Match(text string) bool {
	words := tokens(text, m.insensitive)
	switch m.alg {
	case Any:
		return slices.ContainsFunc(m.words, func(w string) bool { return slices.Contains(words, w) })
	case All:
		return !slices.ContainsFunc(m.words, func(w string) bool { return !slices.Contains(words, w) })
	case Exact:
		for i := 0; i+len(m.words) <= len(words); i++ {
			if slices.Equal(words[i:i+len(m.words)], m.words) {
				return true
			}
		}
		return false
	default:
		return m.fuzzy(words)
	}
}

// fuzzy compares the pattern with every window of the text as long as the pattern, both sorted, so
// word order does not count and one typo does not break a match.
func (m wordMatcher) fuzzy(words []string) bool {
	want := sortedPhrase(m.words)
	for i := 0; i+len(m.words) <= len(words); i++ {
		got := sortedPhrase(words[i : i+len(m.words)])
		longest := max(len([]rune(want)), len([]rune(got)))
		if 1-float64(levenshtein.ComputeDistance(want, got))/float64(longest) >= fuzzyThreshold {
			return true
		}
	}
	return false
}

func sortedPhrase(words []string) string {
	sorted := slices.Clone(words)
	slices.Sort(sorted)
	return strings.Join(sorted, " ")
}

// tokens splits on anything that is not a letter or a digit. Insensitive matching also folds case
// and accents, so "citta" finds "Città".
func tokens(text string, insensitive bool) []string {
	if insensitive {
		folded, _, err := transform.String(transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC), text)
		if err == nil {
			text = folded
		}
		text = strings.ToLower(text)
	}
	return strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
}
