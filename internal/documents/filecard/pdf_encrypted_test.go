package filecard

import (
	"strings"
	"testing"
)

// An encrypted PDF's Info strings are ciphertext: a SICK datasheet, RC4 with an owner password
// only, opened anywhere and carded as "Titled: H\x8f\xa2z…" (measured 2026-10-16, pdfinfo reads
// "W100 WT100-P1432, Data sheet"). The card must not describe a file by its ciphertext.

const encryptedPDFTrailer = "trailer<</Size 10/Encrypt 9 0 R/Info 1 0 R/Root 4 0 R>>\n%%EOF\n"

func TestAnEncryptedPDFsInfoStringsAreNotRead(t *testing.T) {
	path := writeFile(t, "scheda.pdf", "%PDF-1.4\n"+
		"1 0 obj<</Title <8fa27a3dd7b7f8c3><>/Subject <4f99c13e>/Author <4cbe2be8>>>endobj\n"+
		"9 0 obj<</Filter/Standard/V 2/R 3/Length 128/P -1068>>endobj\n"+encryptedPDFTrailer)

	card := build(t, path, "scheda.pdf")
	rendered := card.Render()
	if card.Title != "" {
		t.Errorf("title = %q: read from ciphertext", card.Title)
	}
	for _, line := range []string{"Subject:", "Author:", "Titled:"} {
		if strings.Contains(rendered, line) {
			t.Errorf("the card carries %q from ciphertext:\n%s", line, rendered)
		}
	}
}

func TestAnEncryptedPDFStillTakesItsPlaintextXMP(t *testing.T) {
	// /EncryptMetadata false leaves the XMP packet readable, and then it is the truth.
	path := writeFile(t, "scheda.pdf", "%PDF-1.4\n"+
		"1 0 obj<</Title <8fa27a3dd7b7f8c3>>>endobj\n"+
		"<x:xmpmeta xmlns:x=\"adobe:ns:meta/\"><rdf:RDF><rdf:Description>"+
		"<dc:title><rdf:Alt><rdf:li xml:lang=\"x-default\">W100 WT100-P1432, Data sheet</rdf:li></rdf:Alt></dc:title>"+
		"</rdf:Description></rdf:RDF></x:xmpmeta>\n"+
		"9 0 obj<</Filter/Standard/V 2/R 3/EncryptMetadata false>>endobj\n"+encryptedPDFTrailer)

	if got := build(t, path, "scheda.pdf").Title; got != "W100 WT100-P1432, Data sheet" {
		t.Fatalf("title = %q, want the plaintext XMP title", got)
	}
}
