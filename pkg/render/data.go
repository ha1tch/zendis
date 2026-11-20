package render

import (
	"fmt"
	"strings"

	"github.com/ha1tch/zendis/pkg/disasm"
)

// DataRow is one line of a data record: the bytes it stands for and the
// directive that assembles to them.
type DataRow struct {
	Address uint16
	Bytes   []byte
	Text    string // "DB 0x3E,0x01", "DW 0x8000", `DB "SCORE",0x0D`, "DS 32,0x00"
	Fill    bool   // a DS row: every byte is Bytes[0]
}

// Widths of the rows, in bytes. A listing keeps numbers to the width of
// its hex column, the widest instruction; source has no such column.
const (
	listingRowBytes = 4
	sourceRowBytes  = 16
	textRowBytes    = 32
	listingTextRow  = 16
	minFillRun      = 4
)

// EffectiveDataKind is the kind a dialect writes a range in. zenas has
// every kind. sjasmplus has no compact hex form. pasmo and SNASM get
// bytes for everything: their DW, DS and strings were not tried against
// the assemblers themselves, and a DB of numbers is what both are known
// to take.
func EffectiveDataKind(kind disasm.DataKind, dialect Dialect) disasm.DataKind {
	switch dialect {
	case DialectPasmo, DialectSnasm:
		return disasm.DataBytes
	case DialectSjasmplus:
		if kind == disasm.DataHex {
			return disasm.DataBytes
		}
	}
	return kind
}

// DataRows splits a data record into the lines that write it in dialect.
// A row never runs across an address breakAt reports (a labelled one),
// so that the label can stand ahead of its own row; breakAt may be nil.
// listing asks for the narrow rows of the listing form. The rows'
// bytes, in order, are the record's.
func DataRows(rec Record, dialect Dialect, listing bool, breakAt func(uint16) bool) []DataRow {
	kind := EffectiveDataKind(rec.Kind, dialect)
	var rows []DataRow
	// Cut the record at the labelled addresses first; each piece is then
	// written by its kind on its own.
	start := 0
	for i := 1; i <= len(rec.Bytes); i++ {
		if i == len(rec.Bytes) || (breakAt != nil && breakAt(rec.Address+uint16(i))) {
			rows = append(rows, pieceRows(rec.Address+uint16(start), rec.Bytes[start:i], kind, listing)...)
			start = i
		}
	}
	return rows
}

func pieceRows(addr uint16, b []byte, kind disasm.DataKind, listing bool) []DataRow {
	width := sourceRowBytes
	if listing {
		width = listingRowBytes
	}
	switch kind {
	case disasm.DataWords:
		return chunkRows(addr, b, width, true, wordsText)
	case disasm.DataHex:
		return chunkRows(addr, b, width, false, func(c []byte) string { return "DB 0x " + hexDump(c) })
	case disasm.DataText:
		return textRows(addr, b, listing)
	case disasm.DataFill:
		return fillRows(addr, b, width)
	}
	return chunkRows(addr, b, width, false, bytesText)
}

func bytesText(c []byte) string { return "DB " + hexList(c) }

// wordsText writes pairs of bytes as little-endian words. chunkRows
// gives it whole words, or the one byte left over at the end of a piece
// of odd length, which is a DB.
func wordsText(c []byte) string {
	if len(c) == 1 {
		return bytesText(c)
	}
	parts := make([]string, 0, len(c)/2)
	for i := 0; i+1 < len(c); i += 2 {
		parts = append(parts, fmt.Sprintf("0x%04X", uint16(c[i])|uint16(c[i+1])<<8))
	}
	return "DW " + strings.Join(parts, ",")
}

// chunkRows cuts b into rows of width bytes. With words set a row holds
// whole words (width is even), and an odd byte left at the end is a row
// of its own.
func chunkRows(addr uint16, b []byte, width int, words bool, text func([]byte) string) []DataRow {
	var rows []DataRow
	for i := 0; i < len(b); {
		end := i + width
		if end > len(b) {
			end = len(b)
		}
		if n := end - i; words && n > 1 && n%2 == 1 {
			end-- // leave the odd byte for the next row
		}
		rows = append(rows, DataRow{Address: addr + uint16(i), Bytes: b[i:end], Text: text(b[i:end])})
		i = end
	}
	return rows
}

