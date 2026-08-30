package disasm

import "strings"

// BranchTarget reports the absolute address a JP, CALL, JR, or DJNZ
// instruction targets, derived directly from the instruction's own
// encoded Bytes -- not by re-parsing Operands text. ok is false for
// anything else, including JP (HL)/(IX)/(IY) (the target is
// register-computed at runtime, not encoded in the instruction) and
// RST (a fixed, well-known interrupt vector rather than a label-worthy
// address -- T-2's own scope names JP/JR/CALL/DJNZ specifically).
//
// This works uniformly across DD/FD-prefixed pass-through forms too:
// a DD/FD prefix byte only ever prepends the encoding, so "the
// displacement is the last byte" and "the absolute address is the
// last two bytes, little-endian" hold regardless of Prefix.
func (in *Instruction) BranchTarget() (target uint16, ok bool) {
	switch in.Mnemonic {
	case "DJNZ", "JR":
		if len(in.Bytes) == 0 {
			return 0, false
		}
		d := int8(in.Bytes[len(in.Bytes)-1])
		return in.Address + uint16(in.Length) + uint16(d), true
	case "JP", "CALL":
		if strings.HasPrefix(in.Operands, "(") {
			return 0, false // (HL)/(IX)/(IY): register-computed, not encoded
		}
		if len(in.Bytes) < 2 {
			return 0, false
		}
		lo := in.Bytes[len(in.Bytes)-2]
		hi := in.Bytes[len(in.Bytes)-1]
		return uint16(hi)<<8 | uint16(lo), true
	}
	return 0, false
}
