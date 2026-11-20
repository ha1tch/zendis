package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ha1tch/zendis/pkg/disasm"
)

func rowTexts(rows []DataRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Text
	}
	return out
}

// rowsCover checks that the rows' bytes, in order and at their
// addresses, are the record's: whatever the kind, nothing is lost.
func rowsCover(t *testing.T, rec Record, rows []DataRow) {
	t.Helper()
	var all []byte
	addr := rec.Address
	for _, r := range rows {
		if r.Address != addr {
			t.Errorf("row at %04X, want %04X", r.Address, addr)
		}
		all = append(all, r.Bytes...)
		addr += uint16(len(r.Bytes))
	}
	if !bytes.Equal(all, rec.Bytes) {
		t.Errorf("rows hold % X, the record % X", all, rec.Bytes)
	}
}

func TestDataRowsKinds(t *testing.T) {
	cases := []struct {
		name  string
		kind  disasm.DataKind
		bytes []byte
		want  []string
	}{
		{"bytes", disasm.DataBytes, []byte{1, 2, 3}, []string{"DB 0x01,0x02,0x03"}},
		{"hex", disasm.DataHex, []byte{0x3E, 0x01, 0xC9}, []string{"DB 0x 3E 01 C9"}},
		{"words", disasm.DataWords, []byte{0x00, 0x80, 0x34, 0x12}, []string{"DW 0x8000,0x1234"}},
		{"words, odd", disasm.DataWords, []byte{0x00, 0x80, 0x07}, []string{"DW 0x8000", "DB 0x07"}},
		{"text", disasm.DataText, []byte("SCORE:\x00HI\xD3"), []string{`DB "SCORE:",0x00`, `DB "HI",0xD3`}},
		{"text, quote and lone character", disasm.DataText, []byte("AB\"C\\"), []string{`DB "AB",0x22,0x43,0x5C`}},
		{"text, numbers then a string", disasm.DataText, []byte{1, 2, 'O', 'K', 0}, []string{"DB 0x01,0x02", `DB "OK",0x00`}},
		{"fill", disasm.DataFill, []byte{9, 0, 0, 0, 0, 0, 7, 7, 7}, []string{"DB 0x09", "DS 5,0x00", "DB 0x07,0x07,0x07"}},
	}
	for _, c := range cases {
		rec := Record{Address: 0x9000, Bytes: c.bytes, Kind: c.kind}
		rows := DataRows(rec, DialectZenas, false, nil)
		if got := rowTexts(rows); strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
		rowsCover(t, rec, rows)
		rowsCover(t, rec, DataRows(rec, DialectZenas, true, nil))
	}
}

// TestDataRowsDialects: pasmo and SNASM get numbers for every kind, and
// sjasmplus for the compact hex it does not have.
func TestDataRowsDialects(t *testing.T) {
	rec := Record{Address: 0x9000, Bytes: []byte("HI\x00"), Kind: disasm.DataText}
	for _, d := range []Dialect{DialectPasmo, DialectSnasm} {
		if got := rowTexts(DataRows(rec, d, false, nil)); len(got) != 1 || got[0] != "DB 0x48,0x49,0x00" {
			t.Errorf("%s: %q", d, got)
		}
	}
	if got := rowTexts(DataRows(rec, DialectSjasmplus, false, nil)); got[0] != `DB "HI",0x00` {
		t.Errorf("sjasmplus text: %q", got)
	}
	rec.Kind = disasm.DataHex
	if got := rowTexts(DataRows(rec, DialectSjasmplus, false, nil)); got[0] != "DB 0x48,0x49,0x00" {
		t.Errorf("sjasmplus hex: %q", got)
	}
}

// TestDataRowsBreakAtLabels: a row never runs across a labelled
// address, and the source declares the label ahead of its row.
func TestDataRowsBreakAtLabels(t *testing.T) {
	code := []byte{0x00, 0x80, 0x10, 0x80, 0x20, 0x80}
	hints := &disasm.Hints{
		Labels: map[uint16]string{0x9000: "table", 0x9004: "last"},
		Data:   []disasm.DataRange{{Start: 0x9000, Length: 6, Kind: disasm.DataWords}},
	}
	d := NewDisassembly(code, 0x9000, disasm.TargetZ80, hints, true)
	var out bytes.Buffer
	d.WriteAssemblySource(&out, DialectZenas, nil)
	want := "ORG 0x9000\ntable:\nDW 0x8000,0x8010\nlast:\nDW 0x8020\n"
	if out.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", out.String(), want)
	}
}

func TestListingDataRows(t *testing.T) {
	code := append([]byte("HELLO WORLD\x00"), make([]byte, 40)...)
	hints := &disasm.Hints{Data: []disasm.DataRange{
		{Start: 0x9000, Length: 12, Kind: disasm.DataText},
		{Start: 0x900C, Length: 40, Kind: disasm.DataFill},
	}}
	d := NewDisassembly(code, 0x9000, disasm.TargetZ80, hints, true)
	var out bytes.Buffer
	d.WriteListing(&out, false, nil)
	want := "9000:  48 45 4C 4C 4F 20 57 4F 52 4C 44 00 DB \"HELLO WORLD\",0x00\n900C:  00 00 00 00.. DS 40,0x00\n"
	if out.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", out.String(), want)
	}
}
