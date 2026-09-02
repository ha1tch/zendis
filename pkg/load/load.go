// Package load extracts a (code, org) pair -- the same shape
// pkg/render.DecodeBuffer already takes -- from real ZX Spectrum
// container formats: tape images (.tap, .tzx, .pzx), 48K and
// 128K-family machine snapshots (.sna, .z80, .szx), and RZX
// input-recording files (via their embedded snapshot). Built on
// github.com/ha1tch/zentools, which already parses these formats;
// this package's own job is picking which piece of a tape, snapshot,
// or recording to disassemble and resolving its load address, not
// re-parsing bytes zentools already understands.
package load

// Options controls the org/length overrides every loader in this
// package shares: both tape and snapshot loading produce a (code, org)
// pair, and both need the same "does the caller want the source's own
// answer, or an explicit override" mechanism. Pointers, not zero-value
// sentinels: a valid org or length can legitimately be 0, so there's
// no unused value available to mean "not set" the way there would be
// for, say, a string.
type Options struct {
	// Org overrides the source's own load address (a tape block's
	// header) or resolved PC (a snapshot) when non-nil.
	Org *uint16

	// Length limits how many bytes are returned, starting at the
	// resolved org, when non-nil. nil means "everything the source
	// itself provides": a tape block's own data length, or a 48K
	// snapshot's span from org to the end of its 64K address space
	// (0x10000 - org bytes).
	Length *int
}

// clampLength returns the byte count to actually return: min(available,
// the requested length if set), and an error if a caller explicitly
// asked for more than is available -- silently truncating a deliberate
// over-request would hide a real mistake (a org/length combination that
// doesn't fit the source) rather than surface it.
func (o Options) clampLength(available int) (int, error) {
	if o.Length == nil {
		return available, nil
	}
	if *o.Length < 0 {
		return 0, errInvalidLength
	}
	if *o.Length > available {
		return 0, errLengthExceedsAvailable(*o.Length, available)
	}
	return *o.Length, nil
}
