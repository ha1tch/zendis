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

// DataRange is a byte range, [Start, Start+Length), known to be data,
// and how to write it.
type DataRange struct {
	Start, Length uint16
	Kind          DataKind
}

// DataKind is the way a data range is written: which of an assembler's
// data directives its bytes become.
type DataKind int

const (
	// DataBytes is one number per byte: DB 0x3E,0x01. The zero value.
	DataBytes DataKind = iota
	// DataHex is zenas's compact form, DB 0x 3E 01 C9. The other
	// assemblers have no such form and get DataBytes.
	DataHex
	// DataWords is 16-bit little-endian words: DW 0x8000. A last odd
	// byte is a DB.
	DataWords
	// DataText is quoted strings where the bytes are printable ASCII and
	// numbers where they are not: DB "SCORE",0x0D.
	DataText
	// DataFill writes a run of four or more equal bytes as DS count,value
	// and the rest as DataBytes.
	DataFill
)

// dataKindNames is what a hints file and the command line call each kind.
var dataKindNames = []string{"bytes", "hex", "words", "text", "fill"}

// String is the kind's name in a hints file.
func (k DataKind) String() string {
	if k < 0 || int(k) >= len(dataKindNames) {
		return fmt.Sprintf("DataKind(%d)", int(k))
	}
	return dataKindNames[k]
}

// ParseDataKind reads a kind's name: bytes, hex, words, text or fill.
func ParseDataKind(s string) (DataKind, error) {
	for i, name := range dataKindNames {
		if strings.EqualFold(s, name) {
			return DataKind(i), nil
		}
	}
	return DataBytes, fmt.Errorf("unknown data kind %q (want %s)", s, strings.Join(dataKindNames, ", "))
}

// ParseDataRange reads a data range written as ADDR:LENGTH or
// ADDR:LENGTH:KIND, the form of the command's --data: numbers decimal
// or 0x hex, the kind one of ParseDataKind's, bytes if there is none.
func ParseDataRange(s string) (DataRange, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 && len(parts) != 3 {
		return DataRange{}, fmt.Errorf("data range %q: want ADDR:LENGTH or ADDR:LENGTH:KIND", s)
	}
	addr, err := parseHintAddr(strings.TrimSpace(parts[0]))
	if err != nil {
		return DataRange{}, err
	}
	length, err := parseHintAddr(strings.TrimSpace(parts[1]))
	if err != nil {
		return DataRange{}, err
	}
	kind := DataBytes
	if len(parts) == 3 {
		if kind, err = ParseDataKind(strings.TrimSpace(parts[2])); err != nil {
			return DataRange{}, err
		}
	}
	return DataRange{Start: addr, Length: length, Kind: kind}, nil
}

// DataAt returns the data that starts at addr: the range addr falls in,
// cut to begin at addr and to end where a range given before it begins.
// Where ranges overlap the first one given wins, as with DataRangeAt,
// so a range inside a larger one given after it keeps its own kind and
// the larger one goes on after it.
func (h *Hints) DataAt(addr uint16) (DataRange, bool) {
	if h == nil {
		return DataRange{}, false
	}
	for i, dr := range h.Data {
		end := int(dr.Start) + int(dr.Length)
		if int(addr) < int(dr.Start) || int(addr) >= end {
			continue
		}
		for _, before := range h.Data[:i] {
			if s := int(before.Start); s > int(addr) && s < end {
				end = s
			}
		}
		return DataRange{Start: addr, Length: uint16(end - int(addr)), Kind: dr.Kind}, true
	}
	return DataRange{}, false
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
//	data    <addr> <length> [bytes|hex|words|text|fill]
//	entry   <addr>
//
// The word after a data range's length is how it is written (see
// DataKind); bytes if there is none.
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
			if len(parts) != 3 && len(parts) != 4 {
				return nil, fmt.Errorf("hints:%d: data needs an address, a length and at most a kind: %q", lineNo, line)
			}
			addr, err := parseHintAddr(parts[1])
			if err != nil {
				return nil, fmt.Errorf("hints:%d: %v", lineNo, err)
			}
			length, err := parseHintAddr(parts[2])
			if err != nil {
				return nil, fmt.Errorf("hints:%d: %v", lineNo, err)
			}
			kind := DataBytes
			if len(parts) == 4 {
				if kind, err = ParseDataKind(parts[3]); err != nil {
					return nil, fmt.Errorf("hints:%d: %v", lineNo, err)
				}
			}
			h.Data = append(h.Data, DataRange{Start: addr, Length: length, Kind: kind})
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