// printable reports whether c can stand in a quoted string in every
// assembler that gets strings: ASCII from space to tilde, less the quote
// and the backslash. Checked by assembling all of them with zenas and
// with sjasmplus.
func printable(c byte) bool {
	return c >= 0x20 && c <= 0x7E && c != '"' && c != '\\'
}

// textRows writes printable runs of two or more characters as strings
// and the other bytes as numbers. A row ends at its width, after the
// byte that closes a string (a zero, a carriage return or line feed, a
// byte with its top bit set, straight after its last character), and
// where a string begins after numbers, so that each string of a table
// has its own line. A row with no string in it is numbers only, and is
// cut to the width numbers have.
func textRows(addr uint16, b []byte, listing bool) []DataRow {
	width, numbers := textRowBytes, sourceRowBytes
	if listing {
		width, numbers = listingTextRow, listingRowBytes
	}
	inString := stringBytes(b)
	closes := func(c byte) bool { return c == 0x00 || c == 0x0D || c == 0x0A || c >= 0x80 }
	var rows []DataRow
	emit := func(start, end int) {
		hasString := false
		for i := start; i < end; i++ {
			hasString = hasString || inString[i]
		}
		if !hasString {
			rows = append(rows, chunkRows(addr+uint16(start), b[start:end], numbers, false, bytesText)...)
			return
		}
		rows = append(rows, DataRow{Address: addr + uint16(start), Bytes: b[start:end], Text: textText(b[start:end], inString[start:end])})
	}
	start := 0
	for i := 1; i <= len(b); i++ {
		closed := i-2 >= start && inString[i-2] && !inString[i-1] && closes(b[i-1])
		rowHasString := false
		for k := start; k < i && !rowHasString; k++ {
			rowHasString = inString[k]
		}
		begins := i < len(b) && inString[i] && !inString[i-1] && !rowHasString
		boundary := i < len(b) && (closed || begins)
		if i == len(b) || i-start == width || boundary {
			emit(start, i)
			start = i
		}
	}
	return rows
}

// stringBytes marks the bytes that are written inside quotes: those of a
// run of two or more printable characters.
func stringBytes(b []byte) []bool {
	in := make([]bool, len(b))
	for i := 0; i < len(b); {
		j := i
		for j < len(b) && printable(b[j]) {
			j++
		}
		if j-i >= 2 {
			for k := i; k < j; k++ {
				in[k] = true
			}
		}
		if j == i {
			j++
		}
		i = j
	}
	return in
}

func textText(c []byte, inString []bool) string {
	var parts []string
	for i := 0; i < len(c); {
		if inString[i] {
			j := i
			for j < len(c) && inString[j] {
				j++
			}
			parts = append(parts, `"`+string(c[i:j])+`"`)
			i = j
			continue
		}
		parts = append(parts, fmt.Sprintf("0x%02X", c[i]))
		i++
	}
	return "DB " + strings.Join(parts, ",")
}

// fillRows writes each run of minFillRun or more equal bytes as one DS
// and what lies between the runs as bytes.
func fillRows(addr uint16, b []byte, width int) []DataRow {
	var rows []DataRow
	plain := 0 // start of the bytes not yet written
	flush := func(end int) {
		if end > plain {
			rows = append(rows, chunkRows(addr+uint16(plain), b[plain:end], width, false, bytesText)...)
		}
	}
	for i := 0; i < len(b); {
		j := i
		for j < len(b) && b[j] == b[i] {
			j++
		}
		if j-i >= minFillRun {
			flush(i)
			rows = append(rows, DataRow{Address: addr + uint16(i), Bytes: b[i:j], Fill: true,
				Text: fmt.Sprintf("DS %d,0x%02X", j-i, b[i])})
			plain = j
		}
		i = j
	}
	flush(len(b))
	return rows
}

// rowHex is the hex column of a listing row: every byte, or for a DS row
// its first few and a mark that they go on.
func rowHex(r DataRow) string {
	if r.Fill && len(r.Bytes) > listingRowBytes {
		return hexDump(r.Bytes[:listingRowBytes]) + ".."
	}
	return hexDump(r.Bytes)
}
