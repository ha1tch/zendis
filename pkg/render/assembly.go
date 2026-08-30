package render

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/ha1tch/zendis/pkg/disasm"
)

// Dialect selects which assembler's syntax WriteAssemblySource
// targets. All five share one renderer for the ordinary instruction
// body -- investigating pasmo's and sjasmplus's own source found no
// reason for the instruction/DB/label/comment text itself to differ
// between any of them (0x hex, DB/DEFB, an optional trailing colon on
// labels, and ";" comments are all pre-registered, case-flexible forms
// in both). Dialect only changes the small, specific divergences each
// real assembler was confirmed to need.
type Dialect string

const (
	DialectPlain     Dialect = "plain" // bare, zenas's own conventions; DialectZenas is an identical alias
	DialectZenas     Dialect = "zenas"
	DialectPasmo     Dialect = "pasmo"
	DialectSjasmplus Dialect = "sjasmplus"
	DialectSnasm     Dialect = "snasm"
)

// WriteAssemblySource renders a bare, re-assemblable form to w: an ORG
// directive, one instruction or DB line per record, no address/hex
// columns, and a "Name:" declaration at every labeled address -- an
// operand referencing "Start" only assembles if "Start:" exists
// somewhere in the same source, so a label always gets one, whether it
// came from the hints file or was generated. hints may be nil (treated
// as empty).
//
// z80nTarget should reflect whether records was decoded under
// disasm.TargetZ80N -- SNASM needs to know this for its own mode
// preamble (see below).
//
// warnOut receives the per-instruction fallback warnings
// pasmo/snasm can trigger (pass io.Discard to suppress them).
//
// Per-dialect divergences, each confirmed against the real assembler,
// not assumed:
//
//   - DialectPasmo: a Z80N instruction, an undocumented bare
//     IN (C)/OUT (C),0, or DDCB/FDCB's undocumented copy-to-register
//     form -- pasmo has none of these -- falls back to a DB line of
//     raw bytes with a warning naming the reason (PasmoUnrepresentable).
//
//   - DialectSjasmplus: indents every non-label line (real sjasmplus
//     parses an unindented directive at column 0 as a label by
//     default -- confirmed against the compiled binary: "Unrecognized
//     instruction: 0x8000", ORG consumed as a label; labels themselves
//     stay unindented, that's how they're recognized) and uppercases
//     the IXh/IXl half-register spelling zendis shares with zenas and
//     pasmo, since real sjasmplus only accepts all-caps IXH/IYL
//     (confirmed: "Label not found: IXh", parsed as an undefined label
//     reference, not the register).
//
//   - DialectSnasm (Mike Dailly's SNASM): converts every 0x literal to
//     $ (its only accepted hex prefix -- "0x8000" fails outright,
//     "Label next to number without white space"); emits an
//     "OPT Z80"/"OPT ZXNEXT" mode directive first, which SNASM requires
//     and no other dialect here does ("Symbol LD already defined"
//     without it); rewrites "MUL D,E" to bare "MUL" (SNASM's Z80N MUL
//     takes no operand syntax at all -- every other Z80N form checked
//     matched zendis's rendering unchanged); and falls back to DB for
//     the same three undocumented categories as pasmo
//     (SnasmUnrepresentable) -- with the DDCB/FDCB rotate-group case
//     being the more important one to catch, since SNASM doesn't
//     reject that syntax, it silently emits the wrong opcode with no
//     warning at all.
func WriteAssemblySource(w io.Writer, records []Record, labelFor map[uint16]string, hints *disasm.Hints, org uint16, dialect Dialect, z80nTarget bool, warnOut io.Writer) {
	if hints == nil {
		hints = &disasm.Hints{}
	}
	if warnOut == nil {
		warnOut = io.Discard
	}

	pasmoFallback := dialect == DialectPasmo
	snasmFallback := dialect == DialectSnasm
	indent := ""
	if dialect == DialectSjasmplus {
		indent = "    "
	}

	// SNASM needs $-prefixed hex, not 0x, which is the one dialect
	// where the shared renderer's literal format doesn't just work --
	// rather than thread a conversion through every individual write
	// call below, buffer this dialect's whole output locally and
	// post-process it once at the end; the other four dialects write
	// straight through to w as before.
	target := w
	var buf *bytes.Buffer
	if snasmFallback {
		buf = &bytes.Buffer{}
		target = buf
	}

	if snasmFallback {
		if z80nTarget {
			fmt.Fprintln(target, "OPT ZXNEXT")
		} else {
			fmt.Fprintln(target, "OPT Z80")
		}
	}

	fmt.Fprintf(target, "%sORG 0x%04X\n", indent, org)
	for _, rec := range records {
		if label, ok := labelFor[rec.Address]; ok {
			fmt.Fprintf(target, "%s:\n", label)
		}
		if rec.Instr != nil {
			in := rec.Instr
			if pasmoFallback || snasmFallback {
				checkFn := PasmoUnrepresentable
				dialectName := "pasmo"
				if snasmFallback {
					checkFn = SnasmUnrepresentable
					dialectName = "SNASM"
				}
				if unrepresentable, reason := checkFn(in); unrepresentable {
					fmt.Fprintf(warnOut, "zendis: WARN %04X: %s is %s (%s) -- %s can't represent it, falling back to DB\n",
						in.Address, hexDump(in.Bytes), reason, in.Text(), dialectName)
					line := indent + "DB " + hexList(in.Bytes)
					if c, ok := hints.Comments[in.Address]; ok {
						line += "  ; " + c
					}
					fmt.Fprintln(target, line)
					continue
				}
			}
			text := resolvedText(in, labelFor)
			if dialect == DialectSjasmplus {
				text = sjasmplusHalfRegisterFix(text)
			}
			if snasmFallback {
				text = snasmMulFix(text)
			}
			line := indent + text
			if c, ok := hints.Comments[in.Address]; ok {
				line += "  ; " + c
			}
			fmt.Fprintln(target, line)
			continue
		}
		line := indent + "DB " + hexList(rec.Bytes)
		if !rec.Truncated {
			if c, ok := hints.Comments[rec.Address]; ok {
				line += "  ; " + c
			}
		}
		fmt.Fprintln(target, line)
	}

	if buf != nil {
		fmt.Fprint(w, snasmLiteralFix(buf.String()))
	}
}

