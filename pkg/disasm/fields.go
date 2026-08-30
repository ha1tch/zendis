package disasm

// Fields holds the standard Z80 opcode bit decomposition: x = bits 7-6,
// y = bits 5-3, z = bits 2-0, p = y>>1, q = y&1. This is the exact
// decomposition zen80 uses internally for its own dispatch (see
// zen80/z80/decode.go), and the one zendis's own decoder builds on
// throughout -- exported so a caller with the same need (e.g. a live
// disassembly view over another project's own memory) can reuse this
// instead of re-deriving the same three lines of bit math.
type Fields struct {
	X, Y, Z, P, Q byte
}

// Decompose extracts the standard Z80 opcode bit fields from a single
// byte.
func Decompose(op byte) Fields {
	y := (op >> 3) & 7
	return Fields{
		X: op >> 6,
		Y: y,
		Z: op & 7,
		P: y >> 1,
		Q: y & 1,
	}
}
