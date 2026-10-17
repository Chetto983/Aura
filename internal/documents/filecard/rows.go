package filecard

import (
	"fmt"
	"strings"
)

// Row is one data row of a worksheet, written as "Header: value; Header: value".
//
// Number is the worksheet's own row number, the one an operator finds in Excel, so a
// citation can be checked against the file. Values are whole: the card's 72-rune cut is for
// a summary, and a lookup needs the value as the file holds it.
type Row struct {
	Sheet  string `json:"sheet"`
	Number int64  `json:"row"`
	Text   string `json:"text"`
}

// rowBudget is the room left for row text in one workbook. The read caps do not bound it: a
// cell holds an index into the string table, so one long string can be referenced by every
// cell of every row.
type rowBudget struct {
	left      int
	exhausted bool
	// stoppedSheet and stoppedRow name the first row left out.
	stoppedSheet string
	stoppedRow   int64
}

func newRowBudget() *rowBudget {
	return &rowBudget{left: maxRowTextBytes}
}

func (b *rowBudget) caveat() string {
	return fmt.Sprintf("Rows from row %d of sheet %q onward, and those of the sheets after it, are not kept "+
		"for search: the workbook's row text reached its %d MiB budget.", b.stoppedRow, b.stoppedSheet, maxRowTextBytes>>20)
}

// keepRow keeps a data row while the workbook's budget has room for it. The first row that
// does not fit stops keeping for the rest of the workbook, so the rows kept are a prefix and
// the caveat can say where they end.
func (t *tableBuilder) keepRow(values []string, position int64) {
	if !t.budget.exhausted {
		if text, fits := rowText(t.headers, values, t.budget.left); fits {
			t.budget.left -= len(text)
			t.rows = append(t.rows, Row{Sheet: t.name, Number: position, Text: text})
			return
		}
		t.budget.exhausted, t.budget.stoppedSheet, t.budget.stoppedRow = true, t.name, position
	}
	t.rowsLeftOut = true
}

// rowText labels each value with the card's header for its column. A column without one,
// in a headerless sheet or past the header's width, takes the letter Excel shows for it,
// so the label is still something the operator can find.
//
// It returns false, having built nothing, when the text could be longer than room. A raw
// value is never shorter than its cleaned form, so the check needs no copy of the value.
func rowText(headers, values []string, room int) (string, bool) {
	size := 0
	for i, value := range values {
		if value != "" {
			size += len(cellLabel(headers, i)) + len(": ") + len(value) + len("; ")
		}
	}
	if size-len("; ") > room {
		return "", false
	}
	pairs := make([]string, 0, len(values))
	for i, value := range values {
		if value = clean(value); value != "" {
			pairs = append(pairs, cellLabel(headers, i)+": "+value)
		}
	}
	return strings.Join(pairs, "; "), true
}

func cellLabel(headers []string, index int) string {
	if index < len(headers) && headers[index] != "" {
		return headers[index]
	}
	return columnLetter(index)
}

// columnLetter is the inverse of columnIndex: 0 is A, 25 is Z, 26 is AA.
func columnLetter(index int) string {
	var letters []byte
	for index++; index > 0; index = (index - 1) / 26 {
		letters = append([]byte{byte('A' + (index-1)%26)}, letters...)
	}
	return string(letters)
}

// ShownWhole reports whether the card already shows every data row of the named sheet whole:
// each row it scanned is one of its samples, no sample value was cut, every row was kept, and
// nothing in the card was cut to fit.
func (c Card) ShownWhole(sheet string) bool {
	if _, _, truncated := c.layout(); truncated {
		return false
	}
	for _, s := range c.Sheets {
		if s.Name == sheet {
			return s.RowsScanned > 0 && s.RowsScanned == len(s.Samples) && !s.samplesCut && !s.rowsLeftOut
		}
	}
	return false
}
