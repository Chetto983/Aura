package filecard

import "strings"

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

// rowText labels each value with the card's header for its column. A column without one,
// in a headerless sheet or past the header's width, takes the letter Excel shows for it,
// so the label is still something the operator can find.
func rowText(headers, values []string) string {
	pairs := make([]string, 0, len(values))
	for i, value := range values {
		value = clean(value)
		if value == "" {
			continue
		}
		label := columnLetter(i)
		if i < len(headers) && headers[i] != "" {
			label = headers[i]
		}
		pairs = append(pairs, label+": "+value)
	}
	return strings.Join(pairs, "; ")
}

// columnLetter is the inverse of columnIndex: 0 is A, 25 is Z, 26 is AA.
func columnLetter(index int) string {
	var letters []byte
	for index++; index > 0; index = (index - 1) / 26 {
		letters = append([]byte{byte('A' + (index-1)%26)}, letters...)
	}
	return string(letters)
}

// ShownWhole reports whether the card already shows every data row of the named sheet:
// each row it scanned is one of its samples, and nothing in the card was cut to fit.
func (c Card) ShownWhole(sheet string) bool {
	if _, _, truncated := c.layout(); truncated {
		return false
	}
	for _, s := range c.Sheets {
		if s.Name == sheet {
			return s.RowsScanned > 0 && s.RowsScanned == len(s.Samples)
		}
	}
	return false
}
