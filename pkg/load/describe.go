package load

import (
	"fmt"
	"io"

	"github.com/ha1tch/zentools/pkg/tap"
	"github.com/ha1tch/zentools/pkg/tzx"
)

// TapeBlocks returns the flat block list of a TAP, TZX or PZX image: the
// list TapeOptions.BlockIndex counts in.
func TapeBlocks(image []byte) ([]tap.Block, error) {
	if len(image) >= len(pzxSignature) && string(image[:len(pzxSignature)]) == pzxSignature {
		return pzxBlocksFor(image)
	}
	if len(image) >= len(tzxSignature) && string(image[:len(tzxSignature)]) == tzxSignature {
		tzxBlocks, err := tzx.Decode(image)
		if err != nil {
			return nil, fmt.Errorf("load: decoding TZX: %w", err)
		}
		var blocks []tap.Block
		for _, b := range tzxBlocks {
			switch b.ID {
			case 0x10, 0x11, 0x14:
				tb, err := tap.DecodeBlock(b.Data)
				if err != nil {
					return nil, fmt.Errorf("load: decoding TZX block 0x%02X as TAP: %w", b.ID, err)
				}
				blocks = append(blocks, tb)
			}
		}
		if len(blocks) == 0 {
			return nil, ErrNoStandardDataBlocks
		}
		return blocks, nil
	}
	blocks, err := tap.Decode(image)
	if err != nil {
		return nil, fmt.Errorf("load: decoding TAP: %w", err)
	}
	return blocks, nil
}

// WriteTapeBlocks lists a tape's blocks, one a line, with the number
// BlockIndex takes and, for a header, what it says of the block after it.
func WriteTapeBlocks(w io.Writer, image []byte) error {
	blocks, err := TapeBlocks(image)
	if err != nil {
		return err
	}
	types := map[byte]string{tap.TypeProgram: "Program", tap.TypeNumArray: "Number array", tap.TypeCharArray: "Character array", tap.TypeCode: "Code"}
	fmt.Fprintf(w, "%d block(s). --block=N takes a block by its number here; --block-name=NAME a header by its name.\n", len(blocks))
	for i, b := range blocks {
		if !b.IsHeader {
			fmt.Fprintf(w, "%3d  data    %5d bytes, flag 0x%02X\n", i, len(b.Data), b.Flag)
			continue
		}
		kind, ok := types[b.Type]
		if !ok {
			kind = fmt.Sprintf("type %d", b.Type)
		}
		line := fmt.Sprintf("%3d  header  %-10q %s, %d bytes", i, b.Name, kind, b.DataLength)
		switch b.Type {
		case tap.TypeCode:
			line += fmt.Sprintf(", loads at 0x%04X", b.Param1)
		case tap.TypeProgram:
			if b.Param1 < 0x8000 {
				line += fmt.Sprintf(", runs from line %d", b.Param1)
			}
		}
		fmt.Fprintln(w, line)
	}
	return nil
}

// WriteNEXBanks lists what a NEX file holds: its entry point and its
// banks, with where each is when the program starts and how much of it
// is used.
func WriteNEXBanks(w io.Writer, image []byte) error {
	info, err := DescribeNEX(image)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "NEX V1.%d, ", info.Version)
	if info.PC == 0 {
		fmt.Fprintf(w, "no entry point (loaded and not run)")
	} else {
		fmt.Fprintf(w, "entry point 0x%04X", info.PC)
	}
	fmt.Fprintf(w, ", stack 0x%04X, bank %d at 0xC000. %d bank(s):\n", info.SP, info.EntryBank, len(info.Banks))
	for _, b := range info.Banks {
		where := "paged in by the program (--bank, --bank-at)"
		if b.Address != 0 {
			where = fmt.Sprintf("at 0x%04X when the program starts", b.Address)
		}
		if b.Empty() {
			fmt.Fprintf(w, "  bank %3d  zeros only                 %s\n", b.Number, where)
			continue
		}
		fmt.Fprintf(w, "  bank %3d  used 0x%04X-0x%04X (%5d)  %s\n", b.Number, b.First, b.Last, b.Last-b.First+1, where)
	}
	return nil
}
