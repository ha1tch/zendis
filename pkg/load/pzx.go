package load

import (
	"fmt"

	"github.com/ha1tch/zentools/pkg/pzx"
	"github.com/ha1tch/zentools/pkg/tap"
)

// pzxSignature is the 4-byte magic PZX files open with (the PZXT
// header block's own tag, which Decode enforces must be first).
const pzxSignature = "PZXT"

// pzxBlocksFor decodes a PZX file's DATA blocks into the same
// []tap.Block shape FromTAP/FromTZX already select from, letting
// selectBlock's existing BlockIndex/BlockName/default-to-Code logic
// serve PZX too without any duplication.
//
// A DATA block's payload is already fully-resolved bytes by the time
// PZX's own decoder produces it, regardless of what pulse timing
// represented them physically on tape -- standard Spectrum ROM
// timing, a turbo loader, anything else. Unlike pzxToTAP in the
// sibling zentools project (which has to gate on standard timing when
// reconstructing a genuinely re-loadable TAP file), extraction here
// only needs the bytes themselves, so no timing check applies -- the
// same insight, and the same design, as zentools's own PZX -> bin
// conversion path, which this mirrors rather than reinvents.
func pzxBlocksFor(image []byte) ([]tap.Block, error) {
	f, err := pzx.Decode(image)
	if err != nil {
		return nil, fmt.Errorf("load: decoding PZX: %w", err)
	}
	var blocks []tap.Block
	for _, block := range f.Blocks {
		db, ok := block.(pzx.DataBlock)
		if !ok {
			continue // PZXT, PULS, PAUS, BRWS, STOP, or a custom RawBlock -- no payload of its own
		}
		tb, err := tap.DecodeBlock(db.Data)
		if err != nil {
			continue // not a TAP-shaped payload -- not selectable, not a file-level error
		}
		blocks = append(blocks, tb)
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("load: no TAP-shaped blocks found in this PZX file's DATA blocks")
	}
	return blocks, nil
}

// FromPZX extracts one block's payload from a PZX image.
func FromPZX(image []byte, opts TapeOptions) (code []byte, org uint16, err error) {
	blocks, err := pzxBlocksFor(image)
	if err != nil {
		return nil, 0, err
	}
	return selectBlock(blocks, opts)
}
