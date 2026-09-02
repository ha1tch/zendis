package load

import (
	"fmt"
	"strings"

	"github.com/ha1tch/zentools/pkg/tap"
	"github.com/ha1tch/zentools/pkg/tzx"
)

// tzxSignature is the 7-byte magic TZX files open with (followed by an
// EOF marker byte and a 2-byte version), used to distinguish a TZX
// image from a bare TAP image without relying on a file extension.
const tzxSignature = "ZXTape!"

// TapeOptions controls tape loading: which block to extract, plus the
// org/length overrides shared with snapshot loading.
type TapeOptions struct {
	Options

	// BlockIndex selects a block by its 0-based position in the flat
	// block list tap.Decode (or the TZX 0x10 unwrap) returns -- see
	// selectBlock's own doc for why this is the raw list, not a
	// derived header+data pairing. Ignored when BlockName is set.
	BlockIndex *int

	// BlockName selects a header block by exact name match (already
	// space-trimmed by zentools's own decoder), using its immediately
	// following data block. Takes precedence over BlockIndex when both
	// are set. Only header blocks have names, so this can never select
	// a bare data block directly -- use BlockIndex for that.
	BlockName string
}

// selectBlock picks one block from the raw, flat block list per opts,
// resolves its org, and returns its payload.
//
// BlockIndex/BlockName both operate on the same flat list tap.Decode
// (or the TZX 0x10 unwrap) returns -- not a header+data pairing --
// because real tapes don't reliably pair one-to-one. Confirmed against
// a real commercial tape image, not assumed: a common pattern is a
// single header block for a small BASIC loader, followed by several
// bare data blocks with no headers at all, meant for a custom loading
// routine that already knows their addresses from its own code, not
// from tape structure. Indexing into a derived pairing would make
// those bare blocks completely unreachable.
//
//   - BlockName matches a header block's Name; its immediately
//     following data block (if any) is returned, org defaulting to
//     the header's own Param1.
//   - BlockIndex selects a block directly by its position in the flat
//     list. If it's a header, behaves like BlockName above. If it's a
//     bare data block, its own payload is returned directly -- but
//     with no header, there is no load address to default to, so
//     Options.Org must be set, or this returns a clear error rather
//     than silently defaulting to 0.
//   - Neither given: the first header block whose Type is
//     tap.TypeCode (unchanged from the original design) -- a
//     reasonable default, not a guarantee of finding the "main" code
//     block on a tape with several TypeCode headers (a screen or data
//     buffer block can legitimately come first; override with
//     BlockName/BlockIndex when it does).
func selectBlock(blocks []tap.Block, opts TapeOptions) (code []byte, org uint16, err error) {
	switch {
	case opts.BlockName != "":
		want := strings.TrimRight(opts.BlockName, " ")
		for i, b := range blocks {
			if b.IsHeader && strings.TrimRight(b.Name, " ") == want {
				return fromHeader(blocks, i, opts)
			}
		}
		return nil, 0, fmt.Errorf("%w: name %q", ErrBlockNotFound, opts.BlockName)

	case opts.BlockIndex != nil:
		idx := *opts.BlockIndex
		if idx < 0 || idx >= len(blocks) {
			return nil, 0, fmt.Errorf("%w: index %d (tape has %d block(s))", ErrBlockNotFound, idx, len(blocks))
		}
		if blocks[idx].IsHeader {
			return fromHeader(blocks, idx, opts)
		}
		if opts.Org == nil {
			return nil, 0, fmt.Errorf("load: block %d is a bare data block with no header -- Org must be set explicitly", idx)
		}
		n, err := opts.clampLength(len(blocks[idx].Data))
		if err != nil {
			return nil, 0, err
		}
		return blocks[idx].Data[:n], *opts.Org, nil

	default:
		for i, b := range blocks {
			if b.IsHeader && b.Type == tap.TypeCode {
				return fromHeader(blocks, i, opts)
			}
		}
		return nil, 0, ErrNoCodeBlock
	}
}

