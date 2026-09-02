package load

import (
	"errors"
	"fmt"
)

// ErrNoCodeBlock is returned by FromTAP/FromTZX/FromTape when no block
// index or name was given and auto-selection found no TypeCode header
// on the tape at all -- there's nothing sensible to disassemble by
// default.
var ErrNoCodeBlock = errors.New("load: no Code (TypeCode) block found on tape; specify BlockIndex or BlockName")

// ErrBlockNotFound is returned when an explicitly requested block
// index or name doesn't match anything decoded from the tape.
var ErrBlockNotFound = errors.New("load: requested block not found")

// ErrHeaderWithoutData is returned when a selected header block has no
// following data block to pair it with -- a malformed or truncated
// tape image.
var ErrHeaderWithoutData = errors.New("load: header block has no following data block")

// ErrNoStandardDataBlocks is returned by FromTZX (and FromTape when it
// dispatches to FromTZX) when a TZX image contains no ID 0x10, 0x11,
// or 0x14 (Standard/Turbo Speed Data, Pure Data) blocks at all --
// only pilot tones, raw pulse sequences, Direct Recording, or other
// genuinely pulse-level block types, which carry no load-address
// concept this package interprets (see FromTZX's own doc comment).
// Distinct from ErrNoCodeBlock, which means data-shaped blocks
// existed but none had a Code header: this means there was nothing to
// even search.
var ErrNoStandardDataBlocks = errors.New("load: TZX has no data-shaped (0x10/0x11/0x14) blocks; only pilot tones, raw pulse sequences, or other pulse-level blocks are present, which this package does not interpret for load-address purposes")

var errInvalidLength = errors.New("load: Length must not be negative")

func errLengthExceedsAvailable(requested, available int) error {
	return fmt.Errorf("load: requested length %d exceeds the %d bytes available", requested, available)
}
