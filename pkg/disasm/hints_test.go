package disasm

import (
	"strings"
	"testing"
)

func TestParseHints(t *testing.T) {
	src := `# a comment line, ignored
label 0x8000 Start
label 32784 GameLoop
comment 0x8010 initialize the score table
data 0x9000 0x20
data 0x9100 32
entry 0x8100

`
	h, err := ParseHints(strings.NewReader(src))
	if err != nil {
		t.Fatalf("ParseHints: %v", err)
	}
	if h.Labels[0x8000] != "Start" {
		t.Errorf("Labels[0x8000] = %q, want Start", h.Labels[0x8000])
	}
	if h.Labels[0x8010] != "GameLoop" { // 32784 decimal == 0x8010
		t.Errorf("Labels[0x8010] (from decimal input) = %q, want GameLoop", h.Labels[0x8010])
	}
	if h.Comments[0x8010] != "initialize the score table" {
		t.Errorf("Comments[0x8010] = %q, want the full trailing text", h.Comments[0x8010])
	}
	if len(h.Data) != 2 || h.Data[0] != (DataRange{0x9000, 0x20}) || h.Data[1] != (DataRange{0x9100, 32}) {
		t.Errorf("Data = %+v, want [{0x9000 0x20} {0x9100 32}]", h.Data)
	}
	if len(h.Entry) != 1 || h.Entry[0] != 0x8100 {
		t.Errorf("Entry = %+v, want [0x8100]", h.Entry)
	}
}

func TestHints_DataRangeAt(t *testing.T) {
	h := &Hints{Data: []DataRange{{Start: 0x9000, Length: 0x10}}}

	if end, ok := h.DataRangeAt(0x8FFF); ok {
		t.Errorf("DataRangeAt(0x8FFF) = %#04x, true; want not found (just before the range)", end)
	}
	if end, ok := h.DataRangeAt(0x9000); !ok || end != 0x9010 {
		t.Errorf("DataRangeAt(0x9000) = %#04x, %v; want 0x9010, true (range start)", end, ok)
	}
	if end, ok := h.DataRangeAt(0x9005); !ok || end != 0x9010 {
		t.Errorf("DataRangeAt(0x9005) = %#04x, %v; want 0x9010, true (mid-range)", end, ok)
	}
	if end, ok := h.DataRangeAt(0x900F); !ok || end != 0x9010 {
		t.Errorf("DataRangeAt(0x900F) = %#04x, %v; want 0x9010, true (last byte, inclusive)", end, ok)
	}
	if end, ok := h.DataRangeAt(0x9010); ok {
		t.Errorf("DataRangeAt(0x9010) = %#04x, true; want not found (exclusive end)", end)
	}
}

func TestHints_DataRangeAt_NilReceiver(t *testing.T) {
	var h *Hints
	if end, ok := h.DataRangeAt(0x8000); ok {
		t.Errorf("nil *Hints DataRangeAt = %#04x, true; want not found, no panic", end)
	}
}

func TestParseHints_Errors(t *testing.T) {
	cases := []struct {
		name, src string
	}{
		{"unknown directive", "bogus 0x8000 x\n"},
		{"label missing name", "label 0x8000\n"},
		{"comment missing text", "comment 0x8000\n"},
		{"data wrong field count", "data 0x8000\n"},
		{"entry wrong field count", "entry 0x8000 extra\n"},
		{"bad address", "label notanaddr Start\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseHints(strings.NewReader(tc.src)); err == nil {
				t.Errorf("ParseHints(%q) = nil error, want an error", tc.src)
			}
		})
	}
}
