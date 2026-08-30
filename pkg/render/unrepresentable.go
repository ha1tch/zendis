package render

import "github.com/ha1tch/zendis/pkg/disasm"

// IsZ80NInstruction reports whether in occupies one of the 29 Z80N-
// repurposed ED opcode slots, regardless of which mnemonic rendered it
// -- ADD/PUSH/JP reuse their classic entries for some Z80N forms, so
// checking the mnemonic name alone would miss those. Checks the raw
// opcode byte (the byte right after the ED prefix) against
// disasm.Z80NLookup directly.
func IsZ80NInstruction(in *disasm.Instruction) bool {
	if in.Prefix != disasm.PrefixED || in.Undefined || len(in.Bytes) < 2 {
		return false
	}
	_, ok := disasm.Z80NLookup(in.Bytes[1])
	return ok
}

// undocumentedFormUnrepresentable reports whether in is one of three
// undocumented forms that neither pasmo nor SNASM can safely
// represent as a mnemonic -- confirmed against both real binaries, not
// assumed. Shared by PasmoUnrepresentable and SnasmUnrepresentable
// since both assemblers reject or mishandle exactly this same set;
// their only difference is Z80N support (pasmo has none, SNASM has
// full support), which each caller checks separately.
//   - The bare "IN (C)" form (flags-only read, no destination
//     register) -- pasmo: "ERROR: Invalid operand"; SNASM: "Syntax
//     Error in IN". Documented "IN r,(C)" renders with a leading
//     register in Operands and is unaffected; only the exact bare
//     form matches.
//   - The bare "OUT (C),0x00" form (no source register) -- pasmo and
//     SNASM both give a syntax error. Documented "OUT (C),r" is
//     unaffected.
//   - DDCB/FDCB's undocumented copy-to-register forms (e.g.
//     "RLC (IX+d),B"). pasmo: "ERROR: End line expected but ','
//     found" for every case tested. SNASM's behaviour is worse and is
//     exactly why this check matters for it specifically: the rotate
//     group (RLC/RRC/RL/RR/SLA/SRA/SLL/SRL) silently accepts the
//     syntax and emits the plain non-copying opcode instead -- "0
//     Errors, 0 Warnings", wrong bytes, no diagnostic at all (checked
//     mechanically across all 8 mnemonics x 7 targets, 56/56
//     combinations, no exceptions). SNASM's RES/SET copy-form fails
//     safely instead, same as pasmo (42/42 sampled combinations
//     rejected). Either way, this function's job is the same: don't
//     let zendis emit the syntax in the first place. The plain
//     (IX+d)-only form (no copy) works fine in both; only the
//     copy-target variant is affected. BIT's own z!=6 duplicate is
//     excluded here since it renders with no copy suffix at all
//     (T-4's own finding: BIT has no result to copy), so there's
//     nothing for either assembler to choke on there.
func undocumentedFormUnrepresentable(in *disasm.Instruction) (bool, string) {
	if in.Mnemonic == "IN" && in.Operands == "(C)" {
		return true, "undocumented bare IN (C)"
	}
	if in.Mnemonic == "OUT" && in.Operands == "(C),0x00" {
		return true, "undocumented bare OUT (C),0"
	}
	if in.Prefix == disasm.PrefixDDCB || in.Prefix == disasm.PrefixFDCB {
		subop := in.Bytes[3]
		x := subop >> 6
		z := subop & 7
		if x != 1 && z != 6 {
			return true, "undocumented DDCB/FDCB copy-to-register form"
		}
	}
	return false, ""
}

// PasmoUnrepresentable reports whether pasmo has no way to write in as
// a mnemonic at all, and why. pasmo additionally has zero Z80N support
// (confirmed by grepping its source for nextreg/z80n/zxnext -- no
// matches at all), unlike SNASM.
func PasmoUnrepresentable(in *disasm.Instruction) (bool, string) {
	if IsZ80NInstruction(in) {
		return true, "Z80N instruction"
	}
	return undocumentedFormUnrepresentable(in)
}

// SnasmUnrepresentable reports whether SNASM has no way to safely
// represent in as a mnemonic, and why. Unlike pasmo, SNASM has full
// Z80N support (confirmed: NextReg, MUL, ADD HL,imm/A etc. all
// assemble correctly, and an entire real, unedited 6393-byte Next
// program round-tripped byte-for-byte through it), so there's no Z80N
// check here at all -- only the three undocumented forms shared with
// pasmo.
func SnasmUnrepresentable(in *disasm.Instruction) (bool, string) {
	return undocumentedFormUnrepresentable(in)
}
