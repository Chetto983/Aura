package gateway

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// maxApprovalLineRunes bounds one line of operator-visible approval text (spec 2026-10-12).
const maxApprovalLineRunes = 120

// approvalStripped are the characters that frame markup and quoted text.
const approvalStripped = "<>&\"`"

// approvalLine is the one sanitizer for operator-visible approval text that comes from the
// model or from stored data: argument keys and the scope labels in the request the model
// relays, and a tool's description of its target, which only the operator sees. Line breaks
// and other whitespace become single spaces, control and invisible format characters (bidi
// overrides, zero-width and tag characters) and approvalStripped are removed, and the result
// is cut to 120 runes. It folds with NFKC first, as the untrusted envelope does, so a
// fullwidth ＜ goes the way of <.
//
// It is structural: it stops markup and line forgery. It is not an injection defence, and the
// words it keeps can still say anything.
func approvalLine(s string) string {
	mapped := strings.Map(func(r rune) rune {
		switch {
		case unicode.IsSpace(r):
			return ' '
		case unicode.IsControl(r), unicode.Is(unicode.Cf, r), strings.ContainsRune(approvalStripped, r):
			return -1
		}
		return r
	}, norm.NFKC.String(s))
	line := strings.Join(strings.Fields(mapped), " ")
	if runes := []rune(line); len(runes) > maxApprovalLineRunes {
		line = strings.TrimRight(string(runes[:maxApprovalLineRunes]), " ")
	}
	return line
}
