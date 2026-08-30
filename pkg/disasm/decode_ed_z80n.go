package disasm

// Z80N (ZX Spectrum Next) extended ED-prefixed instructions.
//
// Byte values and instruction identities are sourced from zen80's own
// dispatch table (zen80/z80/prefix_ed.go) and cross-checked against the
// z80nOpcodes table in zen80/z80/z80n_test.go — the exact byte->name
// mapping zen80's test suite asserts dispatch correctness against. All
// 29 opcodes. Every one of these byte values is undefined on a classic
// Z80 (it behaves as an 8-cycle two-byte NOP there), which is why
// resolution is gated on Target: under TargetZ80 these decode as
// Undefined with only a PossibleZ80N hint attached.
//
// The table and its lookup functions are exported (Z80NEntry,
// Z80NOperandKind, Z80NLookup, Z80NName) so a caller with its own need
// to know "what Z80N instruction lives at this opcode byte" -- e.g. a
// live disassembly view over another project's own memory -- can reuse
// this hard-won table (including the PUSH big-endian-operand quirk)
// instead of re-deriving it.

// Z80NOperandKind selects how a Z80N instruction's operand bytes are
// consumed and rendered.
type Z80NOperandKind int

const (
	// Z80NFixed: no operand bytes follow; operand text (possibly empty)
	// is entirely the entry's Pre field.
	Z80NFixed Z80NOperandKind = iota
	// Z80NImm8: one immediate byte, rendered as 0xNN.
	Z80NImm8
	// Z80NImm8Imm8: two immediate bytes, rendered as 0xNN,0xNN
	// (NEXTREG reg,value).
	Z80NImm8Imm8
	// Z80NImm16LE: a little-endian 16-bit immediate, rendered 0xNNNN.
	Z80NImm16LE
	// Z80NImm16BE: a BIG-endian 16-bit immediate, rendered 0xNNNN.
	// PUSH $im16 (ED 8A) only — the single big-endian operand in the
	// entire Z80/Z80N instruction set ("ED 8A high low"). Reusing the
	// shared little-endian word reader here would silently byte-swap
	// the value; zen80's z80nPush carries the same warning.
	Z80NImm16BE
)

// Z80NEntry describes one Z80N opcode slot. Operand text is rendered as
// Pre + immediates + Post; for Z80NFixed, Pre is the whole operand
// string.
type Z80NEntry struct {
	Mnemonic string
	Pre      string
	Kind     Z80NOperandKind
	Post     string
}

// z80nTable maps the ED sub-opcode byte to its Z80N instruction. The
// slots live in the classically-undefined parts of the x=0 (0x00-0x3F)
// and x=2 (0x80-0xBF) ED ranges; none are in x=1 or x=3. Unexported --
// a map is a reference type, so an exported var here would let any
// importer mutate or delete entries out from under zendis's own
// decoder. Z80NLookup returns a copy; that's the safe, public way in.
var z80nTable = map[byte]Z80NEntry{
	0x23: {Mnemonic: "SWAPNIB"},
	0x24: {Mnemonic: "MIRROR", Pre: "A"},
	0x27: {Mnemonic: "TEST", Kind: Z80NImm8},
	0x28: {Mnemonic: "BSLA", Pre: "DE,B"},
	0x29: {Mnemonic: "BSRA", Pre: "DE,B"},
	0x2A: {Mnemonic: "BSRL", Pre: "DE,B"},
	0x2B: {Mnemonic: "BSRF", Pre: "DE,B"},
	0x2C: {Mnemonic: "BRLC", Pre: "DE,B"},
	0x30: {Mnemonic: "MUL", Pre: "D,E"},
	0x31: {Mnemonic: "ADD", Pre: "HL,A"},
	0x32: {Mnemonic: "ADD", Pre: "DE,A"},
	0x33: {Mnemonic: "ADD", Pre: "BC,A"},
	0x34: {Mnemonic: "ADD", Pre: "HL,", Kind: Z80NImm16LE},
	0x35: {Mnemonic: "ADD", Pre: "DE,", Kind: Z80NImm16LE},
	0x36: {Mnemonic: "ADD", Pre: "BC,", Kind: Z80NImm16LE},
	0x8A: {Mnemonic: "PUSH", Kind: Z80NImm16BE},
	0x90: {Mnemonic: "OUTINB"},
	0x91: {Mnemonic: "NEXTREG", Kind: Z80NImm8Imm8},
	0x92: {Mnemonic: "NEXTREG", Kind: Z80NImm8, Post: ",A"},
	0x93: {Mnemonic: "PIXELDN"},
	0x94: {Mnemonic: "PIXELAD"},
	0x95: {Mnemonic: "SETAE"},
	0x98: {Mnemonic: "JP", Pre: "(C)"},
	0xA4: {Mnemonic: "LDIX"},
	0xA5: {Mnemonic: "LDWS"},
	0xAC: {Mnemonic: "LDDX"},
	0xB4: {Mnemonic: "LDIRX"},
	0xB7: {Mnemonic: "LDPIRX"},
	0xBC: {Mnemonic: "LDDRX"},
}

