package render

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/ha1tch/zendis/pkg/disasm"
)

func TestWriteListing_HintsIntegration(t *testing.T) {
	code := []byte{
		0xC3, 0x0B, 0x80, // 8000: JP 0x800B
		0x00, 0x01, 0x02, 0x03, // 8003: data
		0x04, 0x05, 0x06, 0x07, // 8007: more data
		0x76, // 800B: HALT
	}
	hints := &disasm.Hints{
		Labels:   map[uint16]string{0x800B: "Main"},
		Comments: map[uint16]string{0x800B: "program entry point"},
		Data:     []disasm.DataRange{{Start: 0x8003, Length: 8}},
	}
	records := DecodeBuffer(code, 0x8000, disasm.TargetZ80, hints)
	labelFor := ResolveLabels(records, hints, true)

	var buf bytes.Buffer
	WriteListing(&buf, records, labelFor, hints, false, io.Discard)
	out := buf.String()

	if !strings.Contains(out, "JP Main") {
		t.Errorf("output missing label-substituted branch target:\n%s", out)
	}
	if !strings.Contains(out, "HALT  ; program entry point") {
		t.Errorf("output missing trailing comment:\n%s", out)
	}
	// 8-byte data range chunked into two 4-byte rows.
	if !strings.Contains(out, "8003:  00 01 02 03 DB 0x00,0x01,0x02,0x03") {
		t.Errorf("output missing first 4-byte data chunk:\n%s", out)
	}
	if !strings.Contains(out, "8007:  04 05 06 07 DB 0x04,0x05,0x06,0x07") {
		t.Errorf("output missing second 4-byte data chunk:\n%s", out)
	}
}
