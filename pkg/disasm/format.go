package disasm

import "fmt"

// HexByte formats an 8-bit immediate as zendis's canonical output form,
// e.g. 0x2A. This matches one of zenas's accepted input forms, so
// zendis output can be fed back into zenas without translation.
// Exported so an embedder (e.g. a live disassembly view over zenzx's
// own memory) can render bytes the same way zendis's own decoder does,
// without re-deriving the format.
func HexByte(b byte) string {
	return fmt.Sprintf("0x%02X", b)
}

// HexWord formats a 16-bit immediate/address, e.g. 0x8000.
func HexWord(w uint16) string {
	return fmt.Sprintf("0x%04X", w)
}

// DispText formats a signed 8-bit index displacement as it appears inside
// (IX+d)/(IY+d), e.g. "+0x05" or "-0x03". A zero displacement still
// renders explicitly ("+0x00") since callers only invoke this for the
// indexed forms, where the byte is always present in the encoding.
func DispText(d int8) string {
	if d < 0 {
		return fmt.Sprintf("-0x%02X", -int(d))
	}
	return fmt.Sprintf("+0x%02X", int(d))
}
