package render

import (
	"encoding/json"
	"io"

	"github.com/ha1tch/zendis/pkg/disasm"
)

// JSONRecord is the machine-readable form of one Record, as written by
// WriteJSON.
type JSONRecord struct {
	Kind         string `json:"kind"` // "instruction" or "data"
	Address      int    `json:"address"`
	Bytes        []int  `json:"bytes"`
	Mnemonic     string `json:"mnemonic,omitempty"`
	Operands     string `json:"operands,omitempty"`
	Text         string `json:"text,omitempty"`
	Length       int    `json:"length,omitempty"`
	Undefined    bool   `json:"undefined,omitempty"`
	PossibleZ80N string `json:"possible_z80n,omitempty"`
	Label        string `json:"label,omitempty"`
	Comment      string `json:"comment,omitempty"`
	Truncated    bool   `json:"truncated,omitempty"`
	DataKind     string `json:"data_kind,omitempty"` // a data record's kind when it is not bytes: hex, words, text, fill
}

// WriteJSON writes records as a JSON array of JSONRecord to w -- one
// object per instruction or data run, for feeding a listing into other
// tooling. hints may be nil (treated as empty).
func WriteJSON(w io.Writer, records []Record, labelFor map[uint16]string, hints *disasm.Hints) error {
	if hints == nil {
		hints = &disasm.Hints{}
	}
	out := make([]JSONRecord, 0, len(records))
	for _, rec := range records {
		jr := JSONRecord{Address: int(rec.Address), Bytes: toIntSlice(rec.Bytes)}
		if rec.Instr != nil {
			in := rec.Instr
			jr.Kind = "instruction"
			jr.Mnemonic = in.Mnemonic
			jr.Operands = in.Operands
			jr.Text = resolvedText(in, labelFor)
			jr.Length = in.Length
			jr.Undefined = in.Undefined
			jr.PossibleZ80N = in.PossibleZ80N
			jr.Comment = hints.Comments[in.Address]
		} else {
			jr.Kind = "data"
			jr.Truncated = rec.Truncated
			if rec.Kind != disasm.DataBytes {
				jr.DataKind = rec.Kind.String()
			}
			jr.Comment = hints.Comments[rec.Address]
		}
		if label, ok := labelFor[rec.Address]; ok {
			jr.Label = label
		}
		out = append(out, jr)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