// sjasmplusHalfRegisterFix uppercases the IX/IY half-register suffix --
// confirmed against the real sjasmplus binary: it does not recognize
// mixed-case "IXh"/"IXl" (fails with "Label not found: IXh", parsing it
// as an undefined label reference, not the register), only all-caps
// "IXH"/"IXL". Real pasmo accepts the mixed-case form fine, so this is
// specifically an sjasmplus quirk, applied only for that dialect. Safe
// as an unconditional substring replace: these four exact substrings
// never appear anywhere else in zendis's rendered vocabulary.
func sjasmplusHalfRegisterFix(text string) string {
	text = strings.ReplaceAll(text, "IXh", "IXH")
	text = strings.ReplaceAll(text, "IXl", "IXL")
	text = strings.ReplaceAll(text, "IYh", "IYH")
	text = strings.ReplaceAll(text, "IYl", "IYL")
	return text
}

// snasmMulFix rewrites zendis's "MUL D,E" to SNASM's required bare
// "MUL" -- confirmed against the real binary: SNASM's Z80N MUL has no
// operand syntax at all (the instruction is fixed on D*E->DE; there is
// no other register combination to select), and "MUL D,E" fails with
// "Illegal opcodes are not enabled, opcode \"D\" found" even under
// "opt illegal". Every other Z80N form checked -- SWAPNIB, OUTINB,
// PIXELDN, PIXELAD, SETAE, LDIX/LDWS/LDDX/LDIRX/LDDRX (all correctly
// bare already), MIRROR A, the ADD HL/DE/BC forms, PUSH imm16, TEST
// imm, the five barrel-shift instructions, JP (C) -- matched zendis's
// existing rendering exactly with no changes needed; MUL is the one
// exception. Exact-string replace is safe: "MUL D,E" is the literal,
// complete text Instruction.Text() renders for this one opcode and
// never appears as a substring of anything else in the rendered
// vocabulary.
func snasmMulFix(text string) string {
	return strings.ReplaceAll(text, "MUL D,E", "MUL")
}

// snasmHexPattern matches zendis's own 0x-prefixed hex literal
// rendering exactly (HexByte/HexWord always emit uppercase hex
// digits), so this can't accidentally match anything else in the
// rendered vocabulary -- labels, mnemonics, and comments never
// contain "0x" followed by hex digits.
var snasmHexPattern = regexp.MustCompile(`0x([0-9A-Fa-f]+)`)

// snasmLiteralFix converts every 0x-prefixed literal in text to
// SNASM's required $-prefixed form. Uses ReplaceAllStringFunc rather
// than a $-expansion replacement string: regexp's "$$" escape
// consumes both dollar signs as one literal "$", which also stops the
// following "{1}" from being parsed as a group reference at all --
// "$${1}" was tried and confirmed to produce the literal text "${1}"
// in the output, not the captured hex digits, before landing on this
// version.
func snasmLiteralFix(text string) string {
	return snasmHexPattern.ReplaceAllStringFunc(text, func(match string) string {
		return "$" + match[2:]
	})
}
