package disasm

import "fmt"

// peekByte returns code[offset] and true, or 0 and false if offset is out
// of range.
func peekByte(code []byte, offset int) (byte, bool) {
	if offset < 0 || offset >= len(code) {
		return 0, false
	}
	return code[offset], true
}

// readByte reads a single immediate byte following the opcode(s) already
// consumed. consumed is the number of bytes read so far (including the
// opcode itself); it returns the byte and the new consumed count, or an
// error if the stream is truncated.
func readByte(code []byte, offset, consumed int) (byte, int, error) {
	b, ok := peekByte(code, offset+consumed)
	if !ok {
		return 0, consumed, fmt.Errorf("disasm: truncated instruction at offset %d (need 1 more byte)", offset)
	}
	return b, consumed + 1, nil
}

// readWord reads a little-endian 16-bit immediate.
func readWord(code []byte, offset, consumed int) (uint16, int, error) {
	lo, consumed, err := readByte(code, offset, consumed)
	if err != nil {
		return 0, consumed, err
	}
	hi, consumed, err := readByte(code, offset, consumed)
	if err != nil {
		return 0, consumed, err
	}
	return uint16(hi)<<8 | uint16(lo), consumed, nil
}

// Decode decodes a single instruction starting at code[offset], under
// the default TargetZ80: Z80N-only opcode slots come back Undefined,
// with a PossibleZ80N hint attached. addr is the address recorded on
// the returned Instruction — callers pass the load/org address plus
// offset, so Decode itself never needs to know where in memory the
// code slice was loaded.
func Decode(code []byte, offset int, addr uint16) (*Instruction, error) {
	return DecodeTarget(code, offset, addr, TargetZ80)
}

// DecodeTarget decodes a single instruction starting at code[offset],
// resolving classically-undefined ED opcode slots against the given
// Target. Only the ED table is target-sensitive: Z80N adds no CB,
// DD/FD, or unprefixed instructions.
func DecodeTarget(code []byte, offset int, addr uint16, target Target) (*Instruction, error) {
	op, ok := peekByte(code, offset)
	if !ok {
		return nil, fmt.Errorf("disasm: offset %d out of range (len %d)", offset, len(code))
	}

	switch op {
	case 0xCB:
		return decodeCB(code, offset, addr)
	case 0xED:
		return decodeED(code, offset, addr, target)
	case 0xDD:
		return decodeIndexed(code, offset, addr, "IX", PrefixDD)
	case 0xFD:
		return decodeIndexed(code, offset, addr, "IY", PrefixFD)
	}
	return decodeUnprefixed(code, offset, addr)
}

// decodeUnprefixed decodes the base (no-prefix) instruction table.
func decodeUnprefixed(code []byte, offset int, addr uint16) (*Instruction, error) {
	op := code[offset]
	mnem, operands, consumed, err := decodeUnprefixedCore(code, offset, addr, op, 1)
	if err != nil {
		return nil, err
	}
	return &Instruction{
		Address:  addr,
		Bytes:    append([]byte(nil), code[offset:offset+consumed]...),
		Mnemonic: mnem,
		Operands: operands,
		Length:   consumed,
		Prefix:   PrefixNone,
	}, nil
}

