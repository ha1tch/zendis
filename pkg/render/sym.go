package render

import (
	"fmt"
	"io"
	"sort"

	"github.com/ha1tch/zendis/pkg/disasm"
)

// CollectSymbols builds the full name->address set a --sym file should
// contain: every resolved label (auto-generated or hints-file, from
// labelFor) plus every hints-file label whether or not it happens to be
// a branch target -- a label naming a data table's start address, say,
// is a real symbol worth exporting even though nothing in the listing
// jumps to it. hints may be nil (treated as empty).
func CollectSymbols(labelFor map[uint16]string, hints *disasm.Hints) map[string]uint16 {
	if hints == nil {
		hints = &disasm.Hints{}
	}
	symbols := make(map[string]uint16, len(labelFor)+len(hints.Labels))
	for addr, name := range labelFor {
		symbols[name] = addr
	}
	for addr, name := range hints.Labels {
		symbols[name] = addr
	}
	return symbols
}

// WritePasmoSym writes a pasmo-format symbol file to w (also zenas's
// own format -- zenas's --sym deliberately reuses pasmo's convention):
// NAME<tabs>EQU 0XXXXH, sorted by name, 4-digit zero-padded uppercase
// hex with a leading 0 and trailing H. The tab count matches pasmo's
// own real padding scheme (asm.cxx's tablabel: 2 tabs under 8 chars, 1
// tab for 8-15, a single space at 16+) rather than the always-2-tabs
// simplification zenas's own writer uses -- cosmetic either way (no
// parser cares about the padding), but this matches real pasmo output
// byte-for-byte for any name length.
func WritePasmoSym(w io.Writer, symbols map[string]uint16) error {
	names := make([]string, 0, len(symbols))
	for name := range symbols {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		pad := "\t\t"
		switch {
		case len(name) >= 16:
			pad = " "
		case len(name) >= 8:
			pad = "\t"
		}
		if _, err := fmt.Fprintf(w, "%s%sEQU 0%04XH\n", name, pad, symbols[name]); err != nil {
			return err
		}
	}
	return nil
}

// WriteSjasmplusSym writes an sjasmplus-format symbol file to w,
// straight from its own WriteLabelEquValue/PrintHex32 (sjio.cpp):
// NAME: EQU 0xVVVVVVVV, sorted by name, 0x-prefixed, 8 hex digits (not
// 4 -- confirmed via PrintHex(dest, value, 8) in its source). A
// completely different shape from pasmo's: colon-then-EQU rather than
// tab padding, 0x-prefix rather than H-suffix, double the digit width.
func WriteSjasmplusSym(w io.Writer, symbols map[string]uint16) error {
	names := make([]string, 0, len(symbols))
	for name := range symbols {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, err := fmt.Fprintf(w, "%s: EQU 0x%08X\n", name, symbols[name]); err != nil {
			return err
		}
	}
	return nil
}
