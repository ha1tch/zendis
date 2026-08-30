package disasm

import "fmt"

// decodeCB decodes a 0xCB-prefixed instruction. offset points at the 0xCB
// byte itself. The CB table is fully regular: x selects the group
// (rotate/shift, BIT, RES, SET), y is either the rotate-op selector or the
// bit number, and z selects the operand register (6 = (HL)).
func decodeCB(code []byte, offset int, addr uint16) (*Instruction, error) {
	op, c, err := readByte(code, offset, 1) // consumed=1 accounts for 0xCB itself
	if err != nil {
		return nil, err
	}
	consumed := c

	f := Decompose(op)
	x, y, z := f.X, f.Y, f.Z

	fin := func(mnem, operands string) (*Instruction, error) {
		return &Instruction{
			Address:  addr,
			Bytes:    append([]byte(nil), code[offset:offset+consumed]...),
			Mnemonic: mnem,
			Operands: operands,
			Length:   consumed,
			Prefix:   PrefixCB,
		}, nil
	}

	switch x {
	case 0:
		return fin(rotNames[y], r[z])
	case 1:
		return fin("BIT", fmt.Sprintf("%d,%s", y, r[z]))
	case 2:
		return fin("RES", fmt.Sprintf("%d,%s", y, r[z]))
	default: // x == 3
		return fin("SET", fmt.Sprintf("%d,%s", y, r[z]))
	}
}