// decodeUnprefixedCore decodes op — already read from the stream — using
// the standard x/y/z/p/q bit decomposition (x = bits7-6, y = bits5-3,
// z = bits2-0, p = y>>1, q = y&1), the same decomposition zen80
// (github.com/ha1tch/zen80) uses to dispatch execution.
//
// consumed is the number of bytes already read to reach op (including op
// itself); any further immediate/displacement bytes are read relative to
// (code, offset) via readByte/readWord. addr is the instruction's own
// address, needed here only to compute JR/DJNZ absolute branch targets.
//
// This core is reusable from decode_ddfd.go for the DD/FD opcodes that
// turn out to be unaffected by the index-register substitution — there
// op is at offset+1, not offset, and consumed starts at 2 instead of 1.
func decodeUnprefixedCore(code []byte, offset int, addr uint16, op byte, consumed int) (mnem, operands string, newConsumed int, err error) {
	f := Decompose(op)
	x, y, z, p, q := f.X, f.Y, f.Z, f.P, f.Q

	relTarget := func(c int, d byte) string {
		return HexWord(addr + uint16(c) + uint16(int8(d)))
	}

	switch x {
	case 0:
		switch z {
		case 0:
			switch {
			case y == 0:
				return "NOP", "", consumed, nil
			case y == 1:
				return "EX", "AF,AF'", consumed, nil
			case y == 2:
				d, c, err := readByte(code, offset, consumed)
				if err != nil {
					return "", "", consumed, err
				}
				consumed = c
				return "DJNZ", relTarget(consumed, d), consumed, nil
			case y == 3:
				d, c, err := readByte(code, offset, consumed)
				if err != nil {
					return "", "", consumed, err
				}
				consumed = c
				return "JR", relTarget(consumed, d), consumed, nil
			default: // y = 4..7
				d, c, err := readByte(code, offset, consumed)
				if err != nil {
					return "", "", consumed, err
				}
				consumed = c
				return "JR", cc[y-4] + "," + relTarget(consumed, d), consumed, nil
			}
		case 1:
			if q == 0 {
				nn, c, err := readWord(code, offset, consumed)
				if err != nil {
					return "", "", consumed, err
				}
				consumed = c
				return "LD", rp[p] + "," + HexWord(nn), consumed, nil
			}
			return "ADD", "HL," + rp[p], consumed, nil
		case 2:
			if q == 0 {
				switch p {
				case 0:
					return "LD", "(BC),A", consumed, nil
				case 1:
					return "LD", "(DE),A", consumed, nil
				case 2:
					nn, c, err := readWord(code, offset, consumed)
					if err != nil {
						return "", "", consumed, err
					}
					consumed = c
					return "LD", "(" + HexWord(nn) + "),HL", consumed, nil
				default: // p == 3
					nn, c, err := readWord(code, offset, consumed)
					if err != nil {
						return "", "", consumed, err
					}
					consumed = c
					return "LD", "(" + HexWord(nn) + "),A", consumed, nil
				}
			}
			switch p {
			case 0:
				return "LD", "A,(BC)", consumed, nil
			case 1:
				return "LD", "A,(DE)", consumed, nil
			case 2:
				nn, c, err := readWord(code, offset, consumed)
				if err != nil {
					return "", "", consumed, err
				}
				consumed = c
				return "LD", "HL,(" + HexWord(nn) + ")", consumed, nil
			default: // p == 3
				nn, c, err := readWord(code, offset, consumed)
				if err != nil {
					return "", "", consumed, err
				}
				consumed = c
				return "LD", "A,(" + HexWord(nn) + ")", consumed, nil
			}
		case 3:
			if q == 0 {
				return "INC", rp[p], consumed, nil
			}
			return "DEC", rp[p], consumed, nil
		case 4:
			return "INC", r[y], consumed, nil
		case 5:
			return "DEC", r[y], consumed, nil
		case 6:
			n, c, err := readByte(code, offset, consumed)
			if err != nil {
				return "", "", consumed, err
			}
			consumed = c
			return "LD", r[y] + "," + HexByte(n), consumed, nil
		case 7:
			switch y {
			case 0:
				return "RLCA", "", consumed, nil
			case 1:
				return "RRCA", "", consumed, nil
			case 2:
				return "RLA", "", consumed, nil
			case 3:
				return "RRA", "", consumed, nil
			case 4:
				return "DAA", "", consumed, nil
			case 5:
				return "CPL", "", consumed, nil
			case 6:
				return "SCF", "", consumed, nil
			default: // y == 7
				return "CCF", "", consumed, nil
			}
		}
	case 1:
		if z == 6 && y == 6 {
			return "HALT", "", consumed, nil
		}
		return "LD", r[y] + "," + r[z], consumed, nil
	case 2:
		if aluTakesA[y] {
			return aluNames[y], "A," + r[z], consumed, nil
		}
		return aluNames[y], r[z], consumed, nil
	case 3:
		switch z {
		case 0:
			return "RET", cc[y], consumed, nil
		case 1:
			if q == 0 {
				return "POP", rp2[p], consumed, nil
			}
			switch p {
			case 0:
				return "RET", "", consumed, nil
			case 1:
				return "EXX", "", consumed, nil
			case 2:
				return "JP", "(HL)", consumed, nil
			default: // p == 3
				return "LD", "SP,HL", consumed, nil
			}
		case 2:
			nn, c, err := readWord(code, offset, consumed)
			if err != nil {
				return "", "", consumed, err
			}
			consumed = c
			return "JP", cc[y] + "," + HexWord(nn), consumed, nil
		case 3:
			switch y {
			case 0:
				nn, c, err := readWord(code, offset, consumed)
				if err != nil {
					return "", "", consumed, err
				}
				consumed = c
				return "JP", HexWord(nn), consumed, nil
			case 1:
				return "", "", consumed, fmt.Errorf("disasm: internal error, CB prefix reached decodeUnprefixedCore")
			case 2:
				n, c, err := readByte(code, offset, consumed)
				if err != nil {
					return "", "", consumed, err
				}
				consumed = c
				return "OUT", "(" + HexByte(n) + "),A", consumed, nil
			case 3:
				n, c, err := readByte(code, offset, consumed)
				if err != nil {
					return "", "", consumed, err
				}
				consumed = c
				return "IN", "A,(" + HexByte(n) + ")", consumed, nil
			case 4:
				return "EX", "(SP),HL", consumed, nil
			case 5:
				return "EX", "DE,HL", consumed, nil
			case 6:
				return "DI", "", consumed, nil
			default: // y == 7
				return "EI", "", consumed, nil
			}
		case 4:
			nn, c, err := readWord(code, offset, consumed)
			if err != nil {
				return "", "", consumed, err
			}
			consumed = c
			return "CALL", cc[y] + "," + HexWord(nn), consumed, nil
		case 5:
			if q == 0 {
				return "PUSH", rp2[p], consumed, nil
			}
			switch p {
			case 0:
				nn, c, err := readWord(code, offset, consumed)
				if err != nil {
					return "", "", consumed, err
				}
				consumed = c
				return "CALL", HexWord(nn), consumed, nil
			default:
				return "", "", consumed, fmt.Errorf("disasm: internal error, prefix byte reached decodeUnprefixedCore")
			}
		case 6:
			n, c, err := readByte(code, offset, consumed)
			if err != nil {
				return "", "", consumed, err
			}
			consumed = c
			if aluTakesA[y] {
				return aluNames[y], "A," + HexByte(n), consumed, nil
			}
			return aluNames[y], HexByte(n), consumed, nil
		default: // z == 7
			return "RST", HexByte(y * 8), consumed, nil
		}
	}
	return "", "", consumed, fmt.Errorf("disasm: internal error, unhandled opcode 0x%02X", op)
}
