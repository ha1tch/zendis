package disasm

// r is the 8-bit register field, indexed by y or z (0-7).
// Index 6 is the (HL) memory operand, not a register.
var r = [8]string{"B", "C", "D", "E", "H", "L", "(HL)", "A"}

// rIndexed produces the r[] table for DD/FD-prefixed instructions, where
// (HL) becomes (IX+d)/(IY+d) and H/L become IXh/IXl or IYh/IYl for the
// undocumented-but-real half-register forms. disp is formatted by the
// caller and passed in as the already-rendered "+5" / "-3" text; pass ""
// for the half-register (non-memory) case.
func rIndexed(reg string, disp string) [8]string {
	return [8]string{
		"B", "C", "D", "E",
		reg + "h", reg + "l",
		"(" + reg + disp + ")",
		"A",
	}
}

// rp is the 16-bit register-pair field for most instructions.
var rp = [4]string{"BC", "DE", "HL", "SP"}

// rp2 is the register-pair field used by PUSH/POP, where slot 3 is AF
// instead of SP.
var rp2 = [4]string{"BC", "DE", "HL", "AF"}

// cc is the condition field.
var cc = [8]string{"NZ", "Z", "NC", "C", "PO", "PE", "P", "M"}

// aluNames is the arithmetic/logic group, indexed by y (0-7).
var aluNames = [8]string{"ADD", "ADC", "SUB", "SBC", "AND", "XOR", "OR", "CP"}

// aluTakesA reports whether aluNames[y] takes an explicit "A," left
// operand in canonical Z80 syntax (ADD/ADC/SBC do; SUB/AND/XOR/OR/CP
// implicitly target A and are written with a single operand).
var aluTakesA = [8]bool{true, true, false, true, false, false, false, false}

// rotNames is the CB-prefixed rotate/shift group, indexed by y (0-7).
var rotNames = [8]string{"RLC", "RRC", "RL", "RR", "SLA", "SRA", "SLL", "SRL"}

// The functions below expose the tables above for reuse outside this
// package -- e.g. a live disassembly view over another project's own
// memory (zenzx) -- without forcing a caller to re-derive or duplicate
// zendis's own naming tables. Functions, not exported vars: the
// underlying arrays are package-level state this decoder depends on
// for its own correctness, and an exported var of array type is
// directly mutable by any importer (in Go, `disasm.SomeArray[0] = "x"`
// on an exported [N]string var silently corrupts it for every future
// caller in the process, including zendis's own decoder). A function
// returning a value has no such risk. Every accessor masks its field
// argument to the table's own valid range, so a caller passing an
// out-of-range byte gets a defined answer, never a panic -- worth
// having when the caller is disassembling live, possibly-not-yet-valid
// memory.

// RegisterName returns the 8-bit register name for a 3-bit register
// field (0-7), e.g. RegisterName(7) == "A". Index 6 is "(HL)", the
// memory operand slot, not a register.
func RegisterName(field byte) string {
	return r[field&7]
}

// RegisterPairName returns the 16-bit register-pair name for a 2-bit
// field (0-3) in the convention most instructions use (LD rp,nn;
// ADD HL,rp; INC/DEC rp), where slot 3 is SP.
func RegisterPairName(field byte) string {
	return rp[field&3]
}

// RegisterPairNameAF returns the 16-bit register-pair name for a 2-bit
// field (0-3) in the PUSH/POP convention, where slot 3 is AF instead
// of SP.
func RegisterPairNameAF(field byte) string {
	return rp2[field&3]
}

// ConditionName returns the condition-code name for a 3-bit field
// (0-7): NZ, Z, NC, C, PO, PE, P, M.
func ConditionName(field byte) string {
	return cc[field&7]
}

// ALUName returns the arithmetic/logic group mnemonic for a 3-bit
// field (0-7): ADD, ADC, SUB, SBC, AND, XOR, OR, CP.
func ALUName(field byte) string {
	return aluNames[field&7]
}

// ALUTakesA reports whether ALUName(field) takes an explicit "A,"
// left operand in canonical Z80 syntax (ADD/ADC/SBC do; SUB/AND/XOR/OR/CP
// implicitly target A and are written with a single operand).
func ALUTakesA(field byte) bool {
	return aluTakesA[field&7]
}

// RotateName returns the CB-prefixed rotate/shift group mnemonic for a
// 3-bit field (0-7): RLC, RRC, RL, RR, SLA, SRA, SLL, SRL. SLL is the
// undocumented "shift left, set bit 0" form.
func RotateName(field byte) string {
	return rotNames[field&7]
}
