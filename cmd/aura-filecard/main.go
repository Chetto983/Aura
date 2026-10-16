// aura-filecard renders one file's card on stdout.
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
	asJSON := flags.Bool("json", false, "emit {\"card\":...,\"description\":...} instead of the rendered text")
	if err := flags.Parse(args); err != nil || flags.NArg() != 1 {
		_, _ = fmt.Fprintln(stderr, "usage: aura-filecard [-name NAME] [-json] PATH")
		return 2
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
		// The ingest stores the card and embeds the description (prd.md §11, 2026-10-16).
		err = json.NewEncoder(stdout).Encode(map[string]string{
			"card": card.Render(), "description": card.Description(),
		})
	} else {
		_, err = fmt.Fprint(stdout, card.Render())
	}
	if err != nil {
		return failed(stderr, err)
	}
	return 0
}

func failed(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintf(stderr, "aura-filecard: %v\n", err)
	return 1
}
