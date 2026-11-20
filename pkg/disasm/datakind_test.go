package disasm

import (
	"strings"
	"testing"
)

func TestParseHintsDataKinds(t *testing.T) {
	h, err := ParseHints(strings.NewReader("data 0x9000 16\ndata 0x9010 8 words\ndata 0x9018 32 TEXT\ndata 0x9038 4 hex\ndata 0x903C 64 fill\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := []DataKind{DataBytes, DataWords, DataText, DataHex, DataFill}
	if len(h.Data) != len(want) {
		t.Fatalf("%d ranges", len(h.Data))
	}
	for i, k := range want {
		if h.Data[i].Kind != k {
			t.Errorf("range %d: %v, want %v", i, h.Data[i].Kind, k)
		}
	}
	for _, bad := range []string{"data 0x9000 4 strings\n", "data 0x9000 4 words extra\n"} {
		if _, err := ParseHints(strings.NewReader(bad)); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}

// TestDataAt: the first range given wins where two overlap, and the
// later, larger one goes on after it.
func TestDataAt(t *testing.T) {
	h := &Hints{Data: []DataRange{
		{Start: 0x9004, Length: 4, Kind: DataWords},
		{Start: 0x9000, Length: 0x10, Kind: DataText},
	}}
	for _, c := range []struct {
		addr uint16
		want DataRange
		ok   bool
	}{
		{0x9000, DataRange{Start: 0x9000, Length: 4, Kind: DataText}, true},
		{0x9002, DataRange{Start: 0x9002, Length: 2, Kind: DataText}, true},
		{0x9004, DataRange{Start: 0x9004, Length: 4, Kind: DataWords}, true},
		{0x9008, DataRange{Start: 0x9008, Length: 8, Kind: DataText}, true},
		{0x9010, DataRange{}, false},
	} {
		if got, ok := h.DataAt(c.addr); got != c.want || ok != c.ok {
			t.Errorf("%04X: %+v %v, want %+v %v", c.addr, got, ok, c.want, c.ok)
		}
	}
	// A range that ends at the top of the address space.
	top := &Hints{Data: []DataRange{{Start: 0xFFF0, Length: 0x10}}}
	if got, ok := top.DataAt(0xFFFF); !ok || got.Length != 1 {
		t.Errorf("FFFF: %+v %v", got, ok)
	}
}

func TestParseDataRange(t *testing.T) {
	for _, c := range []struct {
		in   string
		want DataRange
	}{
		{"0x9000:16", DataRange{Start: 0x9000, Length: 16}},
		{"36864:0x20:words", DataRange{Start: 0x9000, Length: 0x20, Kind: DataWords}},
		{" 0x9000 : 8 : Text ", DataRange{Start: 0x9000, Length: 8, Kind: DataText}},
	} {
		if got, err := ParseDataRange(c.in); err != nil || got != c.want {
			t.Errorf("%q: %+v %v, want %+v", c.in, got, err, c.want)
		}
	}
	for _, bad := range []string{"0x9000", "0x9000:4:strings", "x:4", "0x9000:4:words:more"} {
		if _, err := ParseDataRange(bad); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}
