package disasm

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Hints holds user-supplied annotations for a disassembly: names for
// specific addresses (overriding generated labels), comments to attach
// to specific addresses, and byte ranges known to be data rather than
// code. Exported so any consumer of this package -- not just the
// zendis CLI -- can load and apply the same hints file, e.g. an
// embedder building its own listing view over another project's
// memory.
type Hints struct {
	// Labels overrides a generated Lxxxx label with a user-supplied
	// name for a branch target address.
	Labels map[uint16]string
	// Comments attaches a trailing comment to the instruction (or data
	// line) at a given address.
	Comments map[uint16]string
	// Data marks byte ranges as data rather than code, so a caller
	// doing a linear sweep can render them as DB lines instead of
	// attempting to decode them as instructions.
	Data []DataRange
	// Entry holds additional entry points beyond a caller's own
	// starting address. Parsed and stored, but not consumed by
	// anything in this package or by zendis's own CLI today -- reserved
	// for control-flow-guided traversal, which doesn't exist yet. A
	// caller doing linear decode can ignore this field entirely.
	Entry []uint16
}

// DataRange is a byte range, [Start, Start+Length), known to be data.
type DataRange struct {
	Start, Length uint16
}

// DataRangeAt reports whether addr falls within any marked data range,
// and if so, the range's end address (exclusive) -- the byte count to
// skip is int(end)-int(addr), computed in a wide enough type to avoid
// uint16 wraparound at the top of the address space.
func (h *Hints) DataRangeAt(addr uint16) (end uint16, ok bool) {
	if h == nil {
		return 0, false
	}
	for _, dr := range h.Data {
		if addr >= dr.Start && addr < dr.Start+dr.Length {
			return dr.Start + dr.Length, true
		}
	}
	return 0, false
}

// ParseHints parses a hints file. Each non-blank, non-comment line is
// one directive:
//
//	label   <addr> <name>
//	comment <addr> <text...>
//	data    <addr> <length>
//	entry   <addr>
//
// Addresses and lengths accept 0x-prefixed hex or plain decimal (Go's
// base-0 integer parsing). Comment text runs to the end of the line and
// may contain spaces. Blank lines and lines starting with # are
// ignored. A malformed line is reported as an error naming the line
// number; ParseHints stops at the first error rather than skipping bad
// lines silently.
func ParseHints(r io.Reader) (*Hints, error) {
	h := &Hints{
		Labels:   map[uint16]string{},
		Comments: map[uint16]string{},
	}
	sc := bufio.NewScanner(r)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.SplitN(line, " ", 3)
		directive := fields[0]
		switch directive {
		case "label":
			if len(fields) < 3 {
				return nil, fmt.Errorf("hints:%d: label needs an address and a name: %q", lineNo, line)
			}
			addr, err := parseHintAddr(fields[1])
			if err != nil {
				return nil, fmt.Errorf("hints:%d: %v", lineNo, err)
			}
			h.Labels[addr] = fields[2]
		case "comment":
			if len(fields) < 3 {
				return nil, fmt.Errorf("hints:%d: comment needs an address and text: %q", lineNo, line)
			}
			addr, err := parseHintAddr(fields[1])
			if err != nil {
				return nil, fmt.Errorf("hints:%d: %v", lineNo, err)
			}
			h.Comments[addr] = fields[2]
		case "data":
			parts := strings.Fields(line)
			if len(parts) != 3 {
				return nil, fmt.Errorf("hints:%d: data needs an address and a length: %q", lineNo, line)
			}
			addr, err := parseHintAddr(parts[1])
			if err != nil {
				return nil, fmt.Errorf("hints:%d: %v", lineNo, err)
			}
			length, err := parseHintAddr(parts[2])
			if err != nil {
				return nil, fmt.Errorf("hints:%d: %v", lineNo, err)
			}
			h.Data = append(h.Data, DataRange{Start: addr, Length: length})
		case "entry":
			parts := strings.Fields(line)
			if len(parts) != 2 {
				return nil, fmt.Errorf("hints:%d: entry needs an address: %q", lineNo, line)
			}
			addr, err := parseHintAddr(parts[1])
			if err != nil {
				return nil, fmt.Errorf("hints:%d: %v", lineNo, err)
			}
			h.Entry = append(h.Entry, addr)
		default:
			return nil, fmt.Errorf("hints:%d: unknown directive %q", lineNo, directive)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return h, nil
}

func parseHintAddr(s string) (uint16, error) {
	v, err := strconv.ParseUint(s, 0, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid address/length %q: %w", s, err)
	}
	return uint16(v), nil
}