// Z80NLookup returns a copy of the table entry for a Z80N opcode byte.
// ok is false when op is not a Z80N slot.
func Z80NLookup(op byte) (entry Z80NEntry, ok bool) {
	e, ok := z80nTable[op]
	return e, ok
}

// Z80NName returns a short human-readable name for the Z80N instruction
// occupying op's slot ("MUL D,E", "NEXTREG"), for the PossibleZ80N field
// under TargetZ80. ok is false when op is not a Z80N slot. No operand
// bytes are consumed — under TargetZ80 the slot is a two-byte undefined
// instruction, and the hint must not change that.
func Z80NName(op byte) (name string, ok bool) {
	e, ok := z80nTable[op]
	if !ok {
		return "", false
	}
	if e.Kind == Z80NFixed && e.Pre != "" {
		return e.Mnemonic + " " + e.Pre, true
	}
	return e.Mnemonic, true
}

// decodeEDZ80N resolves op as a Z80N instruction under TargetZ80N,
// consuming any operand bytes. ok is false when op is not a Z80N slot
// (the caller falls back to its generic undefined form). consumed counts
// the bytes already read (the 0xED prefix and op itself).
func decodeEDZ80N(code []byte, offset int, addr uint16, op byte, consumed int) (in *Instruction, ok bool, err error) {
	e, present := Z80NLookup(op)
	if !present {
		return nil, false, nil
	}

	operands := e.Pre
	switch e.Kind {
	case Z80NFixed:
		// operand text is Pre alone
	case Z80NImm8:
		n, c, err := readByte(code, offset, consumed)
		if err != nil {
			return nil, true, err
		}
		consumed = c
		operands += HexByte(n)
	case Z80NImm8Imm8:
		a, c, err := readByte(code, offset, consumed)
		if err != nil {
			return nil, true, err
		}
		b, c, err := readByte(code, offset, c)
		if err != nil {
			return nil, true, err
		}
		consumed = c
		operands += HexByte(a) + "," + HexByte(b)
	case Z80NImm16LE:
		nn, c, err := readWord(code, offset, consumed)
		if err != nil {
			return nil, true, err
		}
		consumed = c
		operands += HexWord(nn)
	case Z80NImm16BE:
		hi, c, err := readByte(code, offset, consumed)
		if err != nil {
			return nil, true, err
		}
		lo, c, err := readByte(code, offset, c)
		if err != nil {
			return nil, true, err
		}
		consumed = c
		operands += HexWord(uint16(hi)<<8 | uint16(lo))
	}
	operands += e.Post

	return &Instruction{
		Address:  addr,
		Bytes:    append([]byte(nil), code[offset:offset+consumed]...),
		Mnemonic: e.Mnemonic,
		Operands: operands,
		Length:   consumed,
		Prefix:   PrefixED,
	}, true, nil
}
