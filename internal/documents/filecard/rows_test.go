package filecard

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func complexCorpus(name string) string {
	return filepath.Join("..", "..", "..", "scripts", "fixtures", "document_retrieval_eval", "complex_corpus", name)
}

type sheetPart struct{ name, xml string }

// workbookWith lays out a workbook whose sheets come in the order given, each from its whole
// worksheet XML. sharedStrings is the string table, or "" for none.
func workbookWith(t *testing.T, name, sharedStrings string, sheets ...sheetPart) string {
	t.Helper()
	var listed, rels strings.Builder
	parts := map[string]string{}
	for i, sheet := range sheets {
		fmt.Fprintf(&listed, `<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, sheet.name, i+1, i+1)
		fmt.Fprintf(&rels, `<Relationship Id="rId%d" Type="worksheet" Target="worksheets/sheet%d.xml"/>`, i+1, i+1)
		parts[fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1)] = sheet.xml
	}
	parts["xl/workbook.xml"] = "<workbook><sheets>" + listed.String() + "</sheets></workbook>"
	parts["xl/_rels/workbook.xml.rels"] = "<Relationships>" + rels.String() + "</Relationships>"
	if sharedStrings != "" {
		parts["xl/sharedStrings.xml"] = sharedStrings
	}
	return writeZip(t, name, parts)
}

func sheetOf(sheetData string) string {
	return "<worksheet><sheetData>" + sheetData + "</sheetData></worksheet>"
}

// oneSheetWorkbook wraps a worksheet's <sheetData> body in the parts a workbook needs.
func oneSheetWorkbook(t *testing.T, name, sheet, sheetData string) string {
	t.Helper()
	return workbookWith(t, name, "", sheetPart{sheet, sheetOf(sheetData)})
}

func inline(ref, value string) string {
	return fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, ref, value)
}

func rowsOf(card Card, sheet string) []Row {
	var rows []Row
	for _, row := range card.Rows {
		if row.Sheet == sheet {
			rows = append(rows, row)
		}
	}
	return rows
}

func rowNumbered(t *testing.T, rows []Row, number int64) Row {
	t.Helper()
	for _, row := range rows {
		if row.Number == number {
			return row
		}
	}
	t.Fatalf("no row %d among %+v", number, rows)
	return Row{}
}

// Ricavi carries a title on row 1 and "Trimestre" on row 2 above its header on row 3. A
// citation has to name the row an operator finds in Excel, and neither banner is data.
func TestRowsCarryTheWorksheetRowNumber(t *testing.T) {
	card := build(t, complexCorpus("budget_consolidato_2026.xlsx"), "budget_consolidato_2026.xlsx")
	rows := rowsOf(card, "Ricavi")

	if len(rows) != 5 {
		t.Fatalf("Ricavi rows = %+v, want the 5 under the header", rows)
	}
	if rows[0].Number != 4 || rows[len(rows)-1].Number != 8 {
		t.Errorf("Ricavi rows run %d..%d, want 4..8", rows[0].Number, rows[len(rows)-1].Number)
	}
	centro := rowNumbered(t, rows, 6)
	if !strings.HasPrefix(centro.Text, "Area: Centro; T1: 1972; T2: 1921") {
		t.Errorf("row 6 = %q, want Centro under the card's headers", centro.Text)
	}
	for _, row := range rows {
		if strings.Contains(row.Text, "Trimestre") || strings.Contains(row.Text, "Ricavi 2026") {
			t.Errorf("a banner became a data row: %+v", row)
		}
	}
}

func TestRowsUseTheCardHeaders(t *testing.T) {
	card := build(t, complexCorpus("magazzino_ricambi_2026.xlsx"), "magazzino_ricambi_2026.xlsx")
	first := rowNumbered(t, rowsOf(card, "Giacenze"), 2)
	if want := "Codice: RIC-10000; Descrizione: Cuscinetto rinforzato 29 mm; Ubicazione: S7-F02; " +
		"Fornitore: F01; Ragione sociale: Meccanica Bassi S.r.l.; Giacenza: 4; Scorta minima: 22; " +
		"Da riordinare: SI"; first.Text != want {
		t.Errorf("row 2 = %q\nwant      %q", first.Text, want)
	}

	long := strings.Repeat("guarnizione in viton ", 6)
	path := oneSheetWorkbook(t, "note.xlsx", "Note",
		`<row r="1">`+inline("A1", "Codice")+inline("B1", "Nota")+inline("C1", "Citta")+`</row>`+
			`<row r="2">`+inline("A2", "K1")+inline("B2", "  "+long+"\n")+inline("C2", "TORINO")+`</row>`+
			`<row r="3">`+inline("A3", "K2")+inline("C3", "GENOVA")+`</row>`)
	rows := rowsOf(build(t, path, "note.xlsx"), "Note")
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want the 2 under the header", rows)
	}
	if want := "Codice: K1; Nota: " + strings.TrimSpace(long) + "; Citta: TORINO"; rows[0].Text != want {
		t.Errorf("a long value was not kept whole and clean:\n got %q\nwant %q", rows[0].Text, want)
	}
	if rows[1].Text != "Codice: K2; Citta: GENOVA" {
		t.Errorf("an empty cell was not skipped: %q", rows[1].Text)
	}

	// A shared string written as formatted runs arrives in pieces; every piece is the value.
	run := strings.Repeat("sostituire la guarnizione ", 5)
	richPath := workbookWith(t, "ricco.xlsx",
		`<sst><si><t>Nota</t></si><si>`+
			`<r><rPr><b/></rPr><t>`+run+`</t></r><r><t>`+run+`</t></r><r><t>`+run+`</t></r>`+
			`<r><t>`+run+`fine</t></r></si></sst>`,
		sheetPart{"Ricco", sheetOf(`<row r="1"><c r="A1" t="s"><v>0</v></c></row>` +
			`<row r="2"><c r="A2" t="s"><v>1</v></c></row>`)})
	rich := rowsOf(build(t, richPath, "ricco.xlsx"), "Ricco")
	if want := "Nota: " + clean(strings.Repeat(run, 4)+"fine"); len(rich) != 1 || rich[0].Text != want {
		t.Errorf("a value in formatted runs was cut:\n got %+v\nwant %q", rich, want)
	}
}

// Riordino has no header row, which is what the card says. Its cells are labelled with the
// column letters the operator sees, and a gap keeps the letter of the cell's own column.
func TestAHeaderlessSheetLabelsCellsByColumnLetter(t *testing.T) {
	card := build(t, complexCorpus("magazzino_ricambi_2026.xlsx"), "magazzino_ricambi_2026.xlsx")
	riordino := rowsOf(card, "Riordino")
	if got := rowNumbered(t, riordino, 9).Text; got != "A: RIC-10175" {
		t.Errorf("row 9 = %q, want A: RIC-10175", got)
	}
	if got := rowNumbered(t, riordino, 2).Text; got != "A: RIC-10000; E: F01; F: 39" {
		t.Errorf("row 2 = %q, want its gap to keep the letters E and F", got)
	}

	var data strings.Builder
	data.WriteString(`<row r="1">` + inline("A1", "Codice") + inline("B1", "Nota") + `</row>`)
	for i := 2; i <= headerProbeRows+1; i++ {
		fmt.Fprintf(&data, `<row r="%d">%s%s</row>`, i, inline(fmt.Sprintf("A%d", i), "K"), inline(fmt.Sprintf("B%d", i), "n"))
	}
	// Past the probe a row can be wider than the header; AB is past it, and BM is the 65th
	// column, past what a card reads.
	data.WriteString(`<row r="20">` + inline("A20", "K20") + inline("AB20", "extra") + inline("BM20", "dropped") + `</row>`)
	rows := rowsOf(build(t, oneSheetWorkbook(t, "largo.xlsx", "Largo", data.String()), "largo.xlsx"), "Largo")
	if got := rowNumbered(t, rows, 20).Text; got != "Codice: K20; AB: extra" {
		t.Errorf("row 20 = %q, want a column past the headers named by its letter and BM dropped", got)
	}
}

func TestColumnLettersAreTheOnesExcelShows(t *testing.T) {
	for index, want := range map[int]string{0: "A", 1: "B", 25: "Z", 26: "AA", 27: "AB", 51: "AZ", 52: "BA", 63: "BL", 701: "ZZ", 702: "AAA"} {
		if got := columnLetter(index); got != want {
			t.Errorf("columnLetter(%d) = %q, want %q", index, got, want)
		}
	}
	for index := range 2000 {
		if back := columnIndex(columnLetter(index) + "1"); back != index {
			t.Fatalf("columnIndex(columnLetter(%d)) = %d", index, back)
		}
	}
}

// The store's Ragione sociale is a VLOOKUP and Da riordinare an IF. A row is what the file
// shows, so it carries the values LibreOffice cached, never the formulas.
func TestRowsKeepCachedFormulaValues(t *testing.T) {
	card := build(t, complexCorpus("magazzino_ricambi_2026.xlsx"), "magazzino_ricambi_2026.xlsx")
	text := rowNumbered(t, rowsOf(card, "Giacenze"), 2).Text
	for _, want := range []string{"Ragione sociale: Meccanica Bassi S.r.l.", "Da riordinare: SI"} {
		if !strings.Contains(text, want) {
			t.Errorf("row 2 = %q, missing %q", text, want)
		}
	}
	if strings.Contains(text, "VLOOKUP") || strings.Contains(text, "IF(") {
		t.Errorf("a formula leaked into the row: %q", text)
	}
}

// A sheet shorter than the probe window resolves its header when the sheet ends. The rows
// held back until then keep their own positions, gaps included.
func TestProbeRowsKeepTheirPositions(t *testing.T) {
	path := oneSheetWorkbook(t, "corto.xlsx", "Corto",
		`<row r="2">`+inline("A2", "Elenco fornitori 2026")+`</row>`+
			`<row r="4">`+inline("A4", "Codice")+inline("B4", "Citta")+`</row>`+
			`<row r="5">`+inline("A5", "F01")+inline("B5", "Brescia")+`</row>`+
			`<row r="7">`+inline("A7", "F02")+inline("B7", "Bergamo")+`</row>`)
	card := build(t, path, "corto.xlsx")
	rows := rowsOf(card, "Corto")
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want the 2 under the header", rows)
	}
	if rows[0] != (Row{Sheet: "Corto", Number: 5, Text: "Codice: F01; Citta: Brescia"}) ||
		rows[1] != (Row{Sheet: "Corto", Number: 7, Text: "Codice: F02; Citta: Bergamo"}) {
		t.Errorf("rows = %+v, want rows 5 and 7 under Codice and Citta", rows)
	}
	if card.Sheets[0].HeaderRow != 4 {
		t.Errorf("header row = %d, want 4", card.Sheets[0].HeaderRow)
	}
}

func TestRowsStopAtTheRowCap(t *testing.T) {
	var data strings.Builder
	data.WriteString(`<row r="1">` + inline("A1", "Codice") + inline("B1", "Importo") + `</row>`)
	last := maxRowsScanned + 50
	for i := 2; i <= last; i++ {
		fmt.Fprintf(&data, `<row r="%d">%s<c r="B%d"><v>%d</v></c></row>`, i, inline(fmt.Sprintf("A%d", i), fmt.Sprintf("K%06d", i)), i, i)
	}
	path := workbookWith(t, "grande.xlsx", "", sheetPart{"Grande",
		fmt.Sprintf(`<worksheet><dimension ref="A1:B%d"/><sheetData>%s</sheetData></worksheet>`, last, data.String())})
	card := build(t, path, "grande.xlsx")

	if len(card.Rows) != maxRowsScanned-1 || len(card.Rows) != card.Sheets[0].RowsScanned {
		t.Fatalf("rows = %d, want the %d data rows the card scanned", len(card.Rows), card.Sheets[0].RowsScanned)
	}
	if final := card.Rows[len(card.Rows)-1]; final.Number != maxRowsScanned {
		t.Errorf("the last row is %d, want the cap's row %d", final.Number, maxRowsScanned)
	}
	if !strings.Contains(card.Render(), `Sheet "Grande" was sampled`) {
		t.Errorf("a capped sheet's card no longer says it was sampled:\n%s", card.Render())
	}
}

// The ingest extracts a delimited file as text, so its rows must not arrive a second time.
func TestADelimitedFileKeepsNoRows(t *testing.T) {
	card := build(t, writeFile(t, "piccolo.csv", "Codice,Citta\nF01,Brescia\n"), "piccolo.csv")
	if len(card.Rows) != 0 {
		t.Errorf("a CSV kept rows: %+v", card.Rows)
	}
}

func TestShownWhole(t *testing.T) {
	budget := build(t, complexCorpus("budget_consolidato_2026.xlsx"), "budget_consolidato_2026.xlsx")
	if !budget.ShownWhole("Ricavi") {
		t.Errorf("Ricavi's 5 data rows are all in the card, yet it is not shown whole")
	}
	if budget.ShownWhole("Costi") {
		t.Errorf("Costi has 6 data rows and its card shows 5, yet it is shown whole")
	}
	if budget.ShownWhole("Nessuno") {
		t.Errorf("a sheet the workbook does not have is shown whole")
	}

	store := build(t, complexCorpus("magazzino_ricambi_2026.xlsx"), "magazzino_ricambi_2026.xlsx")
	if store.ShownWhole("Giacenze") {
		t.Errorf("Giacenze has 1,500 rows, yet it is shown whole")
	}
	// Fornitori's 5 rows are all samples, but the card was cut to fit.
	if !strings.Contains(store.Render(), cardCutNotice) {
		t.Fatalf("the store's card is no longer cut, so this case tests nothing:\n%s", store.Render())
	}
	if store.ShownWhole("Fornitori") {
		t.Errorf("a sheet of a card that was cut is shown whole")
	}

	empty := build(t, oneSheetWorkbook(t, "vuoto.xlsx", "Vuoto", ""), "vuoto.xlsx")
	if empty.ShownWhole("Vuoto") {
		t.Errorf("a sheet with no data rows is shown whole")
	}

	// The card shows a value of 200 runes cut at 72, so the reader has not seen that row whole.
	data := `<row r="1">` + inline("A1", "Codice") + inline("B1", "Nota") + `</row>`
	for i := 2; i <= sampleRows+1; i++ {
		note := "breve"
		if i == 4 {
			note = strings.Repeat("ò", 200)
		}
		data += fmt.Sprintf(`<row r="%d">%s%s</row>`, i, inline(fmt.Sprintf("A%d", i), fmt.Sprintf("K%d", i)),
			inline(fmt.Sprintf("B%d", i), note))
	}
	long := build(t, oneSheetWorkbook(t, "lunga.xlsx", "Lunga", data), "lunga.xlsx")
	if long.Sheets[0].RowsScanned != len(long.Sheets[0].Samples) || strings.Contains(long.Render(), cardCutNotice) {
		t.Fatalf("every row of Lunga should be a sample of an uncut card:\n%s", long.Render())
	}
	if long.ShownWhole("Lunga") {
		t.Errorf("a sheet whose card cut one of its values is shown whole")
	}
}

// A cell holds an index into the string table, so one long string can be referenced by every
// cell of every row, and no read cap bounds the row text that makes. The workbook's rows share
// one budget, and past it the rows that follow, in this sheet and the next, are left out.
func TestRowTextStopsAtTheWorkbookBudget(t *testing.T) {
	big := strings.Repeat("ricambio ", (1<<20)/9)
	data := `<row r="1">` + inline("A1", "Codice") + inline("B1", "Nota") + inline("C1", "Altra nota") + `</row>`
	for i := 2; i <= 21; i++ {
		data += fmt.Sprintf(`<row r="%d">%s<c r="B%d" t="s"><v>0</v></c><c r="C%d" t="s"><v>0</v></c></row>`,
			i, inline(fmt.Sprintf("A%d", i), fmt.Sprintf("K%02d", i)), i, i)
	}
	small := sheetOf(`<row r="1">` + inline("A1", "Codice") + `</row><row r="2">` + inline("A2", "F01") + `</row>`)
	path := workbookWith(t, "pesante.xlsx", `<sst><si><t>`+big+`</t></si></sst>`,
		sheetPart{"Note", sheetOf(data)}, sheetPart{"Piccolo", small})

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	card := build(t, path, "pesante.xlsx")
	runtime.GC()
	runtime.ReadMemStats(&after)

	kept := 0
	for _, row := range card.Rows {
		kept += len(row.Text)
	}
	note := rowsOf(card, "Note")
	if kept > maxRowTextBytes || len(note) == 0 || len(note) == 20 {
		t.Fatalf("kept %d rows of Note, %d bytes, want some but not all, within %d bytes", len(note), kept, maxRowTextBytes)
	}
	if retained := int64(after.HeapAlloc) - int64(before.HeapAlloc); retained > maxRowTextBytes+4<<20 {
		t.Errorf("the card holds %d MiB, past the %d MiB budget", retained>>20, maxRowTextBytes>>20)
	}
	caveat := fmt.Sprintf(`Rows from row %d of sheet "Note" onward`, note[len(note)-1].Number+1)
	if !strings.Contains(card.Render(), caveat) {
		t.Errorf("the card does not say where its rows stopped (%q):\n%s", caveat, card.Render())
	}
	if rows := rowsOf(card, "Piccolo"); len(rows) != 0 {
		t.Errorf("a sheet after the budget kept rows: %+v", rows)
	}
	if card.ShownWhole("Piccolo") {
		t.Errorf("a sheet whose rows were left out is shown whole")
	}
	alone := build(t, workbookWith(t, "piccolo.xlsx", "", sheetPart{"Piccolo", small}), "piccolo.xlsx")
	if !alone.ShownWhole("Piccolo") {
		t.Errorf("Piccolo on its own is not shown whole, so the case above tests nothing")
	}
}

// Checking a row against the budget must not build it: the budget is there because building
// is what runs out of memory.
func TestARowThatDoesNotFitIsNotBuilt(t *testing.T) {
	huge := strings.Repeat("x", 64<<20)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	text, fits := rowText([]string{"Nota"}, []string{huge}, 1<<20)
	runtime.ReadMemStats(&after)
	if fits || text != "" {
		t.Fatalf("a 64 MiB row fit in 1 MiB")
	}
	if grew := after.TotalAlloc - before.TotalAlloc; grew > 1<<20 {
		t.Errorf("checking the row allocated %d bytes", grew)
	}
	if text, fits := rowText([]string{"Nota"}, []string{"abc"}, len("Nota: abc")); !fits || text != "Nota: abc" {
		t.Errorf("a row exactly the size of the room = %q, %v", text, fits)
	}
	if _, fits := rowText([]string{"Nota"}, []string{"abc"}, len("Nota: abc")-1); fits {
		t.Errorf("a row one byte over the room fit")
	}
}
