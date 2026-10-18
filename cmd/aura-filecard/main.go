// aura-filecard renders one file's card on stdout, or with -source-extensions the list of
// extensions filecard cards as source files.
//
// It exists so the ingest sidecar can build cards WITHOUT a second implementation of
// filecard. The card is what makes a spreadsheet answerable: measured on a 500-row,
// 3-sheet workbook it finds the real header under a merged banner, types every column,
// and reports value distributions -- "Citta: most common Torino (98), Milano (94)" --
// which is exactly the aggregate that scores 0% at every k when asked of passages
// (internal/documents/open.go). A document ingested from the bucket had none of it.
//
// A subprocess rather than an HTTP endpoint, deliberately. Aura's cockpit auth makes only
// GET /healthz public and restricts /readyz to loopback probes; a card endpoint would be a
// new unauthenticated surface on the same server for something that needs no store, no
// identity and no network. services/ingest/extract.py already shells out to `soffice` in
// this same image, so this follows a pattern that is there rather than adding a protocol.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/chetto1983/aura/internal/documents/filecard"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("aura-filecard", flag.ContinueOnError)
	flags.SetOutput(stderr)
	name := flags.String("name", "", "file name to describe the document as (defaults to the path's base name)")
	asJSON := flags.Bool("json", false,
		"emit {\"card\",\"description\",\"rows\"} as JSON instead of the rendered text")
	sources := flags.Bool("source-extensions", false, "print the source-file extensions as a JSON list and exit")
	parseErr := flags.Parse(args)
	wantArgs := 1
	if *sources {
		wantArgs = 0
	}
	if parseErr != nil || flags.NArg() != wantArgs {
		_, _ = fmt.Fprintln(stderr, "usage: aura-filecard [-name NAME] [-json] PATH | -source-extensions")
		return 2
	}
	if *sources {
		// services/ingest reads its list of source extensions from here: there is one copy.
		if err := writeJSON(stdout, filecard.SourceExtensions()); err != nil {
			return failed(stderr, err)
		}
		return 0
	}
	path := flags.Arg(0)
	info, err := os.Stat(path)
	if err != nil {
		return failed(stderr, err)
	}
	fileName := *name
	if fileName == "" {
		fileName = info.Name()
	}

	// A build error is NOT fatal, matching Service.writeCard: a file that could not be read
	// says so in its own card, because a card is how a document is FOUND and refusing to
	// describe a truncated zip would trade a real loss for a small one. The reason goes to
	// stderr so the caller can log it.
	card, buildErr := filecard.Build(filecard.Request{
		Path: path, FileName: fileName, SizeBytes: info.Size(),
	})
	if buildErr != nil {
		_, _ = fmt.Fprintf(stderr, "aura-filecard: %s: %v\n", fileName, buildErr)
	}
	if *asJSON {
		err = writeJSON(stdout, rendered(card))
	} else {
		_, err = fmt.Fprint(stdout, card.Render())
	}
	if err != nil {
		return failed(stderr, err)
	}
	return 0
}

// rendered is what the ingest reads (prd.md §11): it stores the card, embeds the description
// (2026-10-16), turns a workbook's rows into passages, and keeps the label-value pairs above
// its tables for the document fields. rows and header are empty rather than null, so a reader
// never has to tell a missing field from an empty one.
func rendered(card filecard.Card) map[string]any {
	rows := card.Rows
	if rows == nil {
		rows = []filecard.Row{}
	}
	header := []filecard.HeaderPair{}
	for _, sheet := range card.Sheets {
		header = append(header, sheet.Header...)
	}
	return map[string]any{"card": card.Render(), "description": card.Description(), "rows": rows, "header": header}
}

// writeJSON leaves <, > and & as they are: the reader is the ingest, not a browser, and
// encoding/json's HTML escaping writes each as a six-byte \u escape, so a row full of them
// grows up to 6x.
func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func failed(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintf(stderr, "aura-filecard: %v\n", err)
	return 1
}
