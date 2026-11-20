package load

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ha1tch/zentools/pkg/nex"
)

// NEXOptions controls loading from a NEX file, the Spectrum Next's
// executable: which part of it to take, plus the org/length overrides
// every loader shares.
//
// A NEX file is 16 KB banks. Three of them are in the Z80's address
// space when the program starts: bank 5 at 0x4000, bank 2 at 0x8000 and
// the header's entry bank (0 unless the file says otherwise) at 0xC000.
// That is the view taken when neither Bank nor Page is set. Any other
// bank is paged in by the program itself, where it chooses, so it is
// taken one at a time: Bank or Page names it and BankAt says where its
// first byte is.
type NEXOptions struct {
	Options

	// Bank selects one 16 KB bank (0-111) of the file.
	Bank *int
	// Page selects one 8 KB page (0-223): page 2n is the first half of
	// bank n and page 2n+1 the second, as the Next's MMU numbers them.
	// Bank and Page are not given together.
	Page *int
	// BankAt is the address of the first byte of the bank or page: a
	// multiple of 0x2000, as the MMU's slots are. nil is 0xC000.
	BankAt *uint16
}

// NEXInfo is what a NEX file holds, for a caller that has to choose a
// part of it.
type NEXInfo struct {
	Version   int // 2 or 3, for V1.2 or V1.3
	PC, SP    uint16
	EntryBank int // the bank at 0xC000 when the program starts
	Banks     []NEXBank
}

// NEXBank is one bank of a NEX file. Used is how much of it is not
// zero: the offsets of its first and last byte that are not, with
// First > Last for a bank of zeros only.
type NEXBank struct {
	Number      int
	First, Last int
	// Address is where the bank is when the program starts, for the
	// three that are in the address space then; 0 for the others.
	Address uint16
}

// Empty reports a bank that is in the file and holds zeros only.
func (b NEXBank) Empty() bool { return b.First > b.Last }

// DescribeNEX reads a NEX file's header and says which banks it has.
func DescribeNEX(image []byte) (*NEXInfo, error) {
	f, err := nex.Decode(image)
	if err != nil {
		return nil, fmt.Errorf("load: decoding NEX: %w", err)
	}
	info := &NEXInfo{Version: f.Version, PC: f.PC, SP: f.SP, EntryBank: int(f.EntryBank)}
	numbers := make([]int, 0, len(f.Banks))
	for n := range f.Banks {
		numbers = append(numbers, n)
	}
	sort.Ints(numbers)
	for _, n := range numbers {
		first, last := used(f.Banks[n])
		b := NEXBank{Number: n, First: first, Last: last}
		switch n {
		case 5:
			b.Address = 0x4000
		case 2:
			b.Address = 0x8000
		case int(f.EntryBank):
			b.Address = 0xC000
		}
		info.Banks = append(info.Banks, b)
	}
	return info, nil
}

// used returns the offsets of the first and the last byte of b that are
// not zero; first > last when there is none.
func used(b []byte) (first, last int) {
	first, last = len(b), -1
	for i, c := range b {
		if c != 0 {
			if i < first {
				first = i
			}
			last = i
		}
	}
	return first, last
}

// FromNEX extracts code from a NEX file.
//
// With neither Bank nor Page, the result is from the view the program
// starts with. A bank is always 16 KB in the file, most of it zeros
// that were never part of the program, so the default is what is used:
// from the first byte that is not zero in the bank the entry point is
// in, or the entry point if that is lower (in the whole view when the
// file has no entry point, PC 0), to the last one of the view. Org moves the start; Length the end.
//
// With Bank or Page, the result is the used part of that bank or page,
// at BankAt.
func FromNEX(image []byte, opts NEXOptions) (code []byte, org uint16, err error) {
	f, err := nex.Decode(image)
	if err != nil {
		return nil, 0, fmt.Errorf("load: decoding NEX: %w", err)
	}
	if opts.Bank != nil && opts.Page != nil {
		return nil, 0, fmt.Errorf("load: Bank and Page are not given together")
	}

	var mem []byte // the bytes of the view
	base := 0      // the address of mem[0]
	lo := -1       // offset in mem of the default start; -1: the first used byte
	switch {
	case opts.Bank != nil || opts.Page != nil:
		number, what := 0, "bank"
		if opts.Bank != nil {
			number = *opts.Bank
		} else {
			number, what = *opts.Page/2, "page"
			if *opts.Page < 0 {
				number = -1
			}
		}
		bank, ok := f.Banks[number]
		if !ok {
			if what == "page" {
				return nil, 0, fmt.Errorf("load: the NEX file has no page %d (bank %d); it has banks %s", *opts.Page, number, bankList(f))
			}
			return nil, 0, fmt.Errorf("load: the NEX file has no bank %d; it has banks %s", number, bankList(f))
		}
		mem = bank
		if opts.Page != nil {
			half := (*opts.Page % 2) * (nex.BankLen / 2)
			mem = bank[half : half+nex.BankLen/2]
		}
		base = 0xC000
		if opts.BankAt != nil {
			base = int(*opts.BankAt)
		}
		if base%0x2000 != 0 || base+len(mem) > 0x10000 {
			return nil, 0, fmt.Errorf("load: a %s cannot be at 0x%04X: its address has to be a multiple of 0x2000 that leaves room for its %d bytes", what, base, len(mem))
		}
	default:
		if opts.BankAt != nil {
			return nil, 0, fmt.Errorf("load: BankAt is for a Bank or a Page")
		}
		base = 0x4000
		mem = make([]byte, 3*nex.BankLen)
		for slot, n := range []int{5, 2, int(f.EntryBank)} {
			copy(mem[slot*nex.BankLen:], f.Banks[n])
		}
		if f.PC >= 0x4000 {
			slot := (int(f.PC) - base) / nex.BankLen
			first, last := used(mem[slot*nex.BankLen : (slot+1)*nex.BankLen])
			if first <= last {
				lo = slot*nex.BankLen + first
				// The entry point is part of the program even when the
				// bytes there are zeros (NOPs).
				if pc := int(f.PC) - base; pc < lo {
					lo = pc
				}
			}
		}
	}

	first, last := used(mem)
	if first > last {
		return nil, 0, fmt.Errorf("load: that part of the NEX file holds zeros only")
	}
	if lo < 0 {
		lo = first
	}
	if opts.Org != nil {
		lo = int(*opts.Org) - base
		if lo < 0 || lo >= len(mem) {
			return nil, 0, fmt.Errorf("load: Org 0x%04X is outside 0x%04X-0x%04X, where that part of the NEX file is", *opts.Org, base, base+len(mem)-1)
		}
	}
	hi := last + 1
	if hi <= lo {
		hi = len(mem) // an Org past what is used: everything that follows it
	}
	if opts.Length != nil {
		// A length may ask for more than is used, up to the end of the view.
		hi = len(mem)
	}
	n, err := opts.clampLength(hi - lo)
	if err != nil {
		return nil, 0, err
	}
	return mem[lo : lo+n], uint16(base + lo), nil
}

func bankList(f *nex.File) string {
	numbers := make([]int, 0, len(f.Banks))
	for n := range f.Banks {
		numbers = append(numbers, n)
	}
	sort.Ints(numbers)
	parts := make([]string, len(numbers))
	for i, n := range numbers {
		parts[i] = fmt.Sprint(n)
	}
	return strings.Join(parts, ", ")
}
