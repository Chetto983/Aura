package rules

import (
	"errors"
	"strings"
	"testing"
)

func TestMatchers(t *testing.T) {
	cases := []struct {
		name        string
		alg         Algorithm
		pattern     string
		insensitive bool
		text        string
		want        bool
	}{
		{"any: one word is enough", Any, "fattura nota", true, "Questa è una Fattura del 2026", true},
		{"any: no word", Any, "contratto ordine", true, "Questa è una fattura", false},
		{"any: a word inside a longer word is not the word", Any, "nota", true, "notaio di Milano", false},
		{"all: every word, any order", All, "gialli import", true, "IMPORT da Gialli srl", true},
		{"all: one missing", All, "gialli import export", true, "Gialli Import", false},
		{"exact: the phrase", Exact, "Gialli Import", true, "cliente: gialli import, Genova", true},
		{"exact: same words, other order", Exact, "Gialli Import", true, "import gialli", false},
		{"exact: whole words only", Exact, "Gialli Import", true, "Giallissimi Importatori", false},
		{"case-sensitive refuses another case", Exact, "ACME", false, "acme spa", false},
		{"case-sensitive accepts its own case", Exact, "ACME", false, "ACME SpA", true},
		{"accents fold when insensitive", Any, "citta", true, "La Città di Roma", true},
		{"accents stay when sensitive", Any, "citta", false, "la città", false},
		{"regex: RE2", Regex, `fattura\s+n\.?\s*\d{4}_\d{3}`, true, "Fattura n. 2026_007", true},
		{"regex: no match", Regex, `^nota`, true, "una nota", false},
		{"fuzzy: one typo", Fuzzy, "Gialli Import", true, "fattura per Gialli Imprt srl", true},
		{"fuzzy: order does not matter", Fuzzy, "Gialli Import", true, "Import Gialli", true},
		{"fuzzy: too far", Fuzzy, "Gialli Import", true, "Rossi Export", false},
		{"none never matches", None, "", true, "anything at all", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Compile(tc.alg, tc.pattern, tc.insensitive)
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			if got := m.Match(tc.text); got != tc.want {
				t.Fatalf("Match(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

// A regex rule that does not parse is refused when it is saved, with the parser's own
// message, so the operator can fix it; it never reaches a pass (Review Focus 5).
func TestInvalidRegexRefused(t *testing.T) {
	_, err := Compile(Regex, `fattura(`, true)
	if !errors.Is(err, ErrInvalidPattern) {
		t.Fatalf("err = %v, want ErrInvalidPattern", err)
	}
	if !strings.Contains(err.Error(), "missing closing )") {
		t.Fatalf("err = %q, want the RE2 parse error", err)
	}
}

func TestCompileRefusesWhatCannotMatch(t *testing.T) {
	for _, tc := range []struct {
		alg     Algorithm
		pattern string
	}{
		{Any, "   "},
		{All, ""},
		{Exact, "--"},
		{Fuzzy, ""},
		{Regex, ""},
		{Algorithm("soundex"), "x"},
	} {
		if _, err := Compile(tc.alg, tc.pattern, true); !errors.Is(err, ErrInvalidPattern) {
			t.Errorf("Compile(%q, %q) err = %v, want ErrInvalidPattern", tc.alg, tc.pattern, err)
		}
	}
}
