// Package disasm decodes Z80 and Z80N machine code into textual instructions.
//
// The decoder follows the standard bit decomposition of a Z80 opcode byte
// into x (bits 7-6), y (bits 5-3) and z (bits 2-0) fields, with p = y>>1 and
// q = y&1 used for the two-register-pair groupings. This is the same
// decomposition zen80 (github.com/ha1tch/zen80) uses to dispatch execution;
// zendis uses it to produce mnemonics and operand text instead.
package disasm

import "fmt"

// Prefix identifies which opcode prefix, if any, introduced an instruction.
type Prefix int

const (
	PrefixNone Prefix = iota
	PrefixCB
	PrefixED
	PrefixDD
	PrefixFD
	PrefixDDCB
	PrefixFDCB
)

func (p Prefix) String() string {
	switch p {
	case PrefixNone:
		return "none"
	case PrefixCB:
		return "CB"
	case PrefixED:
		return "ED"
	case PrefixDD:
		return "DD"
	case PrefixFD:
		return "FD"
	case PrefixDDCB:
		return "DDCB"
	case PrefixFDCB:
		return "FDCB"
	default:
		return "?"
	}
}

// Instruction is a single decoded instruction.
type Instruction struct {
	// Address is the address of the instruction's first byte.
	Address uint16
	// Bytes holds the raw encoding, prefix included.
	Bytes []byte
	// Mnemonic is the instruction name (e.g. "LD", "ADD", "JR").
	Mnemonic string
	// Operands is the fully formatted operand list (e.g. "A,(HL)"), or
	// empty for instructions that take none (e.g. "NOP", "RET").
	Operands string
	// Length is len(Bytes); kept as a field for callers that only have
	// the Instruction and not the original slice.
	Length int
	// Prefix records which prefix table produced this instruction.
	Prefix Prefix
	// Undefined marks an opcode that decodes to a real (if undocumented)
	// Z80 behaviour that zendis does not resolve to a mnemonic under the
	// decoding Target. Length and Bytes are still correct so a caller can
	// keep stepping through the stream; Mnemonic holds a placeholder such
	// as "???".
	Undefined bool
	// PossibleZ80N names the Z80N instruction this opcode would resolve
	// to under TargetZ80N ("NEXTREG", "MUL D,E"), set only when Undefined
	// is true, the Target was TargetZ80, and the opcode occupies a slot
	// Z80N repurposes. Empty otherwise.
	PossibleZ80N string
}

// Target selects which CPU's instruction set classically-undefined
// ED opcode slots are resolved against. Z80N (the Spectrum Next's
// extended Z80) repurposes some of those slots for its own instructions;
// a classic Z80 leaves them undefined.
type Target int

const (
	// TargetZ80 never resolves Z80N-only slots: they decode as Undefined,
	// with Instruction.PossibleZ80N naming what they would have been.
	TargetZ80 Target = iota
	// TargetZ80N resolves the Z80N extended instruction set.
	TargetZ80N
)

func (t Target) String() string {
	switch t {
	case TargetZ80:
		return "z80"
	case TargetZ80N:
		return "z80n"
	default:
		return "?"
	}
}

// ParseTarget parses "z80"/"z80n" into the matching Target -- the
// reverse of String, for a caller configuring Target from a string
// (a config file, an environment variable, a caller's own flag)
// rather than a Go constant. cmd/zendis's own --target flag now uses
// this instead of duplicating the same two-case switch inline, which
// is the same reason it's exported: nothing else in this codebase
// should need a second copy of it either.
func ParseTarget(s string) (Target, error) {
	switch s {
	case "z80":
		return TargetZ80, nil
	case "z80n":
		return TargetZ80N, nil
	default:
		return 0, fmt.Errorf("disasm: unknown target %q (want z80 or z80n)", s)
	}
}

// Text renders the instruction as "MNEMONIC OPERANDS", matching the
// convention zenas expects as assembler input.
func (in *Instruction) Text() string {
	if in.Operands == "" {
		return in.Mnemonic
	}
	return in.Mnemonic + " " + in.Operands
}
