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
	"unicode/utf8"

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
	Match(text *Text) bool
}

// Text is one document's text as the matchers read it. It is split into words once per case mode
// however many rules read it: a pass tokenizing a 1 MB document once per rule spent 58 ms a rule
// doing nothing else (measured 2026-10-18). Not safe for concurrent use.
type Text struct {
	raw   string
	words [2][]string
	sets  [2]map[string]struct{}
	done  [2]bool
}

// NewText wraps a document's text for the matchers.
func NewText(raw string) *Text { return &Text{raw: raw} }

func (t *Text) tokens(insensitive bool) ([]string, map[string]struct{}) {
	mode := 0
	if insensitive {
		mode = 1
	}
	if !t.done[mode] {
		words := tokens(t.raw, insensitive)
		set := make(map[string]struct{}, len(words))
		for _, w := range words {
			set[w] = struct{}{}
		}
		t.words[mode], t.sets[mode], t.done[mode] = words, set, true
	}
	return t.words[mode], t.sets[mode]
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

func (never) Match(*Text) bool { return false }

type regexMatcher struct{ re *regexp.Regexp }

func (m regexMatcher) Match(text *Text) bool { return m.re.MatchString(text.raw) }

type wordMatcher struct {
	alg         Algorithm
	words       []string
	insensitive bool
}

func (m wordMatcher) Match(text *Text) bool {
	words, set := text.tokens(m.insensitive)
	present := func(w string) bool { _, ok := set[w]; return ok }
	switch m.alg {
	case Any:
		return slices.ContainsFunc(m.words, present)
	case All:
		return !slices.ContainsFunc(m.words, func(w string) bool { return !present(w) })
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
// word order does not count and one typo does not break a match. An edit distance is at least the
// difference in length, so a window whose length alone keeps it under the threshold is skipped
// before any distance is computed.
func (m wordMatcher) fuzzy(words []string) bool {
	k := len(m.words)
	if len(words) < k {
		return false
	}
	want := sortedPhrase(m.words)
	wantLen := utf8.RuneCountInString(want)
	lengths := make([]int, len(words)+1)
	for i, w := range words {
		lengths[i+1] = lengths[i] + utf8.RuneCountInString(w)
	}
	for i := 0; i+k <= len(words); i++ {
		gotLen := lengths[i+k] - lengths[i] + k - 1
		longest := max(wantLen, gotLen)
		if float64(max(wantLen-gotLen, gotLen-wantLen)) > (1-fuzzyThreshold)*float64(longest) {
			continue
		}
		got := sortedPhrase(words[i : i+k])
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
