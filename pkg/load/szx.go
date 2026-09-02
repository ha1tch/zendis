package load

import (
	"fmt"

	"github.com/ha1tch/zentools/pkg/szx"
)

// szxSignature is the 4-byte magic .szx (zx-state) files open with,
// used to auto-detect the format the same way tzxSignature does for
// TZX -- unlike .sna/.z80, SZX has a real, reserved magic value.
const szxSignature = "ZXST"

// FromSZX loads a .szx (zx-state) snapshot, 48K or 128K-family.
// szx.Decode produces the identical *snapshot.MachineState type
// SNA/Z80 decoding already does, so no new flattening logic is needed
// here -- this dispatches to the same snapshot48KToCode/
// snapshot128ToCode either of those already use, exactly the way
// FromSnapshot already picks between them for SNA/Z80.
//
// Scope inherited from pkg/szx itself, not narrowed further here:
// peripheral state (AY sound, joysticks, disk controllers) that a
// real .szx file might carry is already dropped by szx.Decode before
// this function ever sees the result -- irrelevant for disassembly
// (this package only ever wanted CPU/paging/memory), but worth naming
// so "SZX support" doesn't imply more fidelity than what's actually
// used. The +3's special all-RAM paging mode is not rejected by
// szx.Decode itself (confirmed by reading it directly: it stores
// Port1FFD's raw byte with no interpretation of any bit) -- the
// rejection comes one layer up, from snapshot128ToCode below, the
// same ErrSpecialPagingMode check a Z80/SNA source in that state
// already gets. One consistent answer regardless of which format
// carried it, just enforced after decode rather than during it.
func FromSZX(image []byte, opts SnapshotOptions) (code []byte, org uint16, err error) {
	s, err := szx.Decode(image)
	if err != nil {
		return nil, 0, fmt.Errorf("load: decoding SZX: %w", err)
	}
	if s.Model.Is128KFamily() {
		return snapshot128ToCode(s, opts)
	}
	return snapshot48KToCode(s, opts)
}
