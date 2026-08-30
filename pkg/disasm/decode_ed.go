package disasm

import "fmt"

// imNames maps the IM y-field to its display text. y=1/5 is the
// undocumented "IM 0/1" form; the rest have well-defined behaviour,
// including the known y=4..7 duplicates of y=0..3.
var imNames = [8]string{"0", "0/1", "1", "2", "0", "0/1", "1", "2"}

// decodeED decodes a 0xED-prefixed instruction. offset points at the
// 0xED byte itself.
//
// Z80N (github.com/ha1tch/zenzx's target) repurposes a number of the
// classically-undefined ED opcode slots for its own instructions
// (NEXTREG, MUL D,E, the LDIX/LDDX/LDWS block-copy family, and others
// — see decode_ed_z80n.go for the full table, T-1 in docs/TRACKING.md
// for the item). Under TargetZ80N those resolve to their Z80N
// mnemonics; under TargetZ80 they stay Undefined, with
// Instruction.PossibleZ80N naming what they would have been, so
// classic-Z80 listings are never silently mislabelled with Next-only
// instructions.
func decodeED(code []byte, offset int, addr uint16, target Target) (*Instruction, error) {
	op, c, err := readByte(code, offset, 1) // consumed=1 accounts for 0xED itself
	if err != nil {
		return nil, err
	}
	consumed := c

	f := Decompose(op)
	x, y, z, p, q := f.X, f.Y, f.Z, f.P, f.Q

	fin := func(mnem, operands string) (*Instruction, error) {
		return &Instruction{
			Address:  addr,
			Bytes:    append([]byte(nil), code[offset:offset+consumed]...),
			Mnemonic: mnem,
			Operands: operands,
			Length:   consumed,
			Prefix:   PrefixED,
		}, nil
	}
	undef := func() (*Instruction, error) {
		return &Instruction{
			Address:   addr,
			Bytes:     append([]byte(nil), code[offset:offset+consumed]...),
			Mnemonic:  "???",
			Operands:  fmt.Sprintf("ED %02X", op),
			Length:    consumed,
			Prefix:    PrefixED,
			Undefined: true,
		}, nil
	}
	// resolve handles a classically-undefined slot: under TargetZ80N a
	// Z80N-repurposed slot decodes to its real instruction; under
	// TargetZ80 it stays a two-byte Undefined with PossibleZ80N naming
	// the Z80N instruction it would have been. Slots Z80N leaves alone
	// are Undefined regardless of target.
	resolve := func() (*Instruction, error) {
		if target == TargetZ80N {
			if in, ok, err := decodeEDZ80N(code, offset, addr, op, consumed); ok {
				return in, err
			}
			return undef()
		}
		in, err := undef()
		if err == nil {
			if hint, ok := Z80NName(op); ok {
				in.PossibleZ80N = hint
			}
		}
		return in, err
	}

	switch x {
	case 1: // 0x40-0x7F: the main ED block
		switch z {
		case 0:
			if y == 6 {
				return fin("IN", "(C)")
			}
			return fin("IN", r[y]+",(C)")
		case 1:
			if y == 6 {
				return fin("OUT", "(C),0x00")
			}
			return fin("OUT", "(C),"+r[y])
		case 2:
			if q == 0 {
				return fin("SBC", "HL,"+rp[p])
			}
			return fin("ADC", "HL,"+rp[p])
		case 3:
			nn, cc2, err := readWord(code, offset, consumed)
			if err != nil {
				return nil, err
			}
			consumed = cc2
			if q == 0 {
				return fin("LD", "("+HexWord(nn)+"),"+rp[p])
			}
			return fin("LD", rp[p]+",("+HexWord(nn)+")")
		case 4:
			return fin("NEG", "")
		case 5:
			if y == 1 {
				return fin("RETI", "")
			}
			return fin("RETN", "")
		case 6:
			return fin("IM", imNames[y])
		case 7:
			switch y {
			case 0:
				return fin("LD", "I,A")
			case 1:
				return fin("LD", "R,A")
			case 2:
				return fin("LD", "A,I")
			case 3:
				return fin("LD", "A,R")
			case 4:
				return fin("RRD", "")
			case 5:
				return fin("RLD", "")
			default: // y == 6, 7: undocumented ED NOP
				return fin("NOP", "")
			}
		}
	case 2: // 0x80-0xBF: block instructions, only y=4..7, z=0..3 defined classically
		if y >= 4 && z <= 3 {
			names := [4][4]string{
				{"LDI", "CPI", "INI", "OUTI"},
				{"LDD", "CPD", "IND", "OUTD"},
				{"LDIR", "CPIR", "INIR", "OTIR"},
				{"LDDR", "CPDR", "INDR", "OTDR"},
			}
			return fin(names[y-4][z], "")
		}
		return resolve()
	}
	// x == 0 (0x00-0x3F) and x == 3 (0xC0-0xFF): classically undefined.
	// The Z80N-repurposed slots live in x == 0 and in x == 2's non-block
	// combinations above; x == 3 has none. resolve() sorts each case by
	// target (see decode_ed_z80n.go).
	return resolve()
}
