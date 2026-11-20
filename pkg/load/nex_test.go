package load

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ha1tch/zentools/pkg/nex"
)

// testNEX makes a NEX file: code at 0x8004 in bank 2 with the entry
// point at 0x8000 (four zeros ahead of the code), a table at the top of
// bank 0, and bank 20 with bytes in both of its pages.
func testNEX(t *testing.T) []byte {
	t.Helper()
	f := &nex.File{PC: 0x8000, SP: 0xFF00, Screen: map[uint8][]byte{}, Banks: map[int][]byte{}}
	bank2 := make([]byte, nex.BankLen)
	copy(bank2[4:], []byte{0xF3, 0x3E, 0x01, 0xC9})
	bank0 := make([]byte, nex.BankLen)
	copy(bank0[0x10:], []byte{1, 2, 3})
	bank20 := make([]byte, nex.BankLen)
	copy(bank20[0x100:], []byte{0xAA, 0xBB})
	copy(bank20[0x2000:], []byte{0xCC, 0xDD, 0xEE})
	f.Banks[2], f.Banks[0], f.Banks[20] = bank2, bank0, bank20
	img, err := nex.Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func u16(v uint16) *uint16 { return &v }
func num(v int) *int       { return &v }

func TestFromNEXDefaultView(t *testing.T) {
	code, org, err := FromNEX(testNEX(t), NEXOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// From the entry point (below the first used byte of bank 2) to the
	// last used byte of the view, which is the table in bank 0.
	if org != 0x8000 || len(code) != 0xC013-0x8000 {
		t.Fatalf("org %04X, %d bytes; want 8000, %d", org, len(code), 0xC013-0x8000)
	}
	if !bytes.Equal(code[4:8], []byte{0xF3, 0x3E, 0x01, 0xC9}) || !bytes.Equal(code[0x4010:], []byte{1, 2, 3}) {
		t.Errorf("the view's bytes are not the banks'")
	}
}

func TestFromNEXOrgAndLength(t *testing.T) {
	code, org, err := FromNEX(testNEX(t), NEXOptions{Options: Options{Org: u16(0x8004), Length: num(4)}})
	if err != nil || org != 0x8004 || !bytes.Equal(code, []byte{0xF3, 0x3E, 0x01, 0xC9}) {
		t.Fatalf("org %04X code % X err %v", org, code, err)
	}
	if _, _, err := FromNEX(testNEX(t), NEXOptions{Options: Options{Org: u16(0x2000)}}); err == nil {
		t.Error("an Org below the view is not an error")
	}
}

func TestFromNEXBankAndPage(t *testing.T) {
	img := testNEX(t)
	code, org, err := FromNEX(img, NEXOptions{Bank: num(20)})
	if err != nil || org != 0xC100 || len(code) != 0x2003-0x100 {
		t.Fatalf("bank 20: org %04X, %d bytes, err %v", org, len(code), err)
	}
	code, org, err = FromNEX(img, NEXOptions{Page: num(41), BankAt: u16(0x6000)})
	if err != nil || org != 0x6000 || !bytes.Equal(code, []byte{0xCC, 0xDD, 0xEE}) {
		t.Fatalf("page 41 at 6000: org %04X code % X err %v", org, code, err)
	}
	code, org, err = FromNEX(img, NEXOptions{Page: num(40), BankAt: u16(0xE000)})
	if err != nil || org != 0xE100 || !bytes.Equal(code, []byte{0xAA, 0xBB}) {
		t.Fatalf("page 40 at E000: org %04X code % X err %v", org, code, err)
	}
	for name, opts := range map[string]NEXOptions{
		"a bank the file has not":  {Bank: num(9)},
		"an address between slots": {Bank: num(20), BankAt: u16(0x9000)},
		"a bank that does not fit": {Bank: num(20), BankAt: u16(0xE000)},
		"a bank and a page":        {Bank: num(20), Page: num(40)},
		"BankAt with neither":      {BankAt: u16(0xC000)},
		"a page of zeros":          {Page: num(5)},
	} {
		if _, _, err := FromNEX(img, opts); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestDescribeNEX(t *testing.T) {
	info, err := DescribeNEX(testNEX(t))
	if err != nil {
		t.Fatal(err)
	}
	if info.PC != 0x8000 || len(info.Banks) != 3 {
		t.Fatalf("%+v", info)
	}
	want := []NEXBank{{Number: 0, First: 0x10, Last: 0x12, Address: 0xC000}, {Number: 2, First: 4, Last: 7, Address: 0x8000}, {Number: 20, First: 0x100, Last: 0x2002}}
	for i, b := range info.Banks {
		if b != want[i] {
			t.Errorf("bank %d: %+v, want %+v", i, b, want[i])
		}
	}
	var out bytes.Buffer
	if err := WriteNEXBanks(&out, testNEX(t)); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"entry point 0x8000", "bank  20  used 0x0100-0x2002", "at 0x8000 when the program starts"} {
		if !strings.Contains(out.String(), s) {
			t.Errorf("the listing lacks %q:\n%s", s, out.String())
		}
	}
}
