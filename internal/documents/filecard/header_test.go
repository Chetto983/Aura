package filecard

import (
	"path/filepath"
	"slices"
	"testing"
)

// An invoice workbook as generate_corpus.py writes it (spikes/document-routing-benchmark,
// write_invoice): five label-value rows, a blank, then the line-item table. The pairs were read
// as a banner and dropped, so the client and the VAT number reached nothing.
func TestInvoiceHeaderPairsKept(t *testing.T) {
	path := filepath.Join("..", "..", "..", "scripts", "fixtures", "document_retrieval_eval", "corpus", "fattura_2026_001.xlsx")
	card := build(t, path, "fattura_2026_001.xlsx")
	if len(card.Sheets) != 1 {
		t.Fatalf("sheets = %d", len(card.Sheets))
	}
	sheet := card.Sheets[0]
	want := []HeaderPair{
		{Sheet: "fattura", Key: "Fattura n.", Value: "2026_001"},
		{Sheet: "fattura", Key: "Cliente", Value: "ACME SpA"},
		{Sheet: "fattura", Key: "Partita IVA", Value: "IT01234567890"},
		{Sheet: "fattura", Key: "Data emissione", Value: "2026-01-12"},
		{Sheet: "fattura", Key: "Stato", Value: "pagata"},
	}
	if !slices.Equal(sheet.Header, want) {
		t.Fatalf("header = %+v, want %+v", sheet.Header, want)
	}
	var columns []string
	for _, column := range sheet.Columns {
		columns = append(columns, column.Header)
	}
	if !slices.Equal(columns, []string{"Descrizione", "Quantita", "Prezzo unitario", "Importo"}) || sheet.HeaderRow != 7 {
		t.Fatalf("columns %v on row %d: the table under the pairs moved", columns, sheet.HeaderRow)
	}
	var numbers []int64
	for _, row := range card.Rows {
		numbers = append(numbers, row.Number)
	}
	if !slices.Equal(numbers, []int64{8, 10, 11, 12}) {
		t.Fatalf("rows %v: the pairs must not become rows", numbers)
	}
}

// A one-cell banner, a row of numbers and a three-cell row are not a label and its value.
func TestOnlyALabelAndItsValueIsAPair(t *testing.T) {
	path := oneSheetWorkbook(t, "fornitori.xlsx", "elenco",
		`<row r="1">`+inline("A1", "Elenco fornitori 2025")+`</row>`+
			`<row r="2"><c r="A2"><v>2025</v></c><c r="B2"><v>12</v></c></row>`+
			`<row r="3">`+inline("A3", "Cliente:")+inline("B3", "Rossi SRL")+`</row>`+
			`<row r="4">`+inline("A4", "Nota")+inline("B4", "uno")+inline("C4", "due")+`</row>`+
			`<row r="5">`+inline("A5", "Fornitore")+inline("B5", "Citta")+inline("C5", "Totale")+inline("D5", "Stato")+`</row>`+
			`<row r="6">`+inline("A6", "SIEMENS")+inline("B6", "Milano")+`<c r="C6"><v>573</v></c>`+inline("D6", "attivo")+`</row>`)
	card := build(t, path, "fornitori.xlsx")
	want := []HeaderPair{{Sheet: "elenco", Key: "Cliente", Value: "Rossi SRL"}}
	if got := card.Sheets[0].Header; !slices.Equal(got, want) {
		t.Fatalf("header = %+v, want %+v", got, want)
	}
}
