package render

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/ha1tch/zendis/pkg/disasm"
)

func TestWriteJSON_Structure(t *testing.T) {
	code := []byte{0xC3, 0x03, 0x80, 0x76} // JP 0x8003; HALT at 0x8003
	hints := &disasm.Hints{
		Labels:   map[uint16]string{0x8003: "Main"},
		Comments: map[uint16]string{0x8003: "entry"},
	}
	records := DecodeBuffer(code, 0x8000, disasm.TargetZ80, hints)
	labelFor := ResolveLabels(records, hints, true)

	var buf bytes.Buffer
	if err := WriteJSON(&buf, records, labelFor, hints); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var out []JSONRecord
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("WriteJSON produced invalid JSON: %v\n%s", err, buf.String())
	}
	if len(out) != 2 {
		t.Fatalf("got %d records, want 2", len(out))
	}
	if out[0].Kind != "instruction" || out[0].Text != "JP Main" {
		t.Errorf("record 0 = %+v, want kind=instruction text=\"JP Main\"", out[0])
	}
	if out[1].Label != "Main" || out[1].Comment != "entry" {
		t.Errorf("record 1 = %+v, want label=Main comment=entry", out[1])
	}
}

func TestWriteJSON_DataRecord(t *testing.T) {
	code := []byte{0x00, 0x01, 0x02}
	hints := &disasm.Hints{Data: []disasm.DataRange{{Start: 0x8000, Length: 3}}}
	records := DecodeBuffer(code, 0x8000, disasm.TargetZ80, hints)

	var buf bytes.Buffer
	if err := WriteJSON(&buf, records, nil, hints); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	var out []JSONRecord
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(out) != 1 || out[0].Kind != "data" || len(out[0].Bytes) != 3 {
		t.Errorf("got %+v, want a single data record with 3 bytes", out)
	}
}