// fromHeader resolves the data block immediately following the header
// at blocks[headerIdx], applying opts' org override and length clamp.
func fromHeader(blocks []tap.Block, headerIdx int, opts TapeOptions) (code []byte, org uint16, err error) {
	header := blocks[headerIdx]
	if headerIdx+1 >= len(blocks) || blocks[headerIdx+1].IsHeader {
		return nil, 0, fmt.Errorf("%w: %q", ErrHeaderWithoutData, strings.TrimRight(header.Name, " "))
	}
	data := blocks[headerIdx+1].Data

	org = header.Param1
	if opts.Org != nil {
		org = *opts.Org
	}
	n, err := opts.clampLength(len(data))
	if err != nil {
		return nil, 0, err
	}
	return data[:n], org, nil
}

// FromTAP extracts one block's payload from a TAP image.
func FromTAP(image []byte, opts TapeOptions) (code []byte, org uint16, err error) {
	blocks, err := tap.Decode(image)
	if err != nil {
		return nil, 0, fmt.Errorf("load: decoding TAP: %w", err)
	}
	return selectBlock(blocks, opts)
}

// FromTZX extracts one block's payload from a TZX image. IDs 0x10
// (Standard Speed Data), 0x11 (Turbo Speed Data), and 0x14 (Pure
// Data) are all considered for selection -- zentools's own tzx.Block
// doc comment on 0x11 is explicit that its Data field "holds the raw
// payload bytes themselves, the same field 0x10 uses for its own
// payload", and 0x14 is documented as the same shape again. All three
// unwrap identically here via tap.DecodeBlock. Every other TZX block
// type (Pure Tone, raw pulse sequences, Direct Recording,
// Generalized Data, and the rest) is a pulse/timing-level construct
// with no clean load-address concept the way a standard header
// carries one -- skipped for selection purposes, not partially
// interpreted. This is the scope T-10 was filed with, not an
// oversight: if every block on a TZX is one of those pulse-level
// types, FromTZX reports ErrNoStandardDataBlocks -- distinct from
// ErrNoCodeBlock, which means data-shaped blocks existed but none had
// a Code header (or ErrBlockNotFound for an explicit index/name that
// didn't match) -- rather than guessing at a pulse block's payload.
//
// An earlier version of this function kept only 0x10, which silently
// dropped every turbo-loaded block on any real TZX using one -- a
// large fraction of real commercial tapes, which is precisely why
// turbo loaders exist in the first place (faster loading than the
// standard ROM routine allows). Found and fixed for the identical
// reason in the sibling zentools project's own TZX -> tape conversion
// path first; this brings zendis's own loading up to the same
// standard rather than leaving a known limitation in place here.
func FromTZX(image []byte, opts TapeOptions) (code []byte, org uint16, err error) {
	tzxBlocks, err := tzx.Decode(image)
	if err != nil {
		return nil, 0, fmt.Errorf("load: decoding TZX: %w", err)
	}
	var blocks []tap.Block
	for _, b := range tzxBlocks {
		switch b.ID {
		case 0x10, 0x11, 0x14:
		default:
			continue
		}
		tb, err := tap.DecodeBlock(b.Data)
		if err != nil {
			return nil, 0, fmt.Errorf("load: decoding TZX block 0x%02X as TAP: %w", b.ID, err)
		}
		blocks = append(blocks, tb)
	}
	if len(blocks) == 0 {
		return nil, 0, ErrNoStandardDataBlocks
	}
	return selectBlock(blocks, opts)
}

// FromTape extracts one block's payload from a TAP, TZX, or PZX
// image, detected from the image's own bytes (PZX's 4-byte "PZXT"
// signature, or the 7-byte "ZXTape!" TZX opens with) rather than a
// file extension -- a convenience for a caller that doesn't already
// know which format it has.
func FromTape(image []byte, opts TapeOptions) (code []byte, org uint16, err error) {
	if len(image) >= len(pzxSignature) && string(image[:len(pzxSignature)]) == pzxSignature {
		return FromPZX(image, opts)
	}
	if len(image) >= len(tzxSignature) && string(image[:len(tzxSignature)]) == tzxSignature {
		return FromTZX(image, opts)
	}
	return FromTAP(image, opts)
}
