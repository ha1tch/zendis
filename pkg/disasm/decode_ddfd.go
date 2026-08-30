package disasm

import "fmt"

// decodeIndexed decodes a 0xDD- or 0xFD-prefixed instruction. offset
// points at the prefix byte itself; reg is "IX" or "IY"; pfx is the
// Prefix value to record on ordinary (non-CB) results.
//
// The substitution rule (standard, documented Z80 behaviour): any base
// opcode that references HL, H, L or (HL) through the general
// register/register-pair fields gets IX/IY (or IXh/IXl/(IX+d)) substituted
// in its place. Two well-known exceptions are hard-coded: HALT (0x76) and
// EX DE,HL (0xEB) are both unaffected by the prefix — neither routes
// through the generic HL field the substitution rides on. Opcodes whose
// fields don't touch HL/H/L/(HL) at all decode identically to the
// unprefixed table (via decodeUnprefixedCore) and simply carry the extra
// prefix byte.
func decodeIndexed(code []byte, offset int, addr uint16, reg string, pfx Prefix) (*Instruction, error) {
	inner, consumed, err := readByte(code, offset, 1) // consumed=1 accounts for the DD/FD byte
	if err != nil {
		return nil, err
	}

	fin := func(mnem, operands string, c int) (*Instruction, error) {
		return &Instruction{
			Address:  addr,
			Bytes:    append([]byte(nil), code[offset:offset+c]...),
			Mnemonic: mnem,
			Operands: operands,
			Length:   c,
			Prefix:   pfx,
		}, nil
	}

	// Unaffected exceptions.
	if inner == 0x76 {
		return fin("HALT", "", consumed)
	}
	if inner == 0xEB {
		return fin("EX", "DE,HL", consumed)
	}

	// DDCB/FDCB sub-form: prefix, 0xCB, displacement, then a CB-style
	// opcode that always targets (IX+d)/(IY+d), with the undocumented
	// z != 6 forms also copying the result into r[z].
	if inner == 0xCB {
		d, consumed, err := readByte(code, offset, consumed)
		if err != nil {
			return nil, err
		}
		cbOp, consumed2, err := readByte(code, offset, consumed)
		if err != nil {
			return nil, err
		}
		consumed = consumed2

		ddPfx := PrefixDDCB
		if pfx == PrefixFD {
			ddPfx = PrefixFDCB
		}
		cbfin := func(mnem, operands string) (*Instruction, error) {
			return &Instruction{
				Address:  addr,
				Bytes:    append([]byte(nil), code[offset:offset+consumed]...),
				Mnemonic: mnem,
				Operands: operands,
				Length:   consumed,
				Prefix:   ddPfx,
			}, nil
		}

		x := cbOp >> 6
		y := (cbOp >> 3) & 7
		z := cbOp & 7
		mem := "(" + reg + DispText(int8(d)) + ")"
		copyTo := ""
		if z != 6 {
			copyTo = "," + r[z]
		}

		switch x {
		case 0:
			return cbfin(rotNames[y], mem+copyTo)
		case 1:
			// BIT has no result to copy; z != 6 is an undocumented
			// duplicate with no observable difference in outcome.
			return cbfin("BIT", fmt.Sprintf("%d,%s", y, mem))
		case 2:
			return cbfin("RES", fmt.Sprintf("%d,%s%s", y, mem, copyTo))
		default: // x == 3
			return cbfin("SET", fmt.Sprintf("%d,%s%s", y, mem, copyTo))
		}
	}

	// General case: decode `inner` as if it were the unprefixed table,
	// substituting HL/H/L/(HL) for IX(or IY)/IXh/IXl/(IX+d) wherever the
	// opcode's fields reference them.
	op := inner
	f := Decompose(op)
	x, y, z, p, q := f.X, f.Y, f.Z, f.P, f.Q

	// unaffected decodes `op` exactly as the unprefixed table would,
	// via the shared core, and just carries the extra prefix byte.
	unaffected := func() (*Instruction, error) {
		mnem, operands, c, err := decodeUnprefixedCore(code, offset, addr, op, consumed)
		if err != nil {
			return nil, err
		}
		return fin(mnem, operands, c)
	}

	// rpSub/rp2Sub mirror the rp/rp2 tables with slot 2 (HL) replaced by
	// the index register.
	rpSub := rp
	rpSub[2] = reg
	rp2Sub := rp2
	rp2Sub[2] = reg

	// regField resolves an r[]-style field (y or z), substituting the
	// index register's half-registers/indirect form for 4/5/6. It reads
	// a displacement byte for index 6, advancing consumed.
	regField := func(idx byte) (string, error) {
		switch idx {
		case 4:
			return reg + "h", nil
		case 5:
			return reg + "l", nil
		case 6:
			d, c, err := readByte(code, offset, consumed)
			if err != nil {
				return "", err
			}
			consumed = c
			return "(" + reg + DispText(int8(d)) + ")", nil
		default:
			return r[idx], nil
		}
	}

	switch x {
	case 0:
		switch z {
		case 1:
			if q == 1 {
				return fin("ADD", reg+","+rpSub[p], consumed)
			}
			nn, c, err := readWord(code, offset, consumed)
			if err != nil {
				return nil, err
			}
			consumed = c
			return fin("LD", rpSub[p]+","+HexWord(nn), consumed)
		case 2:
			if p == 2 {
				if q == 0 {
					nn, c, err := readWord(code, offset, consumed)
					if err != nil {
						return nil, err
					}
					consumed = c
					return fin("LD", "("+HexWord(nn)+"),"+reg, consumed)
				}
				nn, c, err := readWord(code, offset, consumed)
				if err != nil {
					return nil, err
				}
				consumed = c
				return fin("LD", reg+",("+HexWord(nn)+")", consumed)
			}
			// p != 2: unaffected (BC)/(DE)/A forms, same as unprefixed.
			return unaffected()
		case 3:
			if p == 2 {
				if q == 0 {
					return fin("INC", reg, consumed)
				}
				return fin("DEC", reg, consumed)
			}
			return unaffected()
		case 4:
			f, err := regField(y)
			if err != nil {
				return nil, err
			}
			return fin("INC", f, consumed)
		case 5:
			f, err := regField(y)
			if err != nil {
				return nil, err
			}
			return fin("DEC", f, consumed)
		case 6:
			f, err := regField(y)
			if err != nil {
				return nil, err
			}
			n, c, err := readByte(code, offset, consumed)
			if err != nil {
				return nil, err
			}
			consumed = c
			return fin("LD", f+","+HexByte(n), consumed)
		default:
			// z=0 (NOP/EX AF,AF'/DJNZ/JR family) and z=7 (RLCA etc.):
			// none of these reference HL/H/L, unaffected.
			return unaffected()
		}
	case 1:
		// HALT (0x76) already handled above. When one side of a LD r,r'
		// references (HL)->(IX+d), the *other* side stays a literal
		// register name even if it's H or L: LD H,(IX+d) is the
		// officially documented DD-prefixed form, distinct from the
		// undocumented IXh/IXl half-register substitution that only
		// applies when neither side is a memory reference.
		if y == 6 || z == 6 {
			var lhs, rhs string
			if y == 6 {
				f, err := regField(6)
				if err != nil {
					return nil, err
				}
				lhs, rhs = f, r[z]
			} else {
				f, err := regField(6)
				if err != nil {
					return nil, err
				}
				lhs, rhs = r[y], f
			}
			return fin("LD", lhs+","+rhs, consumed)
		}
		lhs, err := regField(y)
		if err != nil {
			return nil, err
		}
		rhs, err := regField(z)
		if err != nil {
			return nil, err
		}
		return fin("LD", lhs+","+rhs, consumed)
	case 2:
		f, err := regField(z)
		if err != nil {
			return nil, err
		}
		if aluTakesA[y] {
			return fin(aluNames[y], "A,"+f, consumed)
		}
		return fin(aluNames[y], f, consumed)
	case 3:
		switch z {
		case 1:
			if q == 0 {
				return fin("POP", rp2Sub[p], consumed)
			}
			switch p {
			case 2:
				return fin("JP", "("+reg+")", consumed)
			case 3:
				return fin("LD", "SP,"+reg, consumed)
			default:
				return unaffected()
			}
		case 3:
			if y == 4 {
				return fin("EX", "(SP),"+reg, consumed)
			}
			return unaffected()
		case 5:
			if q == 0 {
				return fin("PUSH", rp2Sub[p], consumed)
			}
			return unaffected()
		default:
			// z=0 (RET cc), z=2 (JP cc,nn), z=4 (CALL cc,nn), z=6 (alu n),
			// z=7 (RST): none reference HL/H/L, unaffected.
			return unaffected()
		}
	}
	return unaffected()
}
